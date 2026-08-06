package config

func ptr[T any](v T) *T {
	return &v
}

func deref[T any](v *T) T {
	if v == nil {
		var zero T
		return zero
	}

	return *v
}

func orElse[T any](v *T, fallback T) *T {
	if v == nil {
		return &fallback
	}

	return v
}

// oapi-codegen splits a JSON body across two fields depending on content type.
func body[T any](jsonBody, wildcardBody *T) *T {
	if jsonBody != nil {
		return jsonBody
	}

	return wildcardBody
}
