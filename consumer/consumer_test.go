package consumer_test

import (
	"fmt"
	"os"
	"testing"

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

	var notice *models.Notification
	var message *models.Message

ConsumeMessages:
	for {
		select {
		case notice = <-publisher.Notifications():
			publisher.StopAutoPublish()
		case message = <-consumer.Messages():
			consumer.StopConsuming(false)
			break ConsumeMessages
		case err := <-consumer.Errors():
			assert.NoError(t, err)
		case err := <-channelPool.Errors():
			assert.NoError(t, err)
		default:
			break
		}
	}

	assert.True(t, notice.Success)
	assert.NotNil(t, message)

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
