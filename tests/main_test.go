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

	consumer, err := tcr.NewConsumerFromConfig(ConsumerConfig, ConnectionPool)
	assert.NoError(t, err)
	assert.NotNil(t, consumer)

	consumer.StartConsuming()

	publisher, err := tcr.NewPublisherWithConfig(Seasoning, ConnectionPool)
	assert.NoError(t, err)

	letter := tcr.CreateMockRandomLetter("TcrTestQueue")
	count := 1000

	for i := 0; i < count; i++ {
		publisher.Publish(letter)
	}

	publishSuccessCount := 0
WaitForReceiptsLoop:
	for {
		select {
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
