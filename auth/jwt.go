package auth

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// User is the identity authenticated by RequireAuth.
type User struct {
	Username string
}

// Handler is an HTTP handler that receives an authenticated user.
type Handler func(
	w http.ResponseWriter,
	r *http.Request,
	user User,
)

func getSecretKey() ([]byte, error) {
	key := []byte(os.Getenv("JWT_SECRET"))

	if len(key) < 32 {
		return nil, errors.New("JWT_SECRET must be at least 32 bytes")
	}

	return key, nil
}

// GenerateToken creates a JWT after successful sign-in.
func GenerateToken(username string) (string, error) {
	key, err := getSecretKey()
	if err != nil {
		return "", err
	}

	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		jwt.MapClaims{
			"username": username,
			"exp":      time.Now().Add(24 * time.Hour).Unix(),
		},
	)

	return token.SignedString(key)
}

// VerifyToken verifies a JWT and returns its username.
func VerifyToken(tokenString string) (string, error) {
	key, err := getSecretKey()
	if err != nil {
		return "", err
	}

	token, err := jwt.Parse(
		tokenString,
		func(token *jwt.Token) (any, error) {
			return key, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
	)

	if err != nil {
		return "", fmt.Errorf("verify JWT: %w", err)
	}

	if !token.Valid {
		return "", errors.New("invalid JWT")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("invalid JWT claims")
	}

	username, ok := claims["username"].(string)
	if !ok || username == "" {
		return "", errors.New("missing JWT username")
	}

	return username, nil
}

// RequireAuth verifies the access token and passes the authenticated user to next.
func RequireAuth(next Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("access_token")
		if err != nil {
			handleUnauthenticated(w, r)
			return
		}

		username, err := VerifyToken(cookie.Value)
		if err != nil {
			handleUnauthenticated(w, r)
			return
		}

		next(w, r, User{Username: username})
	}
}

func handleUnauthenticated(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet &&
		(r.URL.Path == "/file/home" || r.URL.Path == "/file/upload") {
		http.Redirect(w, r, "/file/signup", http.StatusSeeOther)
		return
	}

	http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
}
