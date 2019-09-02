package publisher_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/houseofcat/turbocookedrabbit/models"
	"github.com/houseofcat/turbocookedrabbit/pools"
	"github.com/houseofcat/turbocookedrabbit/publisher"
	"github.com/houseofcat/turbocookedrabbit/utils"
	"github.com/stretchr/testify/assert"
)

var Seasoning *models.RabbitSeasoning

func TestMain(m *testing.M) { // Load Configuration On Startup
	var err error
	Seasoning, err = utils.ConvertJSONFileToConfig("publisherseasoning.json")
	if err != nil {
		return
	}
	os.Exit(m.Run())
}

func TestCreatePublisher(t *testing.T) {
	channelPool, err := pools.NewChannelPool(Seasoning, nil, true)
	assert.NoError(t, err)

	channelPool.FlushErrors()

	publisher, err := publisher.NewPublisher(Seasoning, channelPool, nil)
	assert.NoError(t, err)
	assert.NotNil(t, publisher)
}

func TestCreatePublisherAndPublish(t *testing.T) {
	channelPool, err := pools.NewChannelPool(Seasoning, nil, true)
	assert.NoError(t, err)

	channelPool.FlushErrors()

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
AssertLoop:
	for {
		select {
		case chanErr := <-channelPool.Errors():
			assert.NoError(t, chanErr) // This test fails on channel errors.
			break AssertLoop
		case notification := <-publisher.Notifications():
			assert.True(t, notification.Success)
			assert.Equal(t, letterID, notification.LetterID)
			assert.NoError(t, notification.Error)
			break AssertLoop
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func TestAutoPublishSingleMessage(t *testing.T) {
	channelPool, err := pools.NewChannelPool(Seasoning, nil, true)
	assert.NoError(t, err)

	channelPool.FlushErrors()

	publisher, err := publisher.NewPublisher(Seasoning, channelPool, nil)
	assert.NoError(t, err)

	letter := utils.CreateLetter("", "TestQueue", nil)

	publisher.StartAutoPublish()

	err = publisher.QueueLetter(letter)
	assert.NoError(t, err)

	publisher.StopAutoPublish(false)

	// Assert on all Notifications
AssertLoop:
	for {
		select {
		case chanErr := <-channelPool.Errors():
			assert.NoError(t, chanErr) // This test fails on channel errors.
			break AssertLoop
		case notification := <-publisher.Notifications():
			assert.True(t, notification.Success)
			assert.Equal(t, letter.LetterID, notification.LetterID)
			assert.NoError(t, notification.Error)
			break AssertLoop
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func TestAutoPublishManyMessages(t *testing.T) {
	messageCount := 1000

	channelPool, err := pools.NewChannelPool(Seasoning, nil, true)
	assert.NoError(t, err)

	channelPool.FlushErrors()

	publisher, err := publisher.NewPublisher(Seasoning, channelPool, nil)
	assert.NoError(t, err)

	timeStart := time.Now()
	publisher.StartAutoPublish()

	go func() {
		for i := 0; i < messageCount; i++ {
			letter := utils.CreateLetter("", "TestQueue", nil)

			err = publisher.QueueLetter(letter)
			assert.NoError(t, err)
		}

		publisher.StopAutoPublish(false)
	}()

	successCount := 0
	failureCount := 0
	timer := time.NewTimer(1 * time.Minute)

ListeningForNotificationsLoop:
	for {
		select {
		case <-timer.C:
			break ListeningForNotificationsLoop
		case chanErr := <-channelPool.Errors():
			if chanErr != nil {
				failureCount++
			}
		case notification := <-publisher.Notifications():
			if notification.Success {
				successCount++
			} else {
				failureCount++
			}

			if successCount+failureCount == messageCount {
				break ListeningForNotificationsLoop
			}
		default:
			time.Sleep(1 * time.Millisecond)
		}
	}

	elapsed := time.Since(timeStart)

	assert.Equal(t, messageCount, successCount+failureCount)
	fmt.Printf("All Messages Accounted For: %d\r\n", successCount)
	fmt.Printf("Success Count: %d\r\n", successCount)
	fmt.Printf("Failure Count: %d\r\n", failureCount)
	fmt.Printf("Time Elapsed: %s\r\n", elapsed)
}
