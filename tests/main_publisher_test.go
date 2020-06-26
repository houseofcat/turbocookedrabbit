package main_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/houseofcat/turbocookedrabbit/pkg/tcr"
	"github.com/streadway/amqp"
	"github.com/stretchr/testify/assert"
)

// TestBasicPublish is used for some baseline numbers using primarily streadway/amqp.
func TestBasicPublish(t *testing.T) {

	defer leaktest.Check(t)()

	messageCount := 1000

	// Pre-create test messages
	timeStart := time.Now()
	letters := make([]*tcr.Letter, messageCount)

	for i := 0; i < messageCount; i++ {
		letters[i] = tcr.CreateMockLetter(uint64(i), "", fmt.Sprintf("TestQueue-%d", i%10), nil)
	}

	elapsed := time.Since(timeStart)
	t.Logf("Time Elapsed Creating Letters: %s\r\n", elapsed)

	// Test
	timeStart = time.Now()
	amqpConn, err := amqp.Dial(Seasoning.PoolConfig.ConnectionPoolConfig.URI)
	if err != nil {
		return
	}

	amqpChan, err := amqpConn.Channel()
	if err != nil {
		return
	}

	for i := 0; i < messageCount; i++ {
		letter := letters[i]

		err = amqpChan.Publish(
			letter.Envelope.Exchange,
			letter.Envelope.RoutingKey,
			letter.Envelope.Mandatory,
			letter.Envelope.Immediate,
			amqp.Publishing{
				ContentType: letter.Envelope.ContentType,
				Body:        letter.Body,
			})

		if err != nil {
			t.Log(err)
		}

	}

	elapsed = time.Since(timeStart)
	t.Logf("Publish Time: %s\r\n", elapsed)
	t.Logf("Rate: %f msg/s\r\n", float64(messageCount)/elapsed.Seconds())
}

func TestCreatePublisherAndPublish(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	publisher, err := tcr.NewPublisherWithConfig(Seasoning, ConnectionPool)
	assert.NoError(t, err)

	letter := tcr.CreateMockRandomLetter("TcrTestQueue")
	publisher.Publish(letter)

	ConnectionPool.Shutdown()
}

func TestPublishAndWaitForReceipt(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	publisher, err := tcr.NewPublisherWithConfig(Seasoning, ConnectionPool)
	assert.NoError(t, err)

	letter := tcr.CreateMockRandomLetter("TcrTestQueue")
	publisher.Publish(letter)

WaitLoop:
	for {
		select {
		case receipt := <-publisher.PublishReceipts():
			assert.Equal(t, receipt.Success, true)
			break WaitLoop
		default:
			time.Sleep(time.Millisecond * 1)
		}
	}

	ConnectionPool.Shutdown()
}

func TestCreatePublisherAndPublishWithConfirmation(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	publisher, err := tcr.NewPublisherWithConfig(Seasoning, ConnectionPool)
	assert.NoError(t, err)

	letter := tcr.CreateMockRandomLetter("TcrTestQueue")
	publisher.PublishWithConfirmation(letter, time.Millisecond*500)

WaitLoop:
	for {
		select {
		case receipt := <-publisher.PublishReceipts():
			assert.Equal(t, receipt.Success, true)
			break WaitLoop
		default:
			time.Sleep(time.Millisecond * 1)
		}
	}

	ConnectionPool.Shutdown()
}

func TestPublishAccuracy(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	publisher, err := tcr.NewPublisherWithConfig(Seasoning, ConnectionPool)
	assert.NoError(t, err)

	letter := tcr.CreateMockRandomLetter("TcrTestQueue")
	count := 1000

	for i := 0; i < count; i++ {
		publisher.Publish(letter)
	}

	successCount := 0
WaitLoop:
	for {
		select {
		case receipt := <-publisher.PublishReceipts():
			if receipt.Success {
				successCount++
				if successCount == count {
					break WaitLoop
				}
			}
		default:
			time.Sleep(time.Millisecond * 1)
		}
	}

	assert.Equal(t, count, successCount)

	ConnectionPool.Shutdown()
}

func TestPublishWithConfirmationAccuracy(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	publisher, err := tcr.NewPublisherWithConfig(Seasoning, ConnectionPool)
	assert.NoError(t, err)

	letter := tcr.CreateMockRandomLetter("TcrTestQueue")
	count := 1000

	for i := 0; i < count; i++ {
		publisher.PublishWithConfirmation(letter, time.Millisecond*500)
	}

	successCount := 0
WaitLoop:
	for {
		select {
		case receipt := <-publisher.PublishReceipts():
			if receipt.Success {
				successCount++
				if successCount == count {
					break WaitLoop
				}
			}
		default:
			time.Sleep(time.Millisecond * 1)
		}
	}

	assert.Equal(t, count, successCount)

	ConnectionPool.Shutdown()
}
