package ptr

func To[T any](v T) *T {
	return &v
}

func Deref[T any](ptr *T, def T) T {
	if ptr != nil {
		return *ptr
	}
	return def
}
