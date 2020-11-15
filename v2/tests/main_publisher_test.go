package main_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/houseofcat/turbocookedrabbit/v2/pkg/tcr"
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
		letters[i] = tcr.CreateMockLetter("", fmt.Sprintf("TestQueue-%d", i%10), nil)
	}

	elapsed := time.Since(timeStart)
	t.Logf("Time Elapsed Creating Letters: %s\r\n", elapsed)

	// Test
	timeStart = time.Now()
	amqpConn, err := amqp.Dial(Seasoning.PoolConfig.URI)
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
				MessageId:   letter.LetterID.String(),
				Timestamp:   time.Now().UTC(),
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

	publisher := tcr.NewPublisherFromConfig(Seasoning, ConnectionPool)

	letter := tcr.CreateMockRandomLetter("TcrTestQueue")
	publisher.Publish(letter, false)

	ConnectionPool.Shutdown()
}

func TestPublishAndWaitForReceipt(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	publisher := tcr.NewPublisherFromConfig(Seasoning, ConnectionPool)

	letter := tcr.CreateMockRandomLetter("TcrTestQueue")
	publisher.Publish(letter, false)

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

	TestCleanup(t)
}

func TestCreatePublisherAndPublishWithConfirmation(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	publisher := tcr.NewPublisherFromConfig(Seasoning, ConnectionPool)

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

	TestCleanup(t)
}

func TestPublishAccuracy(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	t1 := time.Now()
	fmt.Printf("Benchmark Starts: %s\r\n", t1)
	publisher := tcr.NewPublisherFromConfig(Seasoning, ConnectionPool)

	letter := tcr.CreateMockRandomLetter("TcrTestQueue")
	letter.Envelope.DeliveryMode = amqp.Transient
	count := 100000

	for i := 0; i < count; i++ {
		publisher.Publish(letter, false)
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

	t2 := time.Now()
	diff := t2.Sub(t1)
	fmt.Printf("Benchmark End: %s\r\n", t2)
	fmt.Printf("Messages: %f msg/s\r\n", float64(count)/diff.Seconds())
	TestCleanup(t)
}

func TestPublishWithConfirmationAccuracy(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	publisher := tcr.NewPublisherFromConfig(Seasoning, ConnectionPool)

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

	TestCleanup(t)
}
