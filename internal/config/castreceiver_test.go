package config

import "testing"

func TestCastReceiverID(t *testing.T) {
	for _, test := range []struct {
		name   string
		chosen string
		want   string
	}{
		{"nothing chosen is the stable receiver", "", "F007D354"},
		{"a listed receiver stands", "6F511C87", "6F511C87"},
		{"an unlisted receiver is the stable one", "DEADBEEF", "F007D354"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := CastReceiverID(test.chosen); got != test.want {
				t.Errorf("CastReceiverID(%q) = %q, want %q", test.chosen, got, test.want)
			}
		})
	}
}
