package upload

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

const randomIDBytes = 16

// RandomID returns a filesystem-safe, unpredictable identifier.
func RandomID() (string, error) {
	bytes := make([]byte, randomIDBytes)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate random file ID: %w", err)
	}

	return hex.EncodeToString(bytes), nil
}
