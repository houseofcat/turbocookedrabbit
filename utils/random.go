package utils

import (
	"math/rand"
	"time"
	"unsafe"
)

const (
	randomMin = 1500
	randomMax = 2500

	letterBytes   = "0123456789!@#$%^&*()_+abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

// RandomStringFromSource generates a Random string that should always be unique.
// Example RandSrc.) var src = rand.NewSource(time.Now().UnixNano())
// Source: https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-go
func RandomStringFromSource(size int, src rand.Source) string {

	b := make([]byte, size)

	for i, cache, remain := size-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return *(*string)(unsafe.Pointer(&b))
}

// RandomString creates a new RandomSource to generate a RandomString unique per nanosecond.
func RandomString(size int) string {

	src := rand.NewSource(time.Now().UnixNano())
	b := make([]byte, size)

	for i, cache, remain := size-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return *(*string)(unsafe.Pointer(&b))
}

// RandomBytes returns a RandomString converted to bytes.
func RandomBytes(size int) []byte {
	return []byte(RandomStringFromSource(size, mockRandomSource))
}
