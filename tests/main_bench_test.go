package main_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/houseofcat/turbocookedrabbit/pkg/tcr"
)

func BenchmarkPublishAndConsumeMany(b *testing.B) {

	b.ReportAllocs()

	fmt.Printf("Benchmark Starts: %s\r\n", time.Now())
	messageCount := 10000
	connectionPool, _ := tcr.NewConnectionPool(Seasoning.PoolConfig)
	publisher, _ := tcr.NewPublisherWithConfig(Seasoning, connectionPool)

	consumerConfig, ok := Seasoning.ConsumerConfigs["TurboCookedRabbitConsumer"]
	assert.True(b, ok)

	consumer, _ := tcr.NewConsumerFromConfig(consumerConfig, connectionPool)

	publisher.StartAutoPublishing()

	counter := uint64(0)

	go func() {
		for i := 0; i < messageCount; i++ {
			letter := tcr.CreateMockRandomLetter("ConsumerTestQueue")
			letter.LetterID = counter
			counter++

			publisher.QueueLetter(letter)
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
		case publish := <-publisher.PublishReceipts():
			if publish.Success {
				//fmt.Printf("%s: Published Success - LetterID: %d\r\n", time.Now(), notice.LetterID)
				messagesPublished++
			} else {
				//fmt.Printf("%s: Published Failed - LetterID: %d\r\n", time.Now(), notice.LetterID)
				messagesFailedToPublish++
			}
		case err := <-consumer.Errors():
			fmt.Printf("%s: Consumer - Error: %s\r\n", time.Now(), err)
			consumerErrors++
		case <-consumer.ReceivedMessages():
			//fmt.Printf("%s: MessageReceived\r\n", time.Now())
			messagesReceived++
		default:
			time.Sleep(10 * time.Microsecond)
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

	publisher.StopAutoPublish()

	if err := consumer.StopConsuming(true, true); err != nil {
		b.Error(err)
	}
	connectionPool.Shutdown()
	cancel()
}

func BenchmarkPublishConsumeAckForDuration(b *testing.B) {
	b.ReportAllocs()

	timeDuration := time.Duration(5 * time.Minute)
	timeOut := time.After(timeDuration)
	fmt.Printf("Benchmark Starts: %s\r\n", time.Now())
	fmt.Printf("Est. Benchmark End: %s\r\n", time.Now().Add(timeDuration))

	publisher, _ := tcr.NewPublisherWithConfig(Seasoning, ConnectionPool)
	consumerConfig, ok := Seasoning.ConsumerConfigs["TurboCookedRabbitConsumer-Ackable"]
	assert.True(b, ok)

	consumer, _ := tcr.NewConsumerFromConfig(consumerConfig, ConnectionPool)

	publisher.StartAutoPublishing()

	go publishLoop(timeOut, publisher)

	consumer.StartConsuming()

	consumeLoop(b, timeOut, publisher, consumer)

	publisher.StopAutoPublish()

	if err := consumer.StopConsuming(false, true); err != nil {
		b.Error(err)
	}

	BenchCleanup(b)
}

func publishLoop(timeOut <-chan time.Time, publisher *tcr.Publisher) {
	letterTemplate := tcr.CreateMockRandomLetter("ConsumerTestQueue")

	go func() {
	PublishLoop:
		for {
			select {
			case <-timeOut:
				break PublishLoop
			default:
				newLetter := tcr.Letter(*letterTemplate)
				publisher.QueueLetter(&newLetter)
				letterTemplate.LetterID++
			}
		}
	}()
}

func consumeLoop(
	b *testing.B,
	timeOut <-chan time.Time,
	publisher *tcr.Publisher,
	consumer *tcr.Consumer) {

	messagesReceived := 0
	messagesPublished := 0
	messagesFailedToPublish := 0
	messagesAcked := 0
	messagesFailedToAck := 0
	consumerErrors := 0
	connectionPoolErrors := 0

ConsumeLoop:
	for {
		select {
		case <-timeOut:
			break ConsumeLoop
		case notice := <-publisher.PublishReceipts():
			if notice.Success {
				messagesPublished++
				notice = nil
			} else {
				messagesFailedToPublish++
				notice = nil
			}
		case err := <-consumer.Errors():
			b.Logf("%s: Consumer Error - %s\r\n", time.Now(), err)
			consumerErrors++
		case message := <-consumer.ReceivedMessages():
			messagesReceived++
			go func(msg *tcr.ReceivedMessage) {
				err := msg.Acknowledge()
				if err != nil {
					messagesFailedToAck++
				} else {
					messagesAcked++
				}
			}(message)
		default:
			time.Sleep(1 * time.Microsecond)
			break
		}
	}

	b.Logf("ConnectionPool Errors: %d\r\n", connectionPoolErrors)

	b.Logf("Consumer Errors: %d\r\n", consumerErrors)
	b.Logf("Messages Acked: %d\r\n", messagesAcked)
	b.Logf("Messages Failed to Ack: %d\r\n", messagesFailedToAck)
	b.Logf("Messages Received: %d\r\n", messagesReceived)

	b.Logf("Messages Published: %d\r\n", messagesPublished)
	b.Logf("Messages Failed to Publish: %d\r\n", messagesFailedToPublish)
}
