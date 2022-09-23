package main_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/houseofcat/turbocookedrabbit/v2/pkg/tcr"
	cmap "github.com/orcaman/concurrent-map"
	"github.com/stretchr/testify/assert"
)

func TestWithStress(t *testing.T) {
	cfg, closer := InitTestService(t)
	defer closer()

	timeDuration := time.Duration(2 * time.Hour)
	pubTimeOut := time.After(timeDuration)
	conTimeOut := time.After(timeDuration + (1 * time.Minute))
	fmt.Printf("Benchmark Starts: %s\r\n", time.Now())
	fmt.Printf("Est. Benchmark End: %s\r\n", time.Now().Add(timeDuration))

	publisher := tcr.NewPublisherFromConfig(cfg.Seasoning, cfg.ConnectionPool)
	publisher.StartAutoPublishing()

	consumerConfig, ok := cfg.Seasoning.ConsumerConfigs["TurboCookedRabbitConsumer-Ackable"]
	assert.True(t, ok)

	consumer := tcr.NewConsumerFromConfig(consumerConfig, cfg.ConnectionPool)
	conMap := cmap.New()

	publishDone := make(chan bool, 1)
	go publishQueueLetter(t, conMap, publishDone, pubTimeOut, publisher)

	consumerDone := make(chan bool, 1)
	consumer.StartConsuming()
	go consumeDurationAccuracyLoop(t, conMap, consumerDone, conTimeOut, consumer)

	<-publishDone
	publisher.Shutdown(false)

	<-consumerDone
	if err := consumer.StopConsuming(false, true); err != nil {
		t.Error(err)
	}

	// Breakpoint here and check the conMap for 100% accuracy.
	verifyAccuracyT(t, conMap)
}

func publishQueueLetter(
	t *testing.T,
	conMap cmap.ConcurrentMap,
	done chan bool,
	timeOut <-chan time.Time,
	publisher *tcr.Publisher) {

	queueErrors := 0
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
					fmt.Printf("%s: Publisher failed for LetterID: %s\r\n", time.Now(), notice.LetterID.String())
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
			conMap.Set(newLetter.LetterID.String(), false)

			if !publisher.QueueLetter(newLetter) {
				queueErrors++
			}
		}
	}

	fmt.Printf("Failed to Queue For Publishing: %d\r\n", queueErrors)
	fmt.Printf("Publisher Errors: %d\r\n", publisherErrors)
	fmt.Printf("Messages Published: %d\r\n", messagesPublished)
	fmt.Printf("Messages Failed to Publish: %d\r\n", messagesFailedToPublish)

	done <- true
}

func TestDurationAccuracy(t *testing.T) {
	cfg, closer := InitTestService(t)
	defer closer()

	timeDuration := time.Duration(5 * time.Minute)
	pubTimeOut := time.After(timeDuration)
	conTimeOut := time.After(timeDuration + (1 * time.Minute))
	fmt.Printf("Benchmark Starts: %s\r\n", time.Now())
	fmt.Printf("Est. Benchmark End: %s\r\n", time.Now().Add(timeDuration))

	publisher := tcr.NewPublisherFromConfig(cfg.Seasoning, cfg.ConnectionPool)
	consumerConfig, ok := cfg.Seasoning.ConsumerConfigs["TurboCookedRabbitConsumer-Ackable"]
	assert.True(t, ok)

	consumer := tcr.NewConsumerFromConfig(consumerConfig, cfg.ConnectionPool)
	conMap := cmap.New()

	publishDone := make(chan bool, 1)
	go publishDurationAccuracyLoop(t, conMap, publishDone, pubTimeOut, publisher)

	consumerDone := make(chan bool, 1)
	consumer.StartConsuming()
	go consumeDurationAccuracyLoop(t, conMap, consumerDone, conTimeOut, consumer)

	<-publishDone
	publisher.Shutdown(false)

	<-consumerDone
	if err := consumer.StopConsuming(false, true); err != nil {
		t.Error(err)
	}

	// Breakpoint here and check the conMap for 100% accuracy.
	verifyAccuracyT(t, conMap)
}

func publishDurationAccuracyLoop(
	t *testing.T,
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
			conMap.Set(newLetter.LetterID.String(), false)
			publisher.PublishWithConfirmation(newLetter, 50*time.Millisecond)
		}
	}

	fmt.Printf("Publisher Errors: %d\r\n", publisherErrors)
	fmt.Printf("Messages Published: %d\r\n", messagesPublished)
	fmt.Printf("Messages Failed to Publish: %d\r\n", messagesFailedToPublish)

	done <- true
}

