package auth

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
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