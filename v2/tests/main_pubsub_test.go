package main_test

import (
	"sync"
	"testing"
	"time"

	"github.com/houseofcat/turbocookedrabbit/v2/pkg/tcr"
	"github.com/stretchr/testify/assert"
)

// TestConsumingAfterPublish is a combination test of Consuming and Publishing
func TestConsumingAfterPublish(t *testing.T) {
	cfg, closer := InitTestService(t)
	defer closer()

	timeoutAfter := time.After(time.Minute * 1)
	consumer := tcr.NewConsumerFromConfig(cfg.ConsumerConfig, cfg.ConnectionPool)
	assert.NotNil(t, consumer)

	consumer.Start()

	publisher := tcr.NewPublisherFromConfig(cfg.Seasoning, cfg.ConnectionPool)
	letter := tcr.CreateMockRandomLetter("TcrTestQueue")
	count := 1000 // higher will deadlock publisher since publisher receipts processing wont' be hit yet

	for i := 0; i < count; i++ {
		publisher.Publish(letter, false)
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
			_ = message.Acknowledge()
			receivedMessageCount++
			if receivedMessageCount == count {
				break WaitForConsumer
			}
		default:
			time.Sleep(time.Millisecond * 1)
		}
	}
	assert.Equal(t, count, receivedMessageCount, "Received Message Count: %d  Expected Count: %d", receivedMessageCount, count)

	consumer.Close()
}

// TestLargeConsumingAfterLargePublish is a combination test of Consuming and Publishing
func TestLargeConsumingAfterLargePublish(t *testing.T) {
	cfg, closer := InitTestService(t)
	defer closer()

	timeoutAfter := time.After(time.Minute * 5)
	consumer := tcr.NewConsumerFromConfig(cfg.ConsumerConfig, cfg.ConnectionPool)
	assert.NotNil(t, consumer)

	var wg sync.WaitGroup
	consumer.Start()

	publisher := tcr.NewPublisherFromConfig(cfg.Seasoning, cfg.ConnectionPool)
	letter := tcr.CreateMockRandomLetter("TcrTestQueue")
	count := 1000000

	wg.Add(2)
	go monitorPublish(t, timeoutAfter, publisher, count, &wg)
	go monitorConsumer(t, timeoutAfter, consumer, count, &wg)

	for i := 0; i < count; i++ {
		publisher.Publish(letter, false)
	}

	wg.Wait()
	consumer.Close()

}

// TestLargeConsumingAfterLargePublishConfirmation is a combination test of Consuming and Publishing with confirmation.
func TestLargeConsumingAfterLargePublishConfirmation(t *testing.T) {
	cfg, closer := InitTestService(t)
	defer closer()

	timeoutAfter := time.After(time.Minute * 2)
	consumer := tcr.NewConsumerFromConfig(cfg.ConsumerConfig, cfg.ConnectionPool)
	assert.NotNil(t, consumer)

	consumer.Start()

	publisher := tcr.NewPublisherFromConfig(cfg.Seasoning, cfg.ConnectionPool)
	letter := tcr.CreateMockRandomLetter("TcrTestQueue")
	count := 10000

	var wg sync.WaitGroup

	wg.Add(2)
	go monitorPublish(t, timeoutAfter, publisher, count, &wg)
	go monitorConsumer(t, timeoutAfter, consumer, count, &wg)

	for i := 0; i < count; i++ {
		publisher.PublishWithConfirmation(letter, 500*time.Millisecond)
	}

	wg.Wait()
	consumer.Close()
}

// TestLargePublishConfirmation is a combination test of Consuming and Publishing with confirmation.
func TestLargePublishConfirmation(t *testing.T) {
	cfg, closer := InitTestService(t)
	defer closer()

	timeoutAfter := time.After(time.Minute * 2)

	publisher := tcr.NewPublisherFromConfig(cfg.Seasoning, cfg.ConnectionPool)
	letter := tcr.CreateMockRandomLetter("TcrTestQueue")
	count := 10000

	var wg sync.WaitGroup
	wg.Add(1)

	go monitorPublish(t, timeoutAfter, publisher, count, &wg)

	for i := 0; i < count; i++ {
		publisher.PublishWithConfirmation(letter, 50*time.Millisecond)
	}

	wg.Wait()
}

func monitorPublish(t *testing.T, timeoutAfter <-chan time.Time, pub *tcr.Publisher, count int, wg *sync.WaitGroup) {
	defer wg.Done()

	publishSuccessCount := 0
	publishFailureCount := 0

WaitForReceiptsLoop:
	for {
		select {
		case <-timeoutAfter:
			return
		case receipt := <-pub.PublishReceipts():

			if receipt.Success {
				publishSuccessCount++
				if count == publishSuccessCount+publishFailureCount {
					break WaitForReceiptsLoop
				}
			} else {
				publishFailureCount++
				t.Error("receipt.Succeed is false")
				break WaitForReceiptsLoop
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
}

func monitorConsumer(t *testing.T, timeoutAfter <-chan time.Time, con *tcr.Consumer, count int, wg *sync.WaitGroup) {
	defer wg.Done()

	receivedMessageCount := 0
WaitForConsumer:
	for {
		select {
		case <-timeoutAfter:
			return

		case message := <-con.ReceivedMessages():

			_ = message.Acknowledge()
			receivedMessageCount++
			if receivedMessageCount == count {
				break WaitForConsumer
			}

		default:
			time.Sleep(time.Millisecond * 1)
		}
	}
	assert.Equal(t, count, receivedMessageCount, "Received Message Count: %d  Expected Count: %d", receivedMessageCount, count)

}
