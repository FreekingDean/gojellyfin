package apiutil

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
