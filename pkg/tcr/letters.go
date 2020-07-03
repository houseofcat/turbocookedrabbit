package tcr

import (
	"math/rand"
	"sync/atomic"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/streadway/amqp"
)

var globalLetterID uint64
var mockRandomSource = rand.NewSource(time.Now().UnixNano())
var mockRandom = rand.New(mockRandomSource)

// CreateLetter creates a simple letter for publishing.
func CreateLetter(letterID uint64, exchangeName string, queueName string, body []byte) *Letter {

	envelope := &Envelope{
		Exchange:    exchangeName,
		RoutingKey:  queueName,
		ContentType: "application/json",
	}

	return &Letter{
		LetterID:   letterID,
		RetryCount: uint32(3),
		Body:       body,
		Envelope:   envelope,
	}
}

// CreateMockLetter creates a mock letter for publishing.
func CreateMockLetter(letterID uint64, exchangeName string, queueName string, body []byte) *Letter {

	if letterID == 0 {
		letterID = uint64(1)
	}

	if body == nil { //   h   e   l   l   o       w   o   r   l   d
		body = []byte("\x68\x65\x6c\x6c\x6f\x20\x77\x6f\x72\x6c\x64")
	}

	envelope := &Envelope{
		Exchange:     exchangeName,
		RoutingKey:   queueName,
		ContentType:  "application/json",
		DeliveryMode: 2,
	}

	return &Letter{
		LetterID:   letterID,
		RetryCount: uint32(3),
		Body:       body,
		Envelope:   envelope,
	}
}

// CreateMockRandomLetter creates a mock letter for publishing with random sizes and random Ids.
func CreateMockRandomLetter(queueName string) *Letter {

	letterID := atomic.LoadUint64(&globalLetterID)
	atomic.AddUint64(&globalLetterID, 1)

	body := RandomBytes(mockRandom.Intn(randomMax-randomMin) + randomMin)

	envelope := &Envelope{
		Exchange:     "",
		RoutingKey:   queueName,
		ContentType:  "application/json",
		DeliveryMode: 2,
		Headers:      make(amqp.Table, 0),
	}

	envelope.Headers["x-tcr-testheader"] = "HelloWorldHeader"

	return &Letter{
		LetterID:   letterID,
		RetryCount: uint32(0),
		Body:       body,
		Envelope:   envelope,
	}
}

// CreateMockRandomWrappedBodyLetter creates a mock Letter for publishing with random sizes and random Ids.
func CreateMockRandomWrappedBodyLetter(queueName string) *Letter {

	letterID := atomic.LoadUint64(&globalLetterID)
	atomic.AddUint64(&globalLetterID, 1)

	body := RandomBytes(mockRandom.Intn(randomMax-randomMin) + randomMin)

	envelope := &Envelope{
		Exchange:     "",
		RoutingKey:   queueName,
		ContentType:  "application/json",
		DeliveryMode: 2,
	}

	wrappedBody := &WrappedBody{
		LetterID: letterID,
		Body: &ModdedBody{
			Encrypted:   false,
			Compressed:  false,
			UTCDateTime: time.Now().Format(time.RFC3339),
			Data:        body,
		},
	}

	var json = jsoniter.ConfigFastest
	data, _ := json.Marshal(wrappedBody)

	letter := &Letter{
		LetterID:   letterID,
		RetryCount: uint32(0),
		Body:       data,
		Envelope:   envelope,
	}

	return letter
}
