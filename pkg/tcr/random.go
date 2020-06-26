package tcr

import (
	"math/rand"
	"strings"
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

// RepeatedBytes generates a RandomString and then repeats it up to size.
func RepeatedBytes(size int, repeat int) []byte {

	if repeat < 10 {
		return nil
	}

	buffer := make([]byte, size)

AddString:
	for i := 0; i < size; i++ {
		for j := 0; j < repeat; j++ {
			if i+j == size {
				break AddString
			}

			buffer[i] = byte(i)
		}
	}

	return buffer
}

// RepeatedRandomString generates a RandomString and then repeats it up to size and repeat count.
func RepeatedRandomString(size int, repeat int) string {

	if repeat < 10 {
		return ""
	}

	var builder strings.Builder

AddString:
	for i := 0; i < size; i++ {
		char := RandomString(1)
		for j := 0; j < repeat; j++ {
			if i+j == size {
				break AddString
			}

			builder.WriteString(char)
		}
	}

	return builder.String()
}
