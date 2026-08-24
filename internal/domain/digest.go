package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func StableDigest(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func VerifyDigest(value any, expected string) bool {
	actual, err := StableDigest(value)
	return err == nil && actual == expected
}
