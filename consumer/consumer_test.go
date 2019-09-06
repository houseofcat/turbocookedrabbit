package consumer_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/assert"

	"github.com/houseofcat/turbocookedrabbit/consumer"
	"github.com/houseofcat/turbocookedrabbit/models"
	"github.com/houseofcat/turbocookedrabbit/pools"
	"github.com/houseofcat/turbocookedrabbit/publisher"
	"github.com/houseofcat/turbocookedrabbit/utils"
)

var Seasoning *models.RabbitSeasoning

func TestMain(m *testing.M) { // Load Configuration On Startup
	var err error
	Seasoning, err = utils.ConvertJSONFileToConfig("testconsumerseasoning.json")
	if err != nil {
		return
	}
	os.Exit(m.Run())
}

func TestCreateConsumerAndPublisher(t *testing.T) {
	channelPool, err := pools.NewChannelPool(Seasoning, nil, true)
	assert.NoError(t, err)

	channelPool.FlushErrors()

	publisher, err := publisher.NewPublisher(Seasoning, channelPool, nil, 1)
	assert.NoError(t, err)
	assert.NotNil(t, publisher)

	consumerConfig, ok := Seasoning.ConsumerConfigs["TurboCookedRabbitConsumer-AutoAck"]
	assert.True(t, ok)

	consumer, err := consumer.NewConsumerFromConfig(consumerConfig, channelPool)
	assert.NoError(t, err)
	assert.NotNil(t, consumer)
}

func TestCreateConsumerAndUncleanShutdown(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	channelPool, err := pools.NewChannelPool(Seasoning, nil, true)
	assert.NoError(t, err)

	channelPool.FlushErrors()

	publisher, err := publisher.NewPublisher(Seasoning, channelPool, nil, 1)
	assert.NoError(t, err)
	assert.NotNil(t, publisher)

	consumerConfig, ok := Seasoning.ConsumerConfigs["TurboCookedRabbitConsumer-AutoAck"]
	assert.True(t, ok)

	consumer, err := consumer.NewConsumerFromConfig(consumerConfig, channelPool)
	assert.NoError(t, err)
	assert.NotNil(t, consumer)

	channelPool.Shutdown()
ErrorLoop:
	for {
		select {
		case notice := <-publisher.Notifications():
			fmt.Print(notice.ToString())
		case err := <-consumer.Errors():
			fmt.Printf("%s\r\n", err)
		case err := <-channelPool.Errors():
			fmt.Printf("%s\r\n", err)
		default:
			break ErrorLoop
		}
	}
}

func TestPublishAndConsume(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	channelPool, err := pools.NewChannelPool(Seasoning, nil, true)
	assert.NoError(t, err)

	channelPool.FlushErrors()

	publisher, err := publisher.NewPublisher(Seasoning, channelPool, nil, 1)
	assert.NoError(t, err)
	assert.NotNil(t, publisher)

	consumerConfig, ok := Seasoning.ConsumerConfigs["TurboCookedRabbitConsumer-AutoAck"]
	assert.True(t, ok)

	consumer, err := consumer.NewConsumerFromConfig(consumerConfig, channelPool)
	assert.NoError(t, err)
	assert.NotNil(t, consumer)

	publisher.StartAutoPublish(false)

	err = publisher.QueueLetter(utils.CreateLetter("", "ConsumerTestQueue", nil))
	assert.NoError(t, err)

	err = consumer.StartConsuming()
	assert.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(1)*time.Second)

ConsumeMessages:
	for {
		select {
		case <-ctx.Done():
			fmt.Print("\r\nContextTimeout\r\n")
			break ConsumeMessages
		case notice := <-publisher.Notifications():
			fmt.Printf("UpperLoop: %s\r\n", notice.ToString())
		case message := <-consumer.Messages():
			fmt.Printf("Message Received: %s\r\n", string(message.Body))
			consumer.StopConsuming(false)
			break ConsumeMessages
		case err := <-consumer.Errors():
			assert.NoError(t, err)
		case err := <-channelPool.Errors():
			assert.NoError(t, err)
		default:
			time.Sleep(100 * time.Millisecond)
			break
		}
	}

	publisher.StopAutoPublish()
	channelPool.Shutdown()

ErrorLoop:
	for {
		select {
		case notice := <-publisher.Notifications():
			fmt.Printf("LowerLoop: %s", notice.ToString())
		case err := <-consumer.Errors():
			fmt.Printf("%s\r\n", err)
		case err := <-channelPool.Errors():
			fmt.Printf("%s\r\n", err)
		default:
			break ErrorLoop
		}
	}

	cancel()
}
