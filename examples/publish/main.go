package main

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/houseofcat/turbocookedrabbit/v2/pkg/tcr"
)

type MyMessage struct {
	TestMessage string `json:"TestMessage"`
}

// Examples - Publishing
// August 24th, 2021
func main() {

	config, err := tcr.ConvertJSONFileToConfig("config.json") // Load Configuration On Startup
	if err != nil {
		log.Fatal(err)
	}

	rabbitService, err := tcr.NewRabbitService(config, "", "", nil, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Convenience struct JSON publishing
	// build struct you wish to publish, then publish it.
	err = rabbitService.Publish(&MyMessage{TestMessage: "Hello World"}, "exchangeName", "routingKey/queueName", "my-test-letter", false, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Convenience bytes Publishing
	// send bytes to exchange/queue
	err = rabbitService.PublishData([]byte("Hello World"), "exchangeName", "routingKey/queueName", nil)
	if err != nil {
		log.Fatal(err)
	}

	// Manually use the internal Publisher bytes through the convenience service
	// First get a UUID ID
	id, err := uuid.NewUUID()
	if err != nil {
		log.Fatal(err)
	}
	// Then publish (this time with a confirmation/context)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rabbitService.Publisher.PublishWithConfirmationContext(
		ctx,
		&tcr.Letter{
			LetterID: id,
			Body:     []byte("Hello World"),
			Envelope: &tcr.Envelope{
				Exchange:     "exchangeName",
				RoutingKey:   "routingKey/queueName",
				ContentType:  "application/json",
				Mandatory:    false,
				Immediate:    false,
				DeliveryMode: 2,
			},
		})

	// Publish with the internal auto-publisher
	letter := &tcr.Letter{
		LetterID: id,
		Body:     []byte("Hello World"),
		Envelope: &tcr.Envelope{
			Exchange:     "exchangeName",
			RoutingKey:   "routingKey/queuename",
			ContentType:  "application/json",
			Mandatory:    false,
			Immediate:    false,
			DeliveryMode: 2,
		},
	}
	err = rabbitService.QueueLetter(letter)
	if err != nil {
		log.Fatal(err)
	}
}
