package apiutil

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

func OrElse[T any](v *T, fallback T) T {
	if v == nil {
		return fallback
	}

	return *v
}

func Body[T any](jsonBody, wildcardBody *T) *T {
	if jsonBody != nil {
		return jsonBody
	}

	return wildcardBody
}
