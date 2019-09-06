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

func TestPublishAndConsumeMany(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	messageCount := 1000
	channelPool, _ := pools.NewChannelPool(Seasoning, nil, true)
	publisher, _ := publisher.NewPublisher(Seasoning, channelPool, nil, 1)
	consumerConfig, _ := Seasoning.ConsumerConfigs["TurboCookedRabbitConsumer-AutoAck"]
	consumer, _ := consumer.NewConsumerFromConfig(consumerConfig, channelPool)

	channelPool.FlushErrors()

	publisher.StartAutoPublish(false)

	letters := make([]*models.Letter, messageCount)
	for i := 0; i < messageCount; i++ {
		letters[i] = utils.CreateLetter("", "ConsumerTestQueue", nil)
	}

	go func() {
		for _, letter := range letters {
			publisher.QueueLetter(letter)
		}
	}()

	consumer.StartConsuming()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(2*time.Second))
	messagesReceived := 0
	messagesFailedToPublish := 0
	consumerErrors := 0
	channelPoolErrors := 0

ConsumeMessages:
	for {
		select {
		case <-ctx.Done():
			fmt.Print("\r\nContextTimeout\r\n")
			break ConsumeMessages
		case notice := <-publisher.Notifications():
			if !notice.Success {
				messagesFailedToPublish++
			}
		case <-consumer.Errors():
			consumerErrors++
		case <-consumer.Messages():
			messagesReceived++
		case <-channelPool.Errors():
			channelPoolErrors++
		default:
			time.Sleep(100 * time.Millisecond)
			break
		}

		if messagesReceived+messagesFailedToPublish == messageCount {
			break ConsumeMessages
		}
	}

	assert.Equal(t, messageCount, messagesReceived+messagesFailedToPublish)
	fmt.Printf("Channel Pool Errors: %d\r\n", channelPoolErrors)
	fmt.Printf("Consumer Errors: %d\r\n", consumerErrors)
	fmt.Printf("Messages Received: %d\r\n", messagesReceived)
	fmt.Printf("Messages Failed to Publish: %d\r\n", messagesFailedToPublish)

	consumer.StopConsuming(true)
	consumer.FlushMessages()
	publisher.StopAutoPublish()
	channelPool.Shutdown()
	cancel()
}

func TestPublishAndManyConsumers(t *testing.T) {
	/* 	defer leaktest.Check(t)() // Fail on leaked goroutines.

		messageCount := 1000
		channelPool1, _ := pools.NewChannelPool(Seasoning, nil, true)
		channelPool2, _ := pools.NewChannelPool(Seasoning, nil, true)
		publisher, _ := publisher.NewPublisher(Seasoning, channelPool1, nil, 1)

		channelPool1.FlushErrors()
		channelPool2.FlushErrors()

		publisher.StartAutoPublish(false)

		consumers := make([]*consumer.Consumer, 10)
		for i := 0; i < 10; i++ {
			consumerConfig, _ := Seasoning.ConsumerConfigs["TurboCookedRabbitConsumer-AutoAck"]
			consumerConfig.QueueName += fmt.Sprint("-%d", i)
			consumer, ok := consumer.NewConsumerFromConfig(consumerConfig, channelPool2)
			if ok {
				consumers[i] = consumer
			}
		}

		letters := make([]*models.Letter, messageCount)
		for i := 0; i < messageCount; i++ {
			letters[i] = utils.CreateLetter("", fmt.Sprintf("ConsumerTestQueue-", i%10), nil)
		}

		go func() {
			for _, letter := range letters {
				publisher.QueueLetter(letter)
			}
		}()

		consumer.StartConsuming()

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(10*time.Second))
		messagesReceived := 0
		messagesFailedToPublish := 0
		consumerErrors := 0
		channelPoolErrors := 0

	ConsumeMessages:
		for {
			select {
			case <-ctx.Done():
				fmt.Print("\r\nContextTimeout\r\n")
				break ConsumeMessages
			case notice := <-publisher.Notifications():
				if !notice.Success {
					messagesFailedToPublish++
				}
			case <-consumers.Errors():
				consumerErrors++
			case <-consumer.Messages():
				messagesReceived++
			case <-channelPool1.Errors():
				channelPoolErrors++
			case <-channelPool2.Errors():
				channelPoolErrors++
			default:
				time.Sleep(100 * time.Millisecond)
				break
			}
		}

		assert.Equal(t, messageCount, messagesReceived+messagesFailedToPublish)
		fmt.Printf("Channel Pool Errors: %d\r\n", channelPoolErrors)
		fmt.Printf("Consumer Errors: %d\r\n", consumerErrors)
		fmt.Printf("Messages Received: %d\r\n", messagesReceived)
		fmt.Printf("Messages Failed to Publish: %d\r\n", messagesFailedToPublish)

		consumer.StopConsuming(false)
		publisher.StopAutoPublish()
		channelPool1.Shutdown()
		channelPool2.Shutdown()
		cancel() */
}
