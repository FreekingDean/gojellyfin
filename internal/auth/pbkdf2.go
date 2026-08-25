package auth

import (
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/sha1"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"strconv"
	"strings"
)

// Jellyfin writes its password hashes as a PHC string with the salt and the
// hash in hex rather than base64: $<id>$iterations=<n>$<salt>$<hash>. Verifying
// one is what lets an imported user keep the password they already had.
const jellyfinPrefix = "$PBKDF2"

func isJellyfinHash(encoded string) bool {
	return strings.HasPrefix(encoded, jellyfinPrefix+"$") || strings.HasPrefix(encoded, jellyfinPrefix+"-")
}

func verifyJellyfin(password, encoded string) (bool, error) {
	segments := strings.Split(strings.TrimPrefix(encoded, "$"), "$")
	if len(segments) != 4 {
		return false, fmt.Errorf("a jellyfin hash has four segments, got %d", len(segments))
	}

	var digest func() hash.Hash
	var length int
	switch segments[0] {
	case "PBKDF2":
		digest, length = func() hash.Hash { return sha1.New() }, 32
	case "PBKDF2-SHA512":
		digest, length = func() hash.Hash { return sha512.New() }, 64
	default:
		return false, fmt.Errorf("unsupported jellyfin hash %q", segments[0])
	}

	iterations, err := jellyfinIterations(segments[1])
	if err != nil {
		return false, err
	}

	salt, err := hex.DecodeString(segments[2])
	if err != nil {
		return false, fmt.Errorf("failed to decode the jellyfin salt: %w", err)
	}
	want, err := hex.DecodeString(segments[3])
	if err != nil {
		return false, fmt.Errorf("failed to decode the jellyfin hash: %w", err)
	}
	if len(want) != length {
		return false, fmt.Errorf("a %s hash is %d bytes, got %d", segments[0], length, len(want))
	}

	got, err := pbkdf2.Key(digest, password, salt, iterations, length)
	if err != nil {
		return false, fmt.Errorf("failed to derive the jellyfin hash: %w", err)
	}

	return hmac.Equal(got, want), nil
}

func jellyfinIterations(parameters string) (int, error) {
	for _, parameter := range strings.Split(parameters, ",") {
		name, value, found := strings.Cut(parameter, "=")
		if !found || name != "iterations" {
			continue
		}

		iterations, err := strconv.Atoi(value)
		if err != nil || iterations < 1 {
			return 0, fmt.Errorf("a jellyfin hash needs a positive iteration count, got %q", value)
		}

		return iterations, nil
	}

	return 0, fmt.Errorf("a jellyfin hash needs an iterations parameter, got %q", parameters)
}
