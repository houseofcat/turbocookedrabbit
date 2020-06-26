package main_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/houseofcat/turbocookedrabbit/pkg/tcr"
	"github.com/stretchr/testify/assert"
)

var Seasoning *tcr.RabbitSeasoning
var ConnectionPool *tcr.ConnectionPool
var AckableConsumerConfig *tcr.ConsumerConfig
var ConsumerConfig *tcr.ConsumerConfig

func TestMain(m *testing.M) {

	var err error
	Seasoning, err = tcr.ConvertJSONFileToConfig("testseasoning.json") // Load Configuration On Startup
	if err != nil {
		return
	}
	ConnectionPool, err = tcr.NewConnectionPool(Seasoning.PoolConfig)
	if err != nil {
		fmt.Print(err.Error())
		return
	}

	if config, ok := Seasoning.ConsumerConfigs["TurboCookedRabbitConsumer-Ackable"]; ok {
		AckableConsumerConfig = config
	}

	if config, ok := Seasoning.ConsumerConfigs["TurboCookedRabbitConsumer"]; ok {
		ConsumerConfig = config
	}

	os.Exit(m.Run())
}

// TestConsumingAfterPublish is a combination test of Consuming and Publishing
func TestConsumingAfterPublish(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	timeoutAfter := time.After(time.Minute * 1)
	consumer, err := tcr.NewConsumerFromConfig(ConsumerConfig, ConnectionPool)
	assert.NoError(t, err)
	assert.NotNil(t, consumer)

	consumer.StartConsuming()

	publisher, err := tcr.NewPublisherWithConfig(Seasoning, ConnectionPool)
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

	ConnectionPool.Shutdown()
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

	publisher, err := tcr.NewPublisherWithConfig(Seasoning, ConnectionPool)
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

	ConnectionPool.Shutdown()
}

func monitorPublish(t *testing.T, timeoutAfter <-chan time.Time, pub *tcr.Publisher, count int, done chan struct{}) {

	publishSuccessCount := 0
	publishFailureCount := 0

WaitForReceiptsLoop:
	for {
		select {
		case <-timeoutAfter:
			t.Fatal("test timeout")
		case <-pub.Errors():

		case receipt := <-pub.PublishReceipts():

			if receipt.Success {
				publishSuccessCount++
				if publishSuccessCount == count {
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
