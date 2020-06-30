package main_test

import (
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/houseofcat/turbocookedrabbit/pkg/tcr"
	"github.com/stretchr/testify/assert"
)

// TestConsumingAfterPublish is a combination test of Consuming and Publishing
func TestConsumingAfterPublish(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	timeoutAfter := time.After(time.Minute * 1)
	consumer, err := tcr.NewConsumerFromConfig(ConsumerConfig, ConnectionPool)
	assert.NoError(t, err)
	assert.NotNil(t, consumer)

	consumer.StartConsuming()

	publisher, err := tcr.NewPublisherFromConfig(Seasoning, ConnectionPool)
	assert.NoError(t, err)

	letter := tcr.CreateMockRandomLetter("TcrTestQueue")
	count := 1000 // higher will deadlock publisher since publisher receipts processing wont' be hit yet

	for i := 0; i < count; i++ {
		publisher.Publish(letter)
	}

	publishSuccessCount := 0
WaitForReceiptsLoop:
	for {
		select {
		case <-timeoutAfter:
			t.Fatal("test timeout")
		case receipt := <-publisher.PublishReceipts():
			if receipt.Success {
				publishSuccessCount++
				if publishSuccessCount == count {
					break WaitForReceiptsLoop
				}
			}
		default:
			time.Sleep(time.Millisecond * 1)
		}
	}

	assert.Equal(t, count, publishSuccessCount, "Publish Success Count: %d  Expected Count: %d", publishSuccessCount, count)

	receivedMessageCount := 0
WaitForConsumer:
	for {
		select {
		case <-timeoutAfter:
			t.Fatal("test timeout")
		case message := <-consumer.ReceivedMessages():
			message.Acknowledge()
			receivedMessageCount++
			if receivedMessageCount == count {
				break WaitForConsumer
			}
		default:
			time.Sleep(time.Millisecond * 1)
		}
	}
	assert.Equal(t, count, receivedMessageCount, "Received Message Count: %d  Expected Count: %d", receivedMessageCount, count)

	err = consumer.StopConsuming(false, false)
	assert.NoError(t, err)

	TestCleanup(t)
}

// TestLargeConsumingAfterLargePublish is a combination test of Consuming and Publishing
func TestLargeConsumingAfterLargePublish(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	timeoutAfter := time.After(time.Minute * 5)
	consumer, err := tcr.NewConsumerFromConfig(ConsumerConfig, ConnectionPool)
	assert.NoError(t, err)
	assert.NotNil(t, consumer)

	done1 := make(chan struct{}, 1)
	done2 := make(chan struct{}, 1)
	consumer.StartConsuming()

	publisher, err := tcr.NewPublisherFromConfig(Seasoning, ConnectionPool)
	assert.NoError(t, err)

	letter := tcr.CreateMockRandomLetter("TcrTestQueue")
	count := 1000000

	go monitorPublish(t, timeoutAfter, publisher, count, done1)
	go monitorConsumer(t, timeoutAfter, consumer, count, done2)

	for i := 0; i < count; i++ {
		publisher.Publish(letter)
	}

	<-done1
	<-done2
	err = consumer.StopConsuming(false, false)
	assert.NoError(t, err)

	TestCleanup(t)
}

func monitorPublish(t *testing.T, timeoutAfter <-chan time.Time, pub *tcr.Publisher, count int, done chan struct{}) {

	publishSuccessCount := 0
	publishFailureCount := 0

WaitForReceiptsLoop:
	for {
		select {
		case <-timeoutAfter:
			t.Fatal("test timeout")
		case receipt := <-pub.PublishReceipts():

			if receipt.Success {
				publishSuccessCount++
				if count == publishSuccessCount+publishFailureCount {
					break WaitForReceiptsLoop
				}
			} else {
				publishFailureCount++
			}

		default:
			time.Sleep(time.Millisecond * 1)
		}
	}

FlushRemainingReceiptsIfAny:
	for {
		select {
		case <-pub.PublishReceipts(): // prevent leaked go routines
		default:
			break FlushRemainingReceiptsIfAny
		}
	}

	actualCount := publishSuccessCount + publishFailureCount
	assert.Equal(t, count, actualCount, "Publish Success Count: %d  Expected Count: %d", publishSuccessCount, count)
	done <- struct{}{}
}

func monitorConsumer(t *testing.T, timeoutAfter <-chan time.Time, con *tcr.Consumer, count int, done chan struct{}) {

	receivedMessageCount := 0
WaitForConsumer:
	for {
		select {
		case <-timeoutAfter:
			t.Fatal("test timeout")

		case message := <-con.ReceivedMessages():

			message.Acknowledge()
			receivedMessageCount++
			if receivedMessageCount == count {
				break WaitForConsumer
			}

		default:
			time.Sleep(time.Millisecond * 1)
		}
	}
	assert.Equal(t, count, receivedMessageCount, "Received Message Count: %d  Expected Count: %d", receivedMessageCount, count)
	done <- struct{}{}
}

// TestLargeConsumingAfterLargePublishConfirmation is a combination test of Consuming and Publishing with confirmation.
func TestLargeConsumingAfterLargePublishConfirmation(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	timeoutAfter := time.After(time.Minute * 2)
	consumer, err := tcr.NewConsumerFromConfig(ConsumerConfig, ConnectionPool)
	assert.NoError(t, err)
	assert.NotNil(t, consumer)

	done1 := make(chan struct{}, 1)
	done2 := make(chan struct{}, 1)
	consumer.StartConsuming()

	publisher, err := tcr.NewPublisherFromConfig(Seasoning, ConnectionPool)
	assert.NoError(t, err)

	letter := tcr.CreateMockRandomLetter("TcrTestQueue")
	count := 10000

	go monitorPublish(t, timeoutAfter, publisher, count, done1)
	go monitorConsumer(t, timeoutAfter, consumer, count, done2)

	for i := 0; i < count; i++ {
		publisher.PublishWithConfirmation(letter, 500*time.Millisecond)
	}

	<-done1
	<-done2
	err = consumer.StopConsuming(false, false)
	assert.NoError(t, err)

	TestCleanup(t)
}

// TestLargePublishConfirmation is a combination test of Consuming and Publishing with confirmation.
func TestLargePublishConfirmation(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	timeoutAfter := time.After(time.Minute * 2)
	done1 := make(chan struct{}, 1)

	publisher, err := tcr.NewPublisherFromConfig(Seasoning, ConnectionPool)
	assert.NoError(t, err)

	letter := tcr.CreateMockRandomLetter("TcrTestQueue")
	count := 10000

	go monitorPublish(t, timeoutAfter, publisher, count, done1)

	for i := 0; i < count; i++ {
		publisher.PublishWithConfirmation(letter, 50*time.Millisecond)
	}

	<-done1
	assert.NoError(t, err)

	TestCleanup(t)
}
