package main_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/houseofcat/turbocookedrabbit/v2/pkg/tcr"
	"github.com/streadway/amqp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConnectionGetConnectionAndReturnSlowLoop is designed to be slow test connection recovery by severing all connections
// and then verify connections and channels properly (and evenly over connections) restore and continues publishing.
// All publish errors are not recovered but used to trigger channel recovery, total publish count should be lower
// than expected 10,000 during outages.
func TestConnectionGetChannelAndReturnSlowLoop(t *testing.T) {
	cfg, closer := InitTestService(t)
	defer closer()

	body := []byte("\x68\x65\x6c\x6c\x6f\x20\x77\x6f\x72\x6c\x64")

	wg := &sync.WaitGroup{}
	semaphore := make(chan bool, 100) // at most 100 requests a time
	for i := 0; i < 10000; i++ {      // total request to try

		wg.Add(1)
		semaphore <- true
		go func() {
			defer wg.Done()

			chanHost, err := cfg.ConnectionPool.GetChannelFromPool()
			require.NoError(t, err)

			time.Sleep(time.Millisecond * 100) // artificially create channel poool contention by long exposure

			err = chanHost.Channel.Publish("", "TcrTestQueue", false, false, amqp.Publishing{
				ContentType:  "plaintext/text",
				Body:         body,
				DeliveryMode: 2,
			})

			cfg.ConnectionPool.ReturnChannel(chanHost, err != nil)

			<-semaphore
		}()
	}

	wg.Wait() // wait for the final batch of requests to finish
	cfg.ConnectionPool.Shutdown()
}

// TestOutageAndQueueLetterAccuracy is similar to the above. It is designed to be slow test connection recovery by severing all connections
// and then verify connections and channels properly (and evenly over connections) restore.
func TestOutageAndQueueLetterAccuracy(t *testing.T) {
	cfg, closer := InitTestService(t)
	defer closer()

	letter := tcr.CreateMockRandomLetter("TcrTestQueue")

	// RabbitService is configured (out of the box) to use PublishWithConfirmations on AutoPublish.
	// All publish receipt events outside of the standard publish confirmation for streadway/amqp are also
	// listened to. During a failed publish receipt event, the default (and overridable behavior) is to automatically
	// requeue into the AutoPublisher for retry.
	//
	// This is fairly close to the most durable asynchronous form of Publishing you probably could create.
	// The only thing that would probably improve this, is directly call PublishWithConfirmations more
	// directly (which is an option).
	//
	// RabbitService has been hardened with a solid shutdown mechanism so please hook into this
	// in your application.
	//
	// This still leaves a few open ended vulnerabilities.
	// ApplicationSide:
	//    1.) Uncontrolled shutdown event or panic.
	//    2.) RabbitService publish receipt indicates a retry scenario, but we are mid-shutdown.
	// ServerSide:
	//    1.) A storage failure (without backup).
	//    2.) Split Brain event in HA mode.

	for i := 0; i < 10000; i++ {
		err := cfg.RabbitService.QueueLetter(letter)
		assert.NoError(t, err)
	}

	<-time.After(time.Second * 5) // refresh rate of management API is 5 seconds, this just allows you to see the queue

	cfg.RabbitService.Shutdown(true)
}

// TestPublishWithHeaderAndVerify verifies headers are publishing.
func TestPublishWithHeaderAndVerify(t *testing.T) {
	cfg, closer := InitTestService(t)
	defer closer()

	letter := tcr.CreateMockRandomLetter("TcrTestQueue")

	_ = cfg.RabbitService.PublishLetter(letter)

	consumer, err := cfg.RabbitService.GetConsumer("TurboCookedRabbitConsumer-Ackable")
	assert.NoError(t, err)

	delivery, err := consumer.Get("TcrTestQueue")
	assert.NoError(t, err)

	if delivery != nil {
		testHeader, ok := delivery.Headers["x-tcr-testheader"]
		assert.True(t, ok)

		fmt.Printf("Header Received: %s\r\n", testHeader.(string))
	}
}

// TestPublishWithHeaderAndConsumerReceivedHeader verifies headers are being consumed into ReceivedData.
func TestPublishWithHeaderAndConsumerReceivedHeader(t *testing.T) {
	cfg, closer := InitTestService(t)
	defer closer()

	letter := tcr.CreateMockRandomLetter("TcrTestQueue")

	_ = cfg.RabbitService.PublishLetter(letter)

	consumer, err := cfg.RabbitService.GetConsumer("TurboCookedRabbitConsumer-Ackable")
	consumer.StartConsuming()
	assert.NoError(t, err)

WaitLoop:
	for {
		select {
		case data := <-consumer.ReceivedMessages():

			testHeader, ok := data.Delivery.Headers["x-tcr-testheader"]
			assert.True(t, ok)

			fmt.Printf("Header Received: %s\r\n", testHeader.(string))
			break WaitLoop

		default:
			time.Sleep(1 * time.Millisecond)
		}
	}
}
