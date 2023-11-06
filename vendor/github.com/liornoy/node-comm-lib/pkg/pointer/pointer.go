package pointer

func Int32Ptr(n int32) *int32 {
	return &n
}

func StrPtr(s string) *string {
	return &s
}
