package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	scheme     = "PBKDF2-SHA512"
	iterations = 210000
	saltLength = 16
	keyLength  = 64
	tokenBytes = 16
)

var ErrInvalidHash = errors.New("invalid password hash")

func Hash(password string) (string, error) {
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	key, err := pbkdf2.Key(sha512.New, password, salt, iterations, keyLength)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("$%s$%d$%s$%s", scheme, iterations, encode(salt), encode(key)), nil
}

func Verify(password, hash string) (bool, error) {
	parts := strings.Split(hash, "$")
	if len(parts) != 5 || parts[1] != scheme {
		return false, ErrInvalidHash
	}

	iter, err := strconv.Atoi(parts[2])
	if err != nil {
		return false, ErrInvalidHash
	}

	salt, err := hex.DecodeString(parts[3])
	if err != nil {
		return false, ErrInvalidHash
	}

	want, err := hex.DecodeString(parts[4])
	if err != nil {
		return false, ErrInvalidHash
	}

	got, err := pbkdf2.Key(sha512.New, password, salt, iter, len(want))
	if err != nil {
		return false, err
	}

	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

func NewToken() (string, error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}

func encode(b []byte) string {
	return strings.ToUpper(hex.EncodeToString(b))
}
