package tcr

import (
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/streadway/amqp"
)

var mockRandomSource = rand.NewSource(time.Now().UnixNano())
var mockRandom = rand.New(mockRandomSource)

// CreateLetter creates a simple letter for publishing.
func CreateLetter(exchangeName string, routingKey string, body []byte) *Letter {

	envelope := &Envelope{
		Exchange:    exchangeName,
		RoutingKey:  routingKey,
		ContentType: "application/json",
	}

	return &Letter{
		LetterID:   uuid.New(),
		RetryCount: uint32(3),
		Body:       body,
		Envelope:   envelope,
	}
}

// CreateMockLetter creates a mock letter for publishing.
func CreateMockLetter(exchangeName string, routingKey string, body []byte) *Letter {

	if body == nil { //   h   e   l   l   o       w   o   r   l   d
		body = []byte("\x68\x65\x6c\x6c\x6f\x20\x77\x6f\x72\x6c\x64")
	}

	envelope := &Envelope{
		Exchange:     exchangeName,
		RoutingKey:   routingKey,
		ContentType:  "application/json",
		DeliveryMode: 2,
	}

	return &Letter{
		LetterID:   uuid.New(),
		RetryCount: uint32(3),
		Body:       body,
		Envelope:   envelope,
	}
}

// CreateMockRandomLetter creates a mock letter for publishing with random sizes and random Ids.
func CreateMockRandomLetter(routingKey string) *Letter {

	body := RandomBytes(mockRandom.Intn(randomMax-randomMin) + randomMin)

	envelope := &Envelope{
		Exchange:     "",
		RoutingKey:   routingKey,
		ContentType:  "application/json",
		DeliveryMode: 2,
		Headers:      make(amqp.Table),
	}

	envelope.Headers["x-tcr-testheader"] = "HelloWorldHeader"

	return &Letter{
		LetterID:   uuid.New(),
		RetryCount: uint32(0),
		Body:       body,
		Envelope:   envelope,
	}
}

// CreateMockRandomWrappedBodyLetter creates a mock Letter for publishing with random sizes and random Ids.
func CreateMockRandomWrappedBodyLetter(routingKey string) *Letter {

	body := RandomBytes(mockRandom.Intn(randomMax-randomMin) + randomMin)

	envelope := &Envelope{
		Exchange:     "",
		RoutingKey:   routingKey,
		ContentType:  "application/json",
		DeliveryMode: 2,
	}

	wrappedBody := &WrappedBody{
		LetterID: uuid.New(),
		Body: &ModdedBody{
			Encrypted:   false,
			Compressed:  false,
			UTCDateTime: JSONUtcTimestamp(),
			Data:        body,
		},
	}

	data, _ := json.Marshal(wrappedBody)

	letter := &Letter{
		LetterID:   wrappedBody.LetterID,
		RetryCount: uint32(0),
		Body:       data,
		Envelope:   envelope,
	}

	return letter
}
