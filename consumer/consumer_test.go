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
var ConnectionPool *pools.ConnectionPool
var ChannelPool *pools.ChannelPool

func TestMain(m *testing.M) { // Load Configuration On Startup
	var err error
	Seasoning, err = utils.ConvertJSONFileToConfig("testconsumerseasoning.json")
	if err != nil {
		return
	}
	ConnectionPool, err = pools.NewConnectionPool(Seasoning.PoolConfig, true)
	if err != nil {
		fmt.Print(err.Error())
		return
	}
	ChannelPool, err = pools.NewChannelPool(Seasoning.PoolConfig, ConnectionPool, true)
	if err != nil {
		fmt.Print(err.Error())
		return
	}

	os.Exit(m.Run())
}

func TestCreateConsumer(t *testing.T) {
	channelPool, err := pools.NewChannelPool(Seasoning.PoolConfig, nil, true)
	assert.NoError(t, err)

	channelPool.FlushErrors()

	consumerConfig, ok := Seasoning.ConsumerConfigs["TurboCookedRabbitConsumer-AutoAck"]
	assert.True(t, ok)

	con1, err1 := consumer.NewConsumerFromConfig(consumerConfig, channelPool)
	assert.NoError(t, err1)
	assert.NotNil(t, con1)

	con2, err2 := consumer.NewConsumer(
		Seasoning,
		channelPool,
		"ConsumerTestQueue",
		"MyConsumerName",
		false,
		false,
		false,
		nil,
		0,
		10,
		2,
		5,
		5,
	)
	assert.NoError(t, err2)
	assert.NotNil(t, con2)
}

func TestCreateConsumerAndGet(t *testing.T) {
	channelPool, err := pools.NewChannelPool(Seasoning.PoolConfig, nil, true)
	assert.NoError(t, err)

	channelPool.FlushErrors()

	consumerConfig, ok := Seasoning.ConsumerConfigs["TurboCookedRabbitConsumer-AutoAck"]
	assert.True(t, ok)

	consumer, err := consumer.NewConsumerFromConfig(consumerConfig, channelPool)
	assert.NoError(t, err)
	assert.NotNil(t, consumer)

	_, err = consumer.Get("ConsumerTestQueue", true)
	assert.NoError(t, err)

	_, err = consumer.Get("ConsumerTestQueue", false)
	assert.NoError(t, err)

	_, err = consumer.Get("ConsumerTestQueue2", false)
	assert.Error(t, err)
}

func TestCreateConsumerAndGetBatch(t *testing.T) {
	channelPool, err := pools.NewChannelPool(Seasoning.PoolConfig, nil, true)
	assert.NoError(t, err)

	channelPool.FlushErrors()

	consumerConfig, ok := Seasoning.ConsumerConfigs["TurboCookedRabbitConsumer-AutoAck"]
	assert.True(t, ok)

	consumer, err := consumer.NewConsumerFromConfig(consumerConfig, channelPool)
	assert.NoError(t, err)
	assert.NotNil(t, consumer)

	_, err = consumer.GetBatch("ConsumerTestQueue", 10, true)
	assert.NoError(t, err)

	_, err = consumer.GetBatch("ConsumerTestQueue", 10, false)
	assert.NoError(t, err)

	_, err = consumer.GetBatch("ConsumerTestQueue", -1, false)
	assert.Error(t, err)
}

