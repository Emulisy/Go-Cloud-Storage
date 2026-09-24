package auth

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// User is the identity authenticated by RequireAuth.
type User struct {
	UserID int64
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
func GenerateToken(userID int64) (string, error) {
	if userID < 1 {
		return "", errors.New("invalid user ID")
	}
	key, err := getSecretKey()
	if err != nil {
		return "", err
	}

	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		jwt.MapClaims{
			"userId": strconv.FormatInt(userID, 10),
			"exp":    time.Now().Add(24 * time.Hour).Unix(),
		},
	)

	return token.SignedString(key)
}

// VerifyToken verifies a JWT and returns its user ID.
func VerifyToken(tokenString string) (int64, error) {
	key, err := getSecretKey()
	if err != nil {
		return 0, err
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
		return 0, fmt.Errorf("verify JWT: %w", err)
	}

	if !token.Valid {
		return 0, errors.New("invalid JWT")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, errors.New("invalid JWT claims")
	}

	rawID, ok := claims["userId"].(string)
	if !ok {
		return 0, errors.New("missing JWT user ID")
	}
	userID, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || userID < 1 || strconv.FormatInt(userID, 10) != rawID {
		return 0, errors.New("invalid JWT user ID")
	}
	return userID, nil
}

// RequireAuth verifies the access token and passes the authenticated user to next.
func RequireAuth(next Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("access_token")
		if err != nil {
			handleUnauthenticated(w, r)
			return
		}

		userID, err := VerifyToken(cookie.Value)
		if err != nil {
			handleUnauthenticated(w, r)
			return
		}

		next(w, r, User{UserID: userID})
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
