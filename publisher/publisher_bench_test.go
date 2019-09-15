package publisher_test

import (
	"context"
	"fmt"
	"runtime/trace"
	"testing"
	"time"

	"github.com/houseofcat/turbocookedrabbit/models"
	"github.com/houseofcat/turbocookedrabbit/publisher"
	"github.com/houseofcat/turbocookedrabbit/utils"
)

func BenchmarkAutoPublishRandomLetters(b *testing.B) {
	b.ReportAllocs()
	_, task := trace.NewTask(context.Background(), "BenchmarkAutoPublish")
	defer task.End()

	testQueuePrefix := "PubTQ"
	b.Logf("%s: Purging Queues...", time.Now())
	purgeAllPublisherTestQueues(testQueuePrefix, ChannelPool)

	messageCount := 100000
	letters := make([]*models.Letter, messageCount)
	totalBytes := uint64(0)

	pub, err := publisher.NewPublisher(Seasoning, ChannelPool, ConnectionPool)
	if err != nil {
		b.Log(err.Error())
	}

	done := make(chan bool, 1)
	pub.StartAutoPublish(false)

	// Build letters
	b.Logf("%s: Building Letters", time.Now())
	for i := 0; i < messageCount; i++ {
		letters[i] = utils.CreateMockRandomLetter(fmt.Sprintf("%s-%d", testQueuePrefix, i%10))
		totalBytes += uint64(len(letters[i].Body))
	}
	b.Logf("%s: Finished Building Letters", time.Now())
	b.Logf("%s: Total Size Created: %f MB", time.Now(), float64(totalBytes)/1000000.0)

	go func() {
	NotificationMonitor:
		for {
			select {
			case <-done:
				break NotificationMonitor
			case <-pub.Notifications():
				break
			default:
				break
			}
		}

	}()

	// Queue letters
	startTime := time.Now()
	b.Logf("%s: Queueing Letters", time.Now())
	for i := 0; i < messageCount; i++ {
		pub.QueueLetter(letters[i])
	}
	testDuration := time.Since(startTime)
	b.Logf("%s: Finished Queueing letters after %s", time.Now(), testDuration)
	b.Logf("%s: %f Msg/s", time.Now(), float64(messageCount)/testDuration.Seconds())

	pub.StopAutoPublish()
	done <- true
	time.Sleep(2 * time.Second)

	b.Logf("%s: Purging Queues...", time.Now())
	purgeAllPublisherTestQueues(testQueuePrefix, ChannelPool)
}
