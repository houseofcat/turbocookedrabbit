package utils

import "github.com/houseofcat/turbocookedrabbit/models"

// CreateLetter creates a mock letter for publishing.
func CreateLetter(exchangeName string, queueName string, body []byte) *models.Letter {

	letterID := uint64(1)
	if body == nil {
		body = []byte("\x68\x65\x6c\x6c\x6f\x77\x20\x77\x6f\x72\x6c\x64")
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
