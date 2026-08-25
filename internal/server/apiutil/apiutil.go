// Package apiutil holds the small generic helpers every tag package needs at
// the api boundary. Go cannot alias generic functions, so they live here rather
// than being copied into each package. No domain knowledge belongs in here.
package apiutil

// Ptr returns a pointer to the given value. This is useful for creating pointers
// to literals.
func Ptr[T any](v T) *T {
	return &v
}

// Deref returns the value pointed to by the given pointer. If the pointer is nil,
// it returns the zero value of the type.
func Deref[T any](v *T) T {
	if v == nil {
		var zero T
		return zero
	}

	return *v
}

// OrElse returns the value pointed to by the given pointer. If the pointer is nil,
// it returns the fallback value.
func OrElse[T any](v *T, fallback T) T {
	if v == nil {
		return fallback
	}

	return *v
}

// oapi-codegen splits a JSON body across two fields depending on content type.
func Body[T any](jsonBody, wildcardBody *T) *T {
	if jsonBody != nil {
		return jsonBody
	}

	return wildcardBody
}
