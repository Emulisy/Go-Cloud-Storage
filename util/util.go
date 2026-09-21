package util

import (
	"crypto/sha256"
    "encoding/hex"
	"io"
)

func CalculateSHA256(file io.Reader) (string, error) {
    hasher := sha256.New()

    _, err := io.Copy(hasher, file)
    if err != nil {
        return "", err
    }

    return hex.EncodeToString(hasher.Sum(nil)), nil
}