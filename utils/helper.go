package utils

import "github.com/houseofcat/turbocookedrabbit/models"

// CreateLetter creates a letter for publishing.
func CreateLetter(exchangeName string, queueName string, body []byte) *models.Letter {

	letterID := uint64(1)
	if body == nil {
		body = []byte("\xFF\xFF\x89\xFF\xFF")
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
