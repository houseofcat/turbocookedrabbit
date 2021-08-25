package main

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/houseofcat/turbocookedrabbit/v2/pkg/tcr"
)

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

	// 1.) Publish directly with a Publisher (create your own or use the one prebuilt in RabbitService.Publisher)
	// Get ID
	// Make a Letter (basically context needed to publish your message with the message)
	// Call Publish
	id, err := uuid.NewUUID()
	if err != nil {
		log.Fatal(err)
	}
	letter := &tcr.Letter{
		LetterID: id,
		Body:     []byte("Hello World"),
		Envelope: &tcr.Envelope{
			Exchange:     "exchangeName",
			RoutingKey:   "routingKey/queuename",
			ContentType:  "text/plain",
			Mandatory:    false,
			Immediate:    false,
			Priority:     0,
			DeliveryMode: 2,
		},
	}
	skipReceipt := true
	rabbitService.Publisher.Publish(letter, skipReceipt)

	// There is a lot of convenience that you may end up writing. My library is basically a connection pool with a lot of small
	// conveniences to make using RabbitMQ in general much easier/streamlined.

	// 2.) Convenience RabbitService publishing a struct as JSON.
	// build struct you wish to publish, then publish it.
	type MyMessage struct {
		TestMessage string `json:"TestMessage"`
	}
	err = rabbitService.Publish(&MyMessage{TestMessage: "Hello World"}, "exchangeName", "routingKey/queueName", "my-test-letter", false, nil)
	if err != nil {
		log.Fatal(err)
	}

	// 3.) Convenience RabbitService publishing bytes.
	// send string (as bytes) to exchange/queue
	err = rabbitService.PublishData([]byte("Hello World"), "exchangeName", "routingKey/queueName", nil)
	if err != nil {
		log.Fatal(err)
	}

	// 4.) Convenience RabbitService Publisher using Publish with Confirmation/Timeout.
	// First get a UUID ID
	// Get a timeout context
	id, err = uuid.NewUUID()
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
				ContentType:  "text/plain",
				Mandatory:    false,
				Immediate:    false,
				DeliveryMode: 2,
			},
		})

	// 5.) Publish with the internal auto-publisher
	// Create an ID
	// Create a Letter
	// Queue Letter for Publishing by the library.
	//   Has fault tolerance, retryability, and sends receipts (receipt of publish) to the Receipt Channel to be
	//   worked on further or retried by the calling application or whatever.
	id, err = uuid.NewUUID()
	if err != nil {
		log.Fatal(err)
	}
	letter = &tcr.Letter{
		LetterID: id,
		Body:     []byte("Hello World"),
		Envelope: &tcr.Envelope{
			Exchange:     "exchangeName",
			RoutingKey:   "routingKey/queuename",
			ContentType:  "text/plain",
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