func TestCreateConsumerAndPublisher(t *testing.T) {
	channelPool, err := pools.NewChannelPool(Seasoning.PoolConfig, nil, true)
	assert.NoError(t, err)

	channelPool.FlushErrors()

	publisher, err := publisher.NewPublisher(Seasoning, channelPool, nil)
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

	channelPool, err := pools.NewChannelPool(Seasoning.PoolConfig, nil, true)
	assert.NoError(t, err)

	channelPool.FlushErrors()

	publisher, err := publisher.NewPublisher(Seasoning, channelPool, nil)
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

	channelPool, err := pools.NewChannelPool(Seasoning.PoolConfig, nil, true)
	assert.NoError(t, err)

	channelPool.FlushErrors()

	publisher, err := publisher.NewPublisher(Seasoning, channelPool, nil)
	assert.NoError(t, err)
	assert.NotNil(t, publisher)

	consumerConfig, ok := Seasoning.ConsumerConfigs["TurboCookedRabbitConsumer-AutoAck"]
	assert.True(t, ok)

	consumer, err := consumer.NewConsumerFromConfig(consumerConfig, channelPool)
	assert.NoError(t, err)
	assert.NotNil(t, consumer)

	publisher.StartAutoPublish(false)

	publisher.QueueLetter(utils.CreateMockRandomLetter("ConsumerTestQueue"))

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
			consumer.StopConsuming(false, true)
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

	t.Logf("%s: Benchmark started...", time.Now())

	messagesReceived := 0
	messagesPublished := 0
	messagesFailedToPublish := 0
	consumerErrors := 0
	channelPoolErrors := 0
	messageCount := 200000

	channelPool, _ := pools.NewChannelPool(Seasoning.PoolConfig, nil, true)
	publisher, _ := publisher.NewPublisher(Seasoning, channelPool, nil)
	consumerConfig, _ := Seasoning.ConsumerConfigs["TurboCookedRabbitConsumer-AutoAck"]
	consumer, _ := consumer.NewConsumerFromConfig(consumerConfig, channelPool)

	channelPool.FlushErrors()

	publisher.StartAutoPublish(false)

	letters := make([]*models.Letter, messageCount)
	for i := 0; i < messageCount; i++ {
		letters[i] = utils.CreateMockRandomLetter("ConsumerTestQueue")
	}

	for i := 0; i < len(letters); i++ {
		publisher.QueueLetter(letters[i])
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(2*time.Minute))

MonitorMessages:
	for {
		select {
		case <-ctx.Done():
			t.Logf("%s\r\nContextTimeout\r\n", time.Now())
			break MonitorMessages
		case notice := <-publisher.Notifications():
			if notice.Success {
				messagesPublished++
			} else {
				messagesFailedToPublish++
				t.Logf("%s: Message [ID: %d] failed to publish.", time.Now(), notice.LetterID)
			}

			if messagesPublished+messagesFailedToPublish == messageCount {
				break MonitorMessages
			}
		}
	}

	letters = nil // release memory

	startTime := time.Now()
	consumer.StartConsuming()

ConsumeMessages:
	for {
		select {
		case <-ctx.Done():
			t.Logf("%s\r\nContextTimeout\r\n", time.Now())
			break ConsumeMessages
		case err := <-consumer.Errors():
			if err != nil {
				t.Log(err)
			}
			consumerErrors++
		case <-consumer.Messages():
			messagesReceived++
		case err := <-channelPool.Errors():
			if err != nil {
				t.Log(err)
			}
			channelPoolErrors++
		default:
			time.Sleep(100 * time.Nanosecond)
			break
		}

		if messagesReceived+messagesFailedToPublish >= messageCount {
			break ConsumeMessages
		}
	}
	elapsedTime := time.Since(startTime)

	assert.Equal(t, messageCount, messagesReceived+messagesFailedToPublish)
	t.Logf("%s: Test finished, elapsed time: %f s", time.Now(), elapsedTime.Seconds())
	t.Logf("%s: Consumer Rate: %f msgs/s", time.Now(), (float64(messagesReceived) / elapsedTime.Seconds()))
	t.Logf("%s: Channel Pool Errors: %d\r\n", time.Now(), channelPoolErrors)
	t.Logf("%s: Consumer Errors: %d\r\n", time.Now(), consumerErrors)
	t.Logf("%s: Messages Published: %d\r\n", time.Now(), messagesPublished)
	t.Logf("%s: Messages Failed to Publish: %d\r\n", time.Now(), messagesFailedToPublish)
	t.Logf("%s: Messages Received: %d\r\n", time.Now(), messagesReceived)

	publisher.StopAutoPublish()
	consumer.StopConsuming(false, true)
	channelPool.Shutdown()
	cancel()
}
