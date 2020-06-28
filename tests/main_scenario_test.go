package main_test

import (
	"sync"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/houseofcat/turbocookedrabbit/pkg/tcr"
	"github.com/streadway/amqp"
)

// TestConnectionGetConnectionAndReturnSlowLoop is designed to be slow test connection recovery by severing all connections
// and then verify connections and channels properly (and evenly over connections) restore and continues publishing.
// All publish errors are not recovered but used to trigger channel recovery, total publish count should be lower
// than expected 10,000 during outages.
func TestConnectionGetChannelAndReturnSlowLoop(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	body := []byte("\x68\x65\x6c\x6c\x6f\x20\x77\x6f\x72\x6c\x64")

	wg := &sync.WaitGroup{}
	semaphore := make(chan bool, 100) // at most 100 requests a time
	for i := 0; i < 10000; i++ {      // total request to try

		wg.Add(1)
		semaphore <- true
		go func() {
			defer wg.Done()

			chanHost := ConnectionPool.GetChannelFromPool()

			time.Sleep(time.Millisecond * 100) // artificially create channel poool contention by long exposure

			err := chanHost.Channel.Publish("", "TcrTestQueue", false, false, amqp.Publishing{
				ContentType:  "plaintext/text",
				Body:         body,
				DeliveryMode: 2,
			})

			ConnectionPool.ReturnChannel(chanHost, err != nil)

			<-semaphore
		}()
	}

	wg.Wait() // wait for the final batch of requests to finish
	ConnectionPool.Shutdown()
}

// TestOutageAndPublishConfirmationAccuracy is similar to the above. It is designed to be slow test connection recovery by severing all connections
// and then verify connections and channels properly (and evenly over connections) restore.
func TestOutageAndPublishConfirmationAccuracy(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

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
	// ServerSide:
	//    1.) A storage failure (without backup).
	//    2.) Split Brain event in HA mode.
	RabbitService.Publisher.QueueLetter(letter)

	TestCleanup(t)
}
