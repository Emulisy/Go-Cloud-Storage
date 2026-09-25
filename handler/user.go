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

// UpdateUserNameHandler accepts a userName form field for the authenticated account.
func UpdateUserNameHandler(w http.ResponseWriter, r *http.Request, user auth.User) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPatch {
		w.Header().Set("Allow", "PATCH")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.PostForm.Get("userName"))
	if len(name) < 3 || len(name) > 64 {
		http.Error(w, "Username must be between 3 and 64 bytes", http.StatusBadRequest)
		return
	}
	if err := db.UpdateUserName(user.UserID, name); err != nil {
		if errors.Is(err, db.ErrInvalidCredentials) {
			http.Error(w, "Invalid credentials or inactive account", http.StatusUnauthorized)
			return
		}

		log.Printf("Update user info: %v", err)
		http.Error(w, "Unable to update user information", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("SUCCESS"))
}

// UpdateUserPwdHandler accepts currentPwd and newPwd plaintext form fields.
// The database layer verifies the current password and hashes the replacement.
func UpdateUserPwdHandler(w http.ResponseWriter, r *http.Request, user auth.User) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPatch {
		w.Header().Set("Allow", "PATCH")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}
	currentPwd := r.PostForm.Get("currentPwd")
	newPwd := r.PostForm.Get("newPwd")
	if len(currentPwd) < 5 || len(currentPwd) > 72 || len(newPwd) < 5 || len(newPwd) > 72 {
		http.Error(w, "Passwords must be between 5 and 72 bytes", http.StatusBadRequest)
		return
	}
	if err := db.UpdateUserPwd(user.UserID, currentPwd, newPwd); err != nil {
		if errors.Is(err, db.ErrInvalidCredentials) {
			http.Error(w, "Invalid credentials or inactive account", http.StatusUnauthorized)
			return
		}

		log.Printf("Update user info: %v", err)
		http.Error(w, "Unable to update user information", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("SUCCESS"))
}

// UpdateUserEmailHandler accepts an email form field for the authenticated account.
func UpdateUserEmailHandler(w http.ResponseWriter, r *http.Request, user auth.User) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPatch {
		w.Header().Set("Allow", "PATCH")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}
	email, err := db.NormalizeEmail(r.PostForm.Get("email"))
	if err != nil {
		http.Error(w, "Invalid email address", http.StatusBadRequest)
		return
	}
	if err := db.UpdateUserEmail(user.UserID, email); err != nil {
		if errors.Is(err, db.ErrInvalidCredentials) {
			http.Error(w, "Invalid credentials or inactive account", http.StatusUnauthorized)
			return
		}
		if errors.Is(err, db.ErrEmailExists) {
			http.Error(w, "Email already exists", http.StatusConflict)
			return
		}
		log.Printf("Update user info: %v", err)
		http.Error(w, "Unable to update user information", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("SUCCESS"))
}

//sign out, clear the cookie return nothing
func SignOutHandler(
    w http.ResponseWriter,
    r *http.Request,
) {
    // 1. Check HTTP method.
    if r.Method != http.MethodPost {
        w.Header().Set("Allow", "POST")
        http.Error(
            w,
            "Method not allowed",
            http.StatusMethodNotAllowed,
        )
        return
    }

    // 2. Clear the authentication cookie.
    http.SetCookie(w, &http.Cookie{
        Name:     "access_token",
        Value:    "",
        Path:     "/",
        MaxAge:   -1,
        HttpOnly: true,
        Secure:   r.TLS != nil,
        SameSite: http.SameSiteLaxMode,
    })

    // 3. Return success.
    w.WriteHeader(http.StatusNoContent)
}

// user sign up, create new user in tbl_user
func SignUpHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet: //get signup page
		data, err := os.ReadFile("static/view/signup.html")
		if err != nil {
			http.Error(w, "Can't load signup page", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
		return

	case http.MethodPost: //post sign up info
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}

		username := strings.TrimSpace(r.PostForm.Get("userName"))
		email, emailErr := db.NormalizeEmail(r.PostForm.Get("email"))
		userPwd := r.PostForm.Get("userPwd")

		if emailErr != nil || len(username) < 3 || len(username) > 64 || len(userPwd) < 5 || len(userPwd) > 72 {
			http.Error(w, "Invalid username, email or password", http.StatusBadRequest)
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
		err = db.UserSignUp(username, email, string(hashedPwd))
		if err != nil {
			if errors.Is(err, db.ErrEmailExists) {
				http.Error(w, "Email already exists", http.StatusConflict)
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

	email, emailErr := db.NormalizeEmail(r.PostForm.Get("email"))
	userPwd := r.PostForm.Get("userPwd")

	if emailErr != nil || len(userPwd) < 5 || len(userPwd) > 72 {
		http.Error(w, "Invalid email or password", http.StatusBadRequest)
		return
	}

	userID, err := db.UserSignin(email, userPwd)
	if err != nil {
		if errors.Is(err, db.ErrInvalidCredentials) {
			http.Error(w, "Invalid email or password", http.StatusUnauthorized)
			return
		}

		log.Printf("Signin failed: %v", err)
		http.Error(w, "Unable to sign in", http.StatusInternalServerError)
		return
	}

	token, err := auth.GenerateToken(userID)
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

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("SUCCESS"))
}

// HomeHandler displays the authenticated user's home page.
func HomeHandler(w http.ResponseWriter, r *http.Request, user auth.User) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
func UserInfoHandler(w http.ResponseWriter, r *http.Request, user auth.User) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userInfo, err := db.GetUserInfo(user.UserID)
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
		Email      string `json:"email"`
		SignupAt   string `json:"signupAt"`
		LastActive string `json:"lastActive"`
	}{
		Username:   userInfo.Username,
		Email:      userInfo.Email,
		SignupAt:   userInfo.SignupAt,
		LastActive: userInfo.LastActive,
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Encode user info: %v", err)
	}
}
