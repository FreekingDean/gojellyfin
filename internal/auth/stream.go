package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/google/uuid"
)

const streamTagSeparator = "."

func StreamTag(sessionID uuid.UUID, token string, sourceID uuid.UUID) string {
	return hex.EncodeToString(sessionID[:]) + streamTagSeparator + streamSignature(token, sourceID)
}

func StreamTagSession(tag string) (uuid.UUID, bool) {
	session, _, ok := strings.Cut(tag, streamTagSeparator)
	if !ok {
		return uuid.Nil, false
	}
	raw, err := hex.DecodeString(session)
	if err != nil {
		return uuid.Nil, false
	}
	id, err := uuid.FromBytes(raw)

	return id, err == nil
}

func StreamTagValid(tag, token string, sourceID uuid.UUID) bool {
	_, signature, ok := strings.Cut(tag, streamTagSeparator)
	if !ok || token == "" {
		return false
	}

	return hmac.Equal([]byte(signature), []byte(streamSignature(token, sourceID)))
}

func streamSignature(token string, sourceID uuid.UUID) string {
	mac := hmac.New(sha256.New, []byte(token))
	mac.Write(sourceID[:])

	return hex.EncodeToString(mac.Sum(nil))
}
