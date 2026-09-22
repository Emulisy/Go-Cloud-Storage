package handler

import (
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"golang.org/x/crypto/bcrypt"
	"goCloudStorage/db"
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

		if len(username) < 3 || len(username) > 64 || len(userPwd) < 5 {
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
		w.Write([]byte("SUCCESS"))
			

	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
}

func SigninHandler(w http.ResponseWriter, r *http.Request) {

}