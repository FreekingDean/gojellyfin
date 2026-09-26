package apiutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestZeroOrNilPtr(t *testing.T) {
	tests := []struct {
		name     string
		got      any
		expected any
	}{
		{"Zero int", ZeroOrNilPtr(0), (*int)(nil)},
		{"Zero string", ZeroOrNilPtr(""), (*string)(nil)},
		{"Some int", ZeroOrNilPtr(42), Ptr(42)},
		{"Some string", ZeroOrNilPtr("hi"), Ptr("hi")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, test.got)
		})
	}
}
