package tests

import (
	"fmt"
	"testing"
	"time"

	"github.com/houseofcat/turbocookedrabbit/models"
	"github.com/houseofcat/turbocookedrabbit/pools"
	"github.com/houseofcat/turbocookedrabbit/publisher"
	"github.com/stretchr/testify/assert"
)

func TestCreatePublisher(t *testing.T) {
	Seasoning.Pools.ConnectionCount = 1
	Seasoning.Pools.ChannelCount = 2

	channelPool, err := pools.NewChannelPool(Seasoning, nil, true)
	assert.NoError(t, err)

	// Flush Any Errors
	select {
	case chanErr := <-channelPool.Errors():
		fmt.Print(chanErr)
	default:
		break
	}

	publisher, err := publisher.NewPublisher(Seasoning, channelPool, nil)
	assert.NoError(t, err)
	assert.NotNil(t, publisher)

	// Flush Any Notifications
	select {
	case notification := <-publisher.Notifications():
		if notification.Success {
			fmt.Printf("Publish Success: %d\r\n", notification.LetterID)
		} else {
			fmt.Printf("Publish Failed: %s\r\n", notification.Error)
		}
	default:
		break
	}
}

func TestCreatePublisherAndPublish(t *testing.T) {
	Seasoning.Pools.ConnectionCount = 1
	Seasoning.Pools.ChannelCount = 2

	channelPool, err := pools.NewChannelPool(Seasoning, nil, true)
	assert.NoError(t, err)

	// Flush Any Errors
	select {
	case chanErr := <-channelPool.Errors():
		fmt.Print(chanErr)
	default:
		break
	}

	publisher, err := publisher.NewPublisher(Seasoning, channelPool, nil)
	assert.NoError(t, err)
	assert.NotNil(t, publisher)

	letterID := uint64(1)
	body := "\xFF\xFF\x89\xFF\xFF"
	envelope := &models.Envelope{
		Exchange:    "",
		RoutingKey:  "TestQueue",
		ContentType: "plain/text",
		Mandatory:   false,
		Immediate:   false,
	}

	letter := &models.Letter{
		LetterID:   letterID,
		RetryCount: uint32(3),
		Body:       []byte(body),
		Envelope:   envelope,
	}

	publisher.Publish(letter)

	// Assert on all Notifications
	select {
	case chanErr := <-channelPool.Errors():
		assert.NoError(t, chanErr) // This test fails on channel errors.
		break
	case notification := <-publisher.Notifications():
		assert.True(t, notification.Success)
		assert.Equal(t, letterID, notification.LetterID)
		assert.NoError(t, notification.Error)
		break
	default:
		time.Sleep(100 * time.Millisecond)
	}
}

func TestAutoPublish(t *testing.T) {
	Seasoning.Pools.ConnectionCount = 1
	Seasoning.Pools.ChannelCount = 2

	channelPool, err := pools.NewChannelPool(Seasoning, nil, true)
	assert.NoError(t, err)

	// Flush Any Errors
	select {
	case chanErr := <-channelPool.Errors():
		fmt.Print(chanErr)
	default:
		break
	}

	publisher, err := publisher.NewPublisher(Seasoning, channelPool, nil)
	assert.NoError(t, err)

	letter := CreateLetter("", "TestQueue", nil)

	publisher.StartAutoPublish()

	err = publisher.QueueLetter(letter)
	assert.NoError(t, err)

	publisher.StopAutoPublish(false)

	// Assert on all Notifications
	select {
	case chanErr := <-channelPool.Errors():
		assert.NoError(t, chanErr) // This test fails on channel errors.
		break
	case notification := <-publisher.Notifications():
		assert.True(t, notification.Success)
		assert.Equal(t, letter.LetterID, notification.LetterID)
		assert.NoError(t, notification.Error)
		break
	default:
		time.Sleep(100 * time.Millisecond)
	}
}
