package main_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	cmap "github.com/orcaman/concurrent-map"
	"github.com/stretchr/testify/assert"

	"github.com/houseofcat/turbocookedrabbit/v2/pkg/tcr"
)

func verifyAccuracyB(b *testing.B, conMap cmap.ConcurrentMap) {
	// Breakpoint here and check the conMap for 100% accuracy.
	for item := range conMap.IterBuffered() {
		state := item.Val.(bool)
		if !state {
			fmt.Printf("LetterId: %q was not received.\r\n", item.Key)
		}
	}
}

func BenchmarkPublishAndConsumeMany(b *testing.B) {

	b.ReportAllocs()

	fmt.Printf("Benchmark Starts: %s\r\n", time.Now())
	messageCount := 10000
	connectionPool, _ := tcr.NewConnectionPool(Seasoning.PoolConfig)
	publisher := tcr.NewPublisherFromConfig(Seasoning, connectionPool)

	consumerConfig, ok := Seasoning.ConsumerConfigs["TurboCookedRabbitConsumer"]
	assert.True(b, ok)

	consumer := tcr.NewConsumerFromConfig(consumerConfig, connectionPool)

	publisher.StartAutoPublishing()

	go func() {
		for i := 0; i < messageCount; i++ {
			letter := tcr.CreateMockRandomLetter("TcrTestQueue")
			letter.LetterID = uuid.New()

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

	publisher.Shutdown(false)

	if err := consumer.StopConsuming(true, true); err != nil {
		b.Error(err)
	}
	connectionPool.Shutdown()
	cancel()
}

func BenchmarkPublishForDuration(b *testing.B) {

	b.ReportAllocs()

	timeDuration := time.Duration(time.Minute)
	timeOut := time.After(timeDuration)
	fmt.Printf("Benchmark Starts: %s\r\n", time.Now())
	fmt.Printf("Est. Benchmark End: %s\r\n", time.Now().Add(timeDuration))

	publisher := tcr.NewPublisherFromConfig(Seasoning, ConnectionPool)

	publishDone := make(chan bool, 1)
	conMap := cmap.New()
	publishLoop(b, conMap, publishDone, timeOut, publisher)
	<-publishDone

	publisher.Shutdown(false)

	BenchCleanup(b)
}

func BenchmarkPublishConsumeAckForDuration(b *testing.B) {

	b.ReportAllocs()

	timeDuration := time.Duration(500 * time.Millisecond)
	pubTimeOut := time.After(timeDuration)
	conTimeOut := time.After(timeDuration + (2 * time.Second))
	fmt.Printf("Benchmark Starts: %s\r\n", time.Now())
	fmt.Printf("Est. Benchmark End: %s\r\n", time.Now().Add(timeDuration))

	publisher := tcr.NewPublisherFromConfig(Seasoning, ConnectionPool)
	consumerConfig, ok := Seasoning.ConsumerConfigs["TurboCookedRabbitConsumer-Ackable"]
	assert.True(b, ok)

	consumer := tcr.NewConsumerFromConfig(consumerConfig, ConnectionPool)
	conMap := cmap.New()
	//publisher.StartAutoPublishing()

	publishDone := make(chan bool, 1)
	go publishLoop(b, conMap, publishDone, pubTimeOut, publisher)

	consumerDone := make(chan bool, 1)
	consumer.StartConsuming()
	go consumeLoop(b, conMap, consumerDone, conTimeOut, publisher, consumer)

	<-publishDone
	publisher.Shutdown(false)

	<-consumerDone
	if err := consumer.StopConsuming(false, true); err != nil {
		b.Error(err)
	}

	verifyAccuracyB(b, conMap)
	BenchCleanup(b)
}

func publishLoop(
	b *testing.B,
	conMap cmap.ConcurrentMap,
	done chan bool,
	timeOut <-chan time.Time,
	publisher *tcr.Publisher) {

	messagesPublished := 0
	messagesFailedToPublish := 0
	publisherErrors := 0

PublishLoop:
	for {
		// Read all the queued receipts before next publish.
	ReadReceipts:
		for {
			select {
			case notice := <-publisher.PublishReceipts():
				if notice.Success {
					messagesPublished++
					notice = nil
				} else {
					messagesFailedToPublish++
					notice = nil
				}
			default:
				break ReadReceipts
			}
		}

		select {
		case <-timeOut:
			break PublishLoop
		default:
			newLetter := tcr.CreateMockRandomWrappedBodyLetter("TcrTestQueue")
			conMap.Set(fmt.Sprintf("%d", newLetter.LetterID), false)
			publisher.PublishWithConfirmation(newLetter, 50*time.Millisecond)
		}
	}

	b.Logf("Publisher Errors: %d\r\n", publisherErrors)
	b.Logf("Messages Published: %d\r\n", messagesPublished)
	b.Logf("Messages Failed to Publish: %d\r\n", messagesFailedToPublish)

	done <- true
}

func consumeLoop(
	b *testing.B,
	conMap cmap.ConcurrentMap,
	done chan bool,
	timeOut <-chan time.Time,
	publisher *tcr.Publisher,
	consumer *tcr.Consumer) {

	messagesReceived := 0
	messagesAcked := 0
	messagesFailedToAck := 0
	consumerErrors := 0

ConsumeLoop:
	for {
		select {
		case <-timeOut:
			break ConsumeLoop

		case err := <-consumer.Errors():

			b.Logf("%s: Consumer Error - %s\r\n", time.Now(), err)
			consumerErrors++

		case message := <-consumer.ReceivedMessages():

			messagesReceived++
			body, err := tcr.ReadWrappedBodyFromJSONBytes(message.Delivery.Body)
			if err != nil {
				b.Logf("message was not deserializeable")
			} else {
				// Accuracy check
				if tmp, ok := conMap.Get(fmt.Sprintf("%d", body.LetterID)); ok {
					state := tmp.(bool)
					if state {
						b.Logf("duplicate letter (%d) received!", body.LetterID)
					} else {
						conMap.Set(fmt.Sprintf("%d", body.LetterID), true)
					}
				} else {
					b.Logf("letter (%d) received that wasn't published!", body.LetterID)
				}
			}

			err = message.Acknowledge()
			if err != nil {
				messagesFailedToAck++
			} else {
				messagesAcked++
			}
		default:
			time.Sleep(time.Microsecond)
			break
		}
	}

	b.Logf("Consumer Errors: %d\r\n", consumerErrors)
	b.Logf("Messages Acked: %d\r\n", messagesAcked)
	b.Logf("Messages Failed to Ack: %d\r\n", messagesFailedToAck)
	b.Logf("Messages Received: %d\r\n", messagesReceived)

	done <- true
}
