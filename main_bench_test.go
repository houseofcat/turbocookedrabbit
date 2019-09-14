package main_test

import (
	"context"
	"fmt"
	"runtime/trace"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/houseofcat/turbocookedrabbit/consumer"
	"github.com/houseofcat/turbocookedrabbit/models"
	"github.com/houseofcat/turbocookedrabbit/pools"
	"github.com/houseofcat/turbocookedrabbit/publisher"
	"github.com/houseofcat/turbocookedrabbit/utils"
)

func BenchmarkPublishAndConsumeMany(b *testing.B) {
	b.ReportAllocs()
	ctx, task := trace.NewTask(context.Background(), "BenchmarkPublishAndConsumeMany")
	defer task.End()

	fmt.Printf("Benchmark Starts: %s\r\n", time.Now())
	messageCount := 1000
	channelPool, _ := pools.NewChannelPool(Seasoning.PoolConfig, nil, true)
	publisher, _ := publisher.NewPublisher(Seasoning, channelPool, nil)
	consumerConfig, _ := Seasoning.ConsumerConfigs["TurboCookedRabbitConsumer-AutoAck"]
	consumer, _ := consumer.NewConsumerFromConfig(consumerConfig, channelPool)

	channelPool.FlushErrors()

	publisher.StartAutoPublish(false)

	counter := uint64(0)

	go func() {
		for i := 0; i < messageCount; i++ {
			letter := utils.CreateLetter("", "ConsumerTestQueue", nil)
			letter.LetterID = counter
			counter++

			go publisher.QueueLetter(letter)
			//fmt.Printf("%s: Letter Queued - LetterID: %d\r\n", time.Now(), letter.LetterID)

			time.Sleep(1 * time.Millisecond)
		}
	}()

	consumer.StartConsuming()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(1*time.Minute))
	messagesReceived := 0
	messagesPublished := 0
	messagesFailedToPublish := 0
	consumerErrors := 0
	channelPoolErrors := 0

ReceivePublishConfirmations:
	for {
		select {
		case <-ctx.Done():
			fmt.Print("\r\nContextTimeout\r\n")
			break ReceivePublishConfirmations
		case notice := <-publisher.Notifications():
			if notice.Success {
				//fmt.Printf("%s: Published Success - LetterID: %d\r\n", time.Now(), notice.LetterID)
				messagesPublished++
			} else {
				//fmt.Printf("%s: Published Failed - LetterID: %d\r\n", time.Now(), notice.LetterID)
				messagesFailedToPublish++
			}
		case err := <-channelPool.Errors():
			fmt.Printf("%s: ChannelPool - Error: %s\r\n", time.Now(), err)
			channelPoolErrors++
		case err := <-consumer.Errors():
			fmt.Printf("%s: Consumer - Error: %s\r\n", time.Now(), err)
			consumerErrors++
		case <-consumer.Messages():
			//fmt.Printf("%s: MessageReceived\r\n", time.Now())
			messagesReceived++
		default:
			time.Sleep(5 * time.Millisecond)
			break
		}

		if messagesReceived+messagesFailedToPublish == messageCount {
			break ReceivePublishConfirmations
		}
	}

	assert.Equal(b, messageCount, messagesReceived+messagesFailedToPublish)
	fmt.Printf("Channel Pool Errors: %d\r\n", channelPoolErrors)
	fmt.Printf("Messages Published: %d\r\n", messagesPublished)
	fmt.Printf("Messages Failed to Publish: %d\r\n", messagesFailedToPublish)
	fmt.Printf("Consumer Errors: %d\r\n", consumerErrors)
	fmt.Printf("Consumer Messages Received: %d\r\n", messagesReceived)

	consumer.StopConsuming(true, true)
	publisher.StopAutoPublish()
	channelPool.Shutdown()
	cancel()
}

func BenchmarkPublishConsumeAckForDuration(b *testing.B) {
	b.ReportAllocs()

	timeDuration := time.Duration(5 * time.Minute)
	timeOut := time.After(timeDuration)
	fmt.Printf("Benchmark Starts: %s\r\n", time.Now())
	fmt.Printf("Est. Benchmark End: %s\r\n", time.Now().Add(timeDuration))

	publisher, _ := publisher.NewPublisher(Seasoning, ChannelPool, nil)
	consumerConfig, _ := Seasoning.ConsumerConfigs["TurboCookedRabbitConsumer-Ackable"]
	consumer, _ := consumer.NewConsumerFromConfig(consumerConfig, ChannelPool)

	publisher.StartAutoPublish(false)

	letter := utils.CreateLetter("", "ConsumerTestQueue", nil)

	go func() {
	PublishLoop:
		for {
			select {
			case <-timeOut:
				break PublishLoop
			default:
				newLetter := models.Letter(*letter)
				publisher.QueueLetter(&newLetter)
				//fmt.Printf("%s: Letter Queued - LetterID: %d\r\n", time.Now(), newLetter.LetterID)
				letter.LetterID++
				time.Sleep(5 * time.Microsecond)
			}
		}
	}()

	consumer.StartConsuming()

	messagesReceived := 0
	messagesPublished := 0
	messagesFailedToPublish := 0
	messagesAcked := 0
	messagesFailedToAck := 0
	consumerErrors := 0
	channelPoolErrors := 0
	connectionPoolErrors := 0

ConsumeLoop:
	for {
		select {
		case <-timeOut:
			break ConsumeLoop
		case notice := <-publisher.Notifications():
			if notice.Success {
				messagesPublished++
				notice = nil
			} else {
				messagesFailedToPublish++
				notice = nil
			}
		case err := <-ChannelPool.Errors():
			fmt.Printf("%s: ChannelPool Error - %s\r\n", time.Now(), err)
			channelPoolErrors++
		case err := <-ConnectionPool.Errors():
			fmt.Printf("%s: ConnectionPool Error - %s\r\n", time.Now(), err)
			connectionPoolErrors++
		case err := <-consumer.Errors():
			fmt.Printf("%s: Consumer Error - %s\r\n", time.Now(), err)
			consumerErrors++
		case message := <-consumer.Messages():
			messagesReceived++
			go func(msg *models.Message) {
				err := msg.Acknowledge()
				if err != nil {
					messagesFailedToAck++
				} else {
					messagesAcked++
				}
			}(message)
		default:
			time.Sleep(50 * time.Microsecond)
		}
	}

	fmt.Printf("ChannelPool Errors: %d\r\n", channelPoolErrors)
	fmt.Printf("ConnectionPool Errors: %d\r\n", connectionPoolErrors)

	fmt.Printf("Consumer Errors: %d\r\n", consumerErrors)
	fmt.Printf("Messages Acked: %d\r\n", messagesAcked)
	fmt.Printf("Messages Failed to Ack: %d\r\n", messagesFailedToAck)
	fmt.Printf("Messages Received: %d\r\n", messagesReceived)

	fmt.Printf("Messages Published: %d\r\n", messagesPublished)
	fmt.Printf("Messages Failed to Publish: %d\r\n", messagesFailedToPublish)

	publisher.StopAutoPublish()

	consumer.StopConsuming(false, true)

	ChannelPool.Shutdown()
}
