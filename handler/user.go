package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"goCloudStorage/auth"
	"goCloudStorage/db"
	"golang.org/x/crypto/bcrypt"
)

func SignUpHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		data, err := os.ReadFile("static/view/signup.html")
		if err != nil {
			http.Error(w, "Can't load signup page", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
		return

	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}

		username := strings.TrimSpace(r.PostForm.Get("userName"))
		userPwd := r.PostForm.Get("userPwd")

		if len(username) < 3 || len(username) > 64 || len(userPwd) < 5 || len(userPwd) > 72 {
			http.Error(w, "Invalid username or password", http.StatusBadRequest)
			return
		}

		// Hash the password before storing it.
		hashedPwd, err := bcrypt.GenerateFromPassword(
			[]byte(userPwd),
			bcrypt.DefaultCost,
		)
		if err != nil {
			log.Printf("Password hashing failed: %v", err)
			http.Error(w, "Unable to sign up", http.StatusInternalServerError)
			return
		}

		// Pass the password hash to the database layer.
		err = db.UserSignUp(username, string(hashedPwd))
		if err != nil {
			if errors.Is(err, db.ErrUsernameExists) {
				http.Error(w, "Username already exists", http.StatusConflict)
				return
			}

			log.Printf("Signup failed: %v", err)
			http.Error(w, "Unable to sign up", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("SUCCESS"))

	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
}

func SigninHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	username := strings.TrimSpace(r.PostForm.Get("userName"))
	userPwd := r.PostForm.Get("userPwd")

	if len(username) < 3 || len(username) > 64 || len(userPwd) < 5 || len(userPwd) > 72 {
		http.Error(w, "Invalid username or password", http.StatusBadRequest)
		return
	}

	err := db.UserSignin(username, userPwd)
	if err != nil {
		if errors.Is(err, db.ErrInvalidCredentials) {
			http.Error(w, "Invalid username or password", http.StatusUnauthorized)
			return
		}

		log.Printf("Signin failed: %v", err)
		http.Error(w, "Unable to sign in", http.StatusInternalServerError)
		return
	}

	token, err := auth.GenerateToken(username)
	if err != nil {
		log.Printf("Generate token: %v", err)
		http.Error(w, "Unable to sign in", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   24 * 60 * 60,
	})

	http.Redirect(w, r, "/file/home", http.StatusSeeOther)
}

// HomeHandler displays the authenticated user's home page.
func HomeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if _, err := authenticatedUsername(r); err != nil {
		http.Redirect(w, r, "/file/signup", http.StatusSeeOther)
		return
	}

	data, err := os.ReadFile("static/view/home.html")
	if err != nil {
		http.Error(w, "Unable to load home page", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

// UserInfoHandler returns the authenticated user's information as JSON.
func UserInfoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	username, err := authenticatedUsername(r)
	if err != nil {
		http.Error(w, "Please sign in", http.StatusUnauthorized)
		return
	}

	user, err := db.GetUserInfo(username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "User not found", http.StatusUnauthorized)
			return
		}

		log.Printf("Get user info: %v", err)
		http.Error(w, "Unable to retrieve user information", http.StatusInternalServerError)
		return
	}

	response := struct {
		Username   string `json:"username"`
		Phone      string `json:"phone"`
		Email      string `json:"email"`
		SignupAt   string `json:"signupAt"`
		LastActive string `json:"lastActive"`
	}{
		Username:   user.Username,
		Phone:      user.Phone,
		Email:      user.Email,
		SignupAt:   user.SignupAt,
		LastActive: user.LastActive,
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Encode user info: %v", err)
	}
}

func authenticatedUsername(r *http.Request) (string, error) {
	cookie, err := r.Cookie("access_token")
	if err != nil {
		return "", err
	}

	return auth.VerifyToken(cookie.Value)
}
