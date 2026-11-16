package auth

import (
	"testing"

	"github.com/google/uuid"
)

func TestStreamTag(t *testing.T) {
	session, source, other := uuid.New(), uuid.New(), uuid.New()
	tag := StreamTag(session, "token", source)

	got, ok := StreamTagSession(tag)
	if !ok || got != session {
		t.Fatalf("StreamTagSession(%q) = %v, %v, want %v", tag, got, ok, session)
	}

	tests := []struct {
		name   string
		tag    string
		token  string
		source uuid.UUID
		want   bool
	}{
		{"the file it was minted for", tag, "token", source, true},
		{"another file", tag, "token", other, false},
		{"another session's token", tag, "other", source, false},
		{"no token", tag, "", source, false},
		{"no signature", "abc", "token", source, false},
		{"a forged signature", tag[:33] + "00", "token", source, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := StreamTagValid(test.tag, test.token, test.source); got != test.want {
				t.Errorf("StreamTagValid = %v, want %v", got, test.want)
			}
		})
	}

	for _, bad := range []string{"", "nodot", "zz.ab", "abcd.ef"} {
		if _, ok := StreamTagSession(bad); ok {
			t.Errorf("StreamTagSession(%q) ok, want refused", bad)
		}
	}
}
