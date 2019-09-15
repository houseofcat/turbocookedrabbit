package utils

import (
	"math/rand"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/houseofcat/turbocookedrabbit/models"
)

var globalLetterID uint64
var mockRandomSource = rand.NewSource(time.Now().UnixNano())
var mockRandom = rand.New(mockRandomSource)
var randomMin = 1500
var randomMax = 2500

// CreateLetter creates a simple letter for publishing.
func CreateLetter(exchangeName string, queueName string, body []byte) *models.Letter {

	letterID := uint64(1)

	if body == nil { //   h   e   l   l   o       w   o   r   l   d
		body = []byte("\x68\x65\x6c\x6c\x6f\x20\x77\x6f\x72\x6c\x64")
	}

	envelope := &models.Envelope{
		Exchange:    "",
		RoutingKey:  queueName,
		ContentType: "application/json",
		Mandatory:   false,
		Immediate:   false,
	}

	return &models.Letter{
		LetterID:   letterID,
		RetryCount: uint32(3),
		Body:       body,
		Envelope:   envelope,
	}
}

// CreateMockLetter creates a mock letter for publishing.
func CreateMockLetter(letterID uint64, exchangeName string, queueName string, body []byte) *models.Letter {

	if letterID == 0 {
		letterID = uint64(1)
	}

	if body == nil { //   h   e   l   l   o       w   o   r   l   d
		body = []byte("\x68\x65\x6c\x6c\x6f\x20\x77\x6f\x72\x6c\x64")
	}

	envelope := &models.Envelope{
		Exchange:    "",
		RoutingKey:  queueName,
		ContentType: "application/json",
		Mandatory:   false,
		Immediate:   false,
	}

	return &models.Letter{
		LetterID:   letterID,
		RetryCount: uint32(3),
		Body:       body,
		Envelope:   envelope,
	}
}

// CreateMockRandomLetter creates a mock letter for publishing with random sizes and random Ids.
func CreateMockRandomLetter(queueName string) *models.Letter {

	letterID := atomic.LoadUint64(&globalLetterID)
	atomic.AddUint64(&globalLetterID, 1)

	body := RandomBytes(mockRandom.Intn(randomMax-randomMin) + randomMin)

	envelope := &models.Envelope{
		Exchange:    "",
		RoutingKey:  queueName,
		ContentType: "application/json",
		Mandatory:   false,
		Immediate:   false,
	}

	return &models.Letter{
		LetterID:   letterID,
		RetryCount: uint32(0),
		Body:       body,
		Envelope:   envelope,
	}
}

const (
	letterBytes   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

// RandomString generates a Random string.
// var src = rand.NewSource(time.Now().UnixNano())
//https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-go
func RandomString(size int, src rand.Source) string {

	b := make([]byte, size)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
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
	return []byte(RandomString(size, mockRandomSource))
}
