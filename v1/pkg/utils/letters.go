package utils

import (
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/houseofcat/turbocookedrabbit/v1/pkg/models"
)

var globalLetterID uint64
var mockRandomSource = rand.NewSource(time.Now().UnixNano())
var mockRandom = rand.New(mockRandomSource)

// CreateLetter creates a simple letter for publishing.
func CreateLetter(letterID uint64, exchangeName string, queueName string, body []byte) *models.Letter {

	envelope := &models.Envelope{
		Exchange:    exchangeName,
		RoutingKey:  queueName,
		ContentType: "application/json",
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
		Exchange:    exchangeName,
		RoutingKey:  queueName,
		ContentType: "application/json",
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
	}

	return &models.Letter{
		LetterID:   letterID,
		RetryCount: uint32(0),
		Body:       body,
		Envelope:   envelope,
	}
}