func consumeDurationAccuracyLoop(
	t *testing.T,
	conMap cmap.ConcurrentMap,
	done chan bool,
	timeOut <-chan time.Time,
	consumer *tcr.Consumer) {

	messagesReceived := 0
	messagesAcked := 0
	messagesFailedToAck := 0
	consumerErrors := 0
	surpriseMessages := 0
	duplicateMessages := 0

ConsumeLoop:
	for {
		select {
		case <-timeOut:
			break ConsumeLoop

		case err := <-consumer.Errors():

			fmt.Printf("%s: Consumer Error - %s\r\n", time.Now(), err)
			consumerErrors++

		case message := <-consumer.ReceivedMessages():

			messagesReceived++
			body, err := tcr.ReadWrappedBodyFromJSONBytes(message.Delivery.Body)
			if err != nil {
				fmt.Printf("message was not deserializeable\r\n")
			} else {
				// Accuracy check
				if tmp, ok := conMap.Get(body.LetterID.String()); ok {
					state := tmp.(bool)
					if state {
						duplicateMessages++
						fmt.Printf("duplicate letter (%s) received!\r\n", body.LetterID.String())
					} else {
						conMap.Set(body.LetterID.String(), true)
					}
				} else {
					surpriseMessages++
					fmt.Printf("letter (%s) received that wasn't published!\r\n", body.LetterID.String())
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

	fmt.Printf("Consumer Errors: %d\r\n", consumerErrors)
	fmt.Printf("Messages Acked: %d\r\n", messagesAcked)
	fmt.Printf("Messages Failed to Ack: %d\r\n", messagesFailedToAck)
	fmt.Printf("Messages Received: %d\r\n", messagesReceived)
	fmt.Printf("Messages Unexpected: %d\r\n", surpriseMessages)
	fmt.Printf("Messages Duplicated: %d\r\n", duplicateMessages)

	done <- true
}

func verifyAccuracyT(t *testing.T, conMap cmap.ConcurrentMap) {
	// Breakpoint here and check the conMap for 100% accuracy.
	for item := range conMap.IterBuffered() {
		state := item.Val.(bool)
		if !state {
			fmt.Printf("LetterID: %q was published but never received.\r\n", item.Key)
		}
	}
}

func TestPublishConsumeCountAccuracy(t *testing.T) {
	cfg, closer := InitTestService(t)
	defer closer()

	fmt.Printf("Benchmark Starts: %s\r\n", time.Now())
	publisher := tcr.NewPublisherFromConfig(cfg.Seasoning, cfg.ConnectionPool)
	consumerConfig, ok := cfg.Seasoning.ConsumerConfigs["TurboCookedRabbitConsumer-Ackable"]
	assert.True(t, ok)

	consumer := tcr.NewConsumerFromConfig(consumerConfig, cfg.ConnectionPool)

	conMap := cmap.New()
	count := 10000

	pubCount := publishAccuracyLoop(t, conMap, publisher, count)

	consumer.StartConsuming()
	consumeAccuracyLoop(t, conMap, consumer, pubCount)

	// Breakpoint here to manually check the conMap for 100% accuracy.
	verifyAccuracyT(t, conMap)

	// How this test works:
	// 1.) Try to publish (X) messages with server based ack/confirmation.
	// 2.) Listen for all publish success and failures (even if it was a false failure positive).
	//     We also timeout if it is taking too long waiting on server confirmation without a
	//     retry in place. QueueLetter mechanism includes the retry.
	// 3.) Pass the success count (Y) to the consumer.
	// 4.) Consume (Y) messages.
	// 5.) Compare all the letters published vs. consumed (concurrent map).
	// 6.) Report all those published and not consumed, or consumed and not published.
	// 7.) If we are short messages (which can happen) verify the discrepancy is still
	//     in the queue. For example, if you published 10000, publisher only reported 9999,
	//     consumer only got 9999, the map should show you are missing 1 letter and it
	//     should still be in your queue.

	// PublishWithConfirmation is an at least once delivery mechanism, meaning we can see
	// multiple copies of each message.

	// Consumption is event driven so it will consume forever so we artificially cut it
	// off based on how much we think it should have consumed. This can lead to a difference
	// in pub/sub counts and leave messages in the queue. So we manually verify the queue
	// too.
	cfg.RabbitService.Shutdown()
}

func publishAccuracyLoop(
	t *testing.T,
	conMap cmap.ConcurrentMap,
	publisher *tcr.Publisher,
	count int) int {

	messagesPublished := 0
	messagesFailedToPublish := 0
	publisherErrors := 0
	pubDone := make(chan bool, 1)
	monDone := make(chan bool, 1)

	go func() {
	MonitorLoop:
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
				case <-pubDone:
					break MonitorLoop
				default:
					break ReadReceipts
				}
			}

			if count == messagesPublished+messagesFailedToPublish {
				break
			}
		}
		monDone <- true
	}()

	go func() {
		for i := 0; i < count; i++ {
			newLetter := tcr.CreateMockRandomWrappedBodyLetter("TcrTestQueue")
			conMap.Set(newLetter.LetterID.String(), false)
			publisher.PublishWithConfirmation(newLetter, 50*time.Millisecond)
		}

		pubDone <- true
	}()

	<-monDone
	fmt.Printf("Publisher Errors: %d\r\n", publisherErrors)
	fmt.Printf("Messages Published: %d\r\n", messagesPublished)
	fmt.Printf("Messages Failed to Publish: %d\r\n", messagesFailedToPublish)

	return messagesPublished
}

func consumeAccuracyLoop(
	t *testing.T,
	conMap cmap.ConcurrentMap,
	consumer *tcr.Consumer,
	count int) {

	messagesReceived := 0
	messagesAcked := 0
	messagesFailedToAck := 0
	consumerErrors := 0

	conDone := make(chan bool, 1)
	monDone := make(chan bool, 1)

	go func() {
	MonitorLoop:
		for {
			select {
			case <-conDone:
				monDone <- true
				break MonitorLoop
			case err := <-consumer.Errors():

				fmt.Printf("%s: Consumer Error - %s\r\n", time.Now(), err)
				consumerErrors++

			default:
				break
			}
		}
	}()

AcknowledgeLoop:
	for {
		select {
		case message := <-consumer.ReceivedMessages():

			messagesReceived++
			body, err := tcr.ReadWrappedBodyFromJSONBytes(message.Delivery.Body)
			if err != nil {
				fmt.Print("message was not deserializeable")
			} else {
				// Accuracy check
				if tmp, ok := conMap.Get(body.LetterID.String()); ok {
					state := tmp.(bool)
					if state {
						fmt.Printf("duplicate letter (%s) received!\r\n", body.LetterID.String())
					} else {
						conMap.Set(body.LetterID.String(), true)
					}
				} else {
					fmt.Printf("letter (%s) received that wasn't published!\r\n", body.LetterID.String())
				}
			}

			err = message.Acknowledge()
			if err != nil {
				messagesFailedToAck++
			} else {
				messagesAcked++
			}

			if count == messagesAcked+messagesFailedToAck {
				conDone <- true
				break AcknowledgeLoop
			}
		default:
			time.Sleep(1 * time.Microsecond)
		}
	}

	<-monDone

	fmt.Printf("Consumer Errors: %d\r\n", consumerErrors)
	fmt.Printf("Messages Acked: %d\r\n", messagesAcked)
	fmt.Printf("Messages Failed to Ack: %d\r\n", messagesFailedToAck)
	fmt.Printf("Messages Received: %d\r\n", messagesReceived)
}

var conmap cmap.ConcurrentMap

func TestWithRabbitServiceStress(t *testing.T) {
	cfg, closer := InitTestService(t)
	defer closer()

	timeDuration := time.Duration(2 * time.Hour)
	pubTimeOut := time.After(timeDuration)
	conTimeOut := time.After(timeDuration + (1 * time.Minute))
	fmt.Printf("Benchmark Starts: %s\r\n", time.Now())
	fmt.Printf("Est. Benchmark End: %s\r\n", time.Now().Add(timeDuration))

	conmap = cmap.New()

	publishDone := make(chan bool, 1)
	go publishRabbitServiceQueue(t, cfg, publishDone, pubTimeOut)

	consumerDone := make(chan bool, 1)
	go consumerRabbitService(t, cfg, consumerDone, conTimeOut)

	<-publishDone
	<-consumerDone

	// Breakpoint here and check the conMap for 100% accuracy.
	verifyAccuracyT(t, conmap)
}

func publishRabbitServiceQueue(
	t *testing.T,
	cfg *Config,
	done chan bool,
	timeOut <-chan time.Time) {

PublishLoop:
	for {

		select {
		case <-timeOut:
			break PublishLoop
		default:
			newLetter := tcr.CreateMockRandomWrappedBodyLetter("TcrTestQueue")
			conmap.Set(newLetter.LetterID.String(), false)
			_ = cfg.RabbitService.QueueLetter(newLetter)
		}
	}

	done <- true
}

func consumerRabbitService(
	t *testing.T,
	cfg *Config,
	done chan bool,
	timeOut <-chan time.Time) {

	consumer, _ := cfg.RabbitService.GetConsumer("TurboCookedRabbitConsumer-Ackable")
	consumer.StartConsumingWithAction(consumerAction)

	consumerErrors := 0

ConsumeLoop:
	for {
		select {
		case <-timeOut:
			break ConsumeLoop

		case err := <-consumer.Errors():

			fmt.Printf("%s: Consumer Error - %s\r\n", time.Now(), err)
			consumerErrors++

		default:
			time.Sleep(time.Microsecond)
			break
		}
	}

	fmt.Printf("Consumer Errors: %d\r\n", consumerErrors)

	if err := consumer.StopConsuming(false, true); err != nil {
		t.Error(err)
	}

	done <- true
}

func consumerAction(msg *tcr.ReceivedMessage) {

	body, err := tcr.ReadWrappedBodyFromJSONBytes(msg.Delivery.Body)

	if err != nil {
		fmt.Printf("message was not deserializeable\r\n")
	} else {
		// Accuracy check
		if tmp, ok := conmap.Get(body.LetterID.String()); ok {
			state := tmp.(bool)
			if state {
				fmt.Printf("duplicate letter (%s) received!\r\n", body.LetterID.String())
			} else {
				conmap.Set(body.LetterID.String(), true)
			}
		} else {
			fmt.Printf("letter (%s) received that wasn't published!\r\n", body.LetterID.String())
		}
	}

	err = msg.Acknowledge()
	if err != nil {
		fmt.Printf("Consumer Ack Error: %s\r\n", err)
	}
}
