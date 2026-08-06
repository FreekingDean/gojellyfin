// Package dtos translates domain models into generated api types, for shapes
// that more than one tag's handlers need.
package dtos

import (
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func Ptr[T any](v T) *T {
	return &v
}

func Deref[T any](v *T) T {
	if v == nil {
		var zero T
		return zero
	}

	return *v
}

func OrElse[T any](v *T, fallback T) *T {
	if v == nil {
		return &fallback
	}

	return v
}

// oapi-codegen splits a JSON body across two fields depending on content type.
func Body[T any](jsonBody, wildcardBody *T) *T {
	if jsonBody != nil {
		return jsonBody
	}

	return wildcardBody
}

func UID(s string) *openapi_types.UUID {
	u := uuid.MustParse(s)
	return &u
}
