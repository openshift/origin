package compat_otp

import (
	"math/rand"
	"time"
	"unsafe"
)

const (
	// 6 bits to represent a letter index
	letterIDBits = 6
	// All 1-bits as many as letterIdBits
	letterIDMask = 1<<letterIDBits - 1
	letterIDMax  = 63 / letterIDBits
	letters      = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

// RandStrDefault This is a utility function for random strings random strings' length=8
func RandStrDefault() string {
	return RandStr(8)
}

// RandStr This is a utility function for random strings n: random strings' length
func RandStr(n int) string {
	return RandStrCustomize(letters, n)
}

// RandStrCustomize This is a utility function for random strings n: random strings' length, s: Customizable String Sets
func RandStrCustomize(s string, n int) string {
	var src = rand.NewSource(time.Now().UnixNano())
	b := make([]byte, n)
	// A rand.Int63() generates 63 random bits, enough for letterIdMax letters!
	for i, cache, remain := n-1, src.Int63(), letterIDMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIDMax
		}
		if idx := int(cache & letterIDMask); idx < len(s) {
			b[i] = s[idx]
			i--
		}
		cache >>= letterIDBits
		remain--
	}
	return *(*string)(unsafe.Pointer(&b))
}
