package publisher_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/assert"

	"github.com/houseofcat/turbocookedrabbit/models"
	"github.com/houseofcat/turbocookedrabbit/pools"
	"github.com/houseofcat/turbocookedrabbit/publisher"
	"github.com/houseofcat/turbocookedrabbit/topology"
	"github.com/houseofcat/turbocookedrabbit/utils"
)

var Seasoning *models.RabbitSeasoning
var ConnectionPool *pools.ConnectionPool
var ChannelPool *pools.ChannelPool

func TestMain(m *testing.M) { // Load Configuration On Startup
	var err error
	Seasoning, err = utils.ConvertJSONFileToConfig("testpublisherseasoning.json")
	if err != nil {
		fmt.Print(err.Error())
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

func TestCreatePublisher(t *testing.T) {
	channelPool, err := pools.NewChannelPool(Seasoning.PoolConfig, nil, true)
	assert.NoError(t, err)

	channelPool.FlushErrors()

	publisher, err := publisher.NewPublisher(Seasoning, channelPool, nil)
	assert.NoError(t, err)
	assert.NotNil(t, publisher)
}

func TestCreatePublisherAndPublish(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	channelPool, err := pools.NewChannelPool(Seasoning.PoolConfig, nil, true)
	assert.NoError(t, err)

	channelPool.FlushErrors()

	publisher, err := publisher.NewPublisher(Seasoning, channelPool, nil)
	assert.NoError(t, err)
	assert.NotNil(t, publisher)

	letterID := uint64(1)
	body := "\xFF\xFF\x89\xFF\xFF"
	envelope := &models.Envelope{
		Exchange:    "",
		RoutingKey:  "TestQueue",
		ContentType: "plain/text",
		Mandatory:   false,
		Immediate:   false,
	}

	letter := &models.Letter{
		LetterID:   letterID,
		RetryCount: uint32(3),
		Body:       []byte(body),
		Envelope:   envelope,
	}

	publisher.Publish(letter)

	// Assert on all Notifications
AssertLoop:
	for {
		select {
		case chanErr := <-channelPool.Errors():
			assert.NoError(t, chanErr) // This test fails on channel errors.
			break AssertLoop
		case notification := <-publisher.Notifications():
			assert.True(t, notification.Success)
			assert.Equal(t, letterID, notification.LetterID)
			assert.NoError(t, notification.Error)
			break AssertLoop
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}

	channelPool.Shutdown()
}

func TestAutoPublishSingleMessage(t *testing.T) {

	channelPool, err := pools.NewChannelPool(Seasoning.PoolConfig, nil, true)
	assert.NoError(t, err)

	channelPool.FlushErrors()

	publisher, err := publisher.NewPublisher(Seasoning, channelPool, nil)
	assert.NoError(t, err)

	letter := utils.CreateMockRandomLetter("ConsumerTestQueue")

	publisher.StartAutoPublish(false)

	publisher.QueueLetter(letter)

	publisher.StopAutoPublish()

	// Assert on all Notifications
AssertLoop:
	for {
		select {
		case chanErr := <-channelPool.Errors():
			assert.NoError(t, chanErr) // This test fails on channel errors.
			break AssertLoop
		case notification := <-publisher.Notifications():
			assert.True(t, notification.Success)
			assert.Equal(t, letter.LetterID, notification.LetterID)
			assert.NoError(t, notification.Error)
			break AssertLoop
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}

	channelPool.Shutdown()
}

func TestAutoPublishManyMessages(t *testing.T) {

	defer leaktest.Check(t)() // Fail on leaked goroutines.
	messageCount := 100000

	channelPool, err := pools.NewChannelPool(Seasoning.PoolConfig, nil, true)
	assert.NoError(t, err)

	channelPool.FlushErrors()

	// Purge all queues first.
	purgeAllPublisherTestQueues("PubTQ", channelPool)

	publisher, err := publisher.NewPublisher(Seasoning, channelPool, nil)
	assert.NoError(t, err)

	// Pre-create test messages
	timeStart := time.Now()
	letters := make([]*models.Letter, messageCount)

	for i := 0; i < messageCount; i++ {
		letters[i] = utils.CreateMockLetter(uint64(i), "", fmt.Sprintf("PubTQ-%d", i%10), nil)
	}

	elapsed := time.Since(timeStart)
	fmt.Printf("Time Elapsed Creating Letters: %s\r\n", elapsed)

	timeStart = time.Now()
	publisher.StartAutoPublish(false)

	go func() {

		for _, letter := range letters {
			publisher.QueueLetter(letter)
		}
	}()

	successCount := 0
	failureCount := 0
	timer := time.NewTimer(1 * time.Minute)

ListeningForNotificationsLoop:
	for {
		select {
		case <-timer.C:
			break ListeningForNotificationsLoop
		case chanErr := <-channelPool.Errors():
			if chanErr != nil {
				failureCount++
			}
			break
		case notification := <-publisher.Notifications():
			if notification.Success {
				successCount++
			} else {
				failureCount++
			}

			if successCount+failureCount == messageCount {
				break ListeningForNotificationsLoop
			}

			break
		default:
			time.Sleep(1 * time.Millisecond)
			break
		}
	}

	elapsed = time.Since(timeStart)

	assert.Equal(t, messageCount, successCount+failureCount)
	fmt.Printf("All Messages Accounted For: %d\r\n", successCount)
	fmt.Printf("Success Count: %d\r\n", successCount)
	fmt.Printf("Failure Count: %d\r\n", failureCount)
	fmt.Printf("Time Elapsed: %s\r\n", elapsed)
	fmt.Printf("Rate: %f msg/s\r\n", float64(messageCount)/elapsed.Seconds())

	// Purge all queues.
	purgeAllPublisherTestQueues("PubTQ", channelPool)

	// Shut down everything.
	publisher.StopAutoPublish()
	channelPool.Shutdown()
}

func TestTwoAutoPublishSameChannelPool(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	messageCount := 50000
	publisherMultiple := 2

	channelPool, err := pools.NewChannelPool(Seasoning.PoolConfig, nil, true)
	assert.NoError(t, err)

	// Purge all queues first.
	purgeAllPublisherTestQueues("PubTQ", channelPool)

	publisher1, p1Err := publisher.NewPublisher(Seasoning, channelPool, nil)
	assert.NoError(t, p1Err)

	publisher2, p2Err := publisher.NewPublisher(Seasoning, channelPool, nil)
	assert.NoError(t, p2Err)

	// Pre-create test messages
	timeStart := time.Now()
	letters := make([]*models.Letter, messageCount)

	for i := 0; i < messageCount; i++ {
		letters[i] = utils.CreateMockLetter(uint64(i), "", fmt.Sprintf("PubTQ-%d", i%10), nil)
	}

	elapsed := time.Since(timeStart)
	fmt.Printf("Time Elapsed Creating Letters: %s\r\n", elapsed)

	timeStart = time.Now()
	publisher1.StartAutoPublish(false)
	publisher2.StartAutoPublish(false)

	go func() {

		for _, letter := range letters {
			publisher1.QueueLetter(letter)
			publisher2.QueueLetter(letter)
		}
	}()

	successCount := 0
	failureCount := 0
	timer := time.NewTimer(1 * time.Minute)

	var notification *models.Notification
ListeningForNotificationsLoop:
	for {
		select {
		case <-timer.C:
			break ListeningForNotificationsLoop
		case chanErr := <-channelPool.Errors():
			if chanErr != nil {
				failureCount++
			}
			break
		case notification = <-publisher1.Notifications():
		case notification = <-publisher2.Notifications():
		default:
			time.Sleep(1 * time.Millisecond)
			break
		}

		if notification != nil {
			if notification.Success {
				successCount++
			} else {
				failureCount++
			}

			notification = nil
		}

		if successCount+failureCount == publisherMultiple*messageCount {
			break ListeningForNotificationsLoop
		}
	}

	elapsed = time.Since(timeStart)

	assert.Equal(t, publisherMultiple*messageCount, successCount+failureCount)
	fmt.Printf("All Messages Accounted For: %d\r\n", successCount)
	fmt.Printf("Success Count: %d\r\n", successCount)
	fmt.Printf("Failure Count: %d\r\n", failureCount)
	fmt.Printf("Time Elapsed: %s\r\n", elapsed)
	fmt.Printf("Rate: %f msg/s\r\n", float64(publisherMultiple*messageCount)/elapsed.Seconds())

	// Purge all queues.
	purgeAllPublisherTestQueues("PubTQ", channelPool)

	// Shut down everything.
	publisher1.StopAutoPublish()
	publisher2.StopAutoPublish()
	channelPool.Shutdown()
}

func TestFourAutoPublishSameChannelPool(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	messageCount := 50000
	publisherMultiple := 4

	channelPool, err := pools.NewChannelPool(Seasoning.PoolConfig, nil, true)
	assert.NoError(t, err)

	channelPool.FlushErrors()

	// Purge all queues first.
	purgeAllPublisherTestQueues("PubTQ", ChannelPool)

	publisher1, p1Err := publisher.NewPublisher(Seasoning, channelPool, nil)
	assert.NoError(t, p1Err)

	publisher2, p2Err := publisher.NewPublisher(Seasoning, channelPool, nil)
	assert.NoError(t, p2Err)

	publisher3, p3Err := publisher.NewPublisher(Seasoning, channelPool, nil)
	assert.NoError(t, p3Err)

	publisher4, p4Err := publisher.NewPublisher(Seasoning, channelPool, nil)
	assert.NoError(t, p4Err)

	// Pre-create test messages
	timeStart := time.Now()
	letters := make([]*models.Letter, messageCount)

	for i := 0; i < messageCount; i++ {
		letters[i] = utils.CreateMockLetter(uint64(i), "", fmt.Sprintf("PubTQ-%d", i%10), nil)
	}

	elapsed := time.Since(timeStart)
	fmt.Printf("Time Elapsed Creating Letters: %s\r\n", elapsed)

	timeStart = time.Now()
	publisher1.StartAutoPublish(false)
	publisher2.StartAutoPublish(false)
	publisher3.StartAutoPublish(false)
	publisher4.StartAutoPublish(false)

	go func() {

		for _, letter := range letters {
			publisher1.QueueLetter(letter)
			publisher2.QueueLetter(letter)
			publisher3.QueueLetter(letter)
			publisher4.QueueLetter(letter)
		}
	}()

	successCount := 0
	failureCount := 0
	timer := time.NewTimer(1 * time.Minute)

	var notification *models.Notification
ListeningForNotificationsLoop:
	for {
		select {
		case <-timer.C:
			fmt.Printf(" == Timeout Occurred == ")
			break ListeningForNotificationsLoop
		case chanErr := <-channelPool.Errors():
			if chanErr != nil {
				failureCount++
			}
			break
		case notification = <-publisher1.Notifications():
		case notification = <-publisher2.Notifications():
		case notification = <-publisher3.Notifications():
		case notification = <-publisher4.Notifications():
		default:
			time.Sleep(1 * time.Millisecond)
			break
		}

		if notification != nil {
			if notification.Success {
				successCount++
			} else {
				failureCount++
			}

			notification = nil
		}

		if successCount+failureCount == publisherMultiple*messageCount {
			break ListeningForNotificationsLoop
		}
	}

	elapsed = time.Since(timeStart)

	assert.Equal(t, publisherMultiple*messageCount, successCount+failureCount)
	fmt.Printf("All Messages Accounted For: %d\r\n", successCount)
	fmt.Printf("Success Count: %d\r\n", successCount)
	fmt.Printf("Failure Count: %d\r\n", failureCount)
	fmt.Printf("Time Elapsed: %s\r\n", elapsed)
	fmt.Printf("Rate: %f msg/s\r\n", float64(publisherMultiple*messageCount)/elapsed.Seconds())

	// Purge all queues.
	purgeAllPublisherTestQueues("PubTQ", ChannelPool)

	// Shut down everything.
	publisher1.StopAutoPublish()
	publisher2.StopAutoPublish()
	publisher3.StopAutoPublish()
	publisher4.StopAutoPublish()
	channelPool.Shutdown()
}

func TestFourAutoPublishFourChannelPool(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	messageCount := 50000
	publisherMultiple := 4

	channelPool1, err := pools.NewChannelPool(Seasoning.PoolConfig, nil, true)
	assert.NoError(t, err)

	channelPool2, err := pools.NewChannelPool(Seasoning.PoolConfig, nil, true)
	assert.NoError(t, err)

	channelPool3, err := pools.NewChannelPool(Seasoning.PoolConfig, nil, true)
	assert.NoError(t, err)

	channelPool4, err := pools.NewChannelPool(Seasoning.PoolConfig, nil, true)
	assert.NoError(t, err)

	channelPool1.FlushErrors()
	channelPool2.FlushErrors()
	channelPool3.FlushErrors()
	channelPool4.FlushErrors()

	// Purge all queues first.
	purgeAllPublisherTestQueues("PubTQ", ChannelPool)

	publisher1, p1Err := publisher.NewPublisher(Seasoning, channelPool1, nil)
	assert.NoError(t, p1Err)

	publisher2, p2Err := publisher.NewPublisher(Seasoning, channelPool2, nil)
	assert.NoError(t, p2Err)

	publisher3, p3Err := publisher.NewPublisher(Seasoning, channelPool3, nil)
	assert.NoError(t, p3Err)

	publisher4, p4Err := publisher.NewPublisher(Seasoning, channelPool4, nil)
	assert.NoError(t, p4Err)

	// Pre-create test messages
	timeStart := time.Now()
	letters := make([]*models.Letter, messageCount)

	for i := 0; i < messageCount; i++ {
		letters[i] = utils.CreateMockLetter(uint64(i), "", fmt.Sprintf("PubTQ-%d", i%10), nil)
	}

	elapsed := time.Since(timeStart)
	fmt.Printf("Time Elapsed Creating Letters: %s\r\n", elapsed)

	timeStart = time.Now()
	publisher1.StartAutoPublish(false)
	publisher2.StartAutoPublish(false)
	publisher3.StartAutoPublish(false)
	publisher4.StartAutoPublish(false)

	go func() {

		for _, letter := range letters {
			publisher1.QueueLetter(letter)
			publisher2.QueueLetter(letter)
			publisher3.QueueLetter(letter)
			publisher4.QueueLetter(letter)
		}
	}()

	successCount := 0
	failureCount := 0
	timer := time.NewTimer(1 * time.Minute)

	var notification *models.Notification
ListeningForNotificationsLoop:
	for {
		select {
		case <-timer.C:
			break ListeningForNotificationsLoop
		case notification = <-publisher1.Notifications():
		case notification = <-publisher2.Notifications():
		case notification = <-publisher3.Notifications():
		case notification = <-publisher4.Notifications():
		default:
			time.Sleep(1 * time.Millisecond)
			break
		}

		if notification != nil {
			if notification.Success {
				successCount++
			} else {
				failureCount++
			}

			notification = nil
		}

		if successCount+failureCount == publisherMultiple*messageCount {
			break ListeningForNotificationsLoop
		}
	}

	elapsed = time.Since(timeStart)

	assert.Equal(t, publisherMultiple*messageCount, successCount+failureCount)
	fmt.Printf("All Messages Accounted For: %d\r\n", successCount)
	fmt.Printf("Success Count: %d\r\n", successCount)
	fmt.Printf("Failure Count: %d\r\n", failureCount)
	fmt.Printf("Time Elapsed: %s\r\n", elapsed)
	fmt.Printf("Rate: %f msg/s\r\n", float64(publisherMultiple*messageCount)/elapsed.Seconds())

	// Purge all queues.
	purgeAllPublisherTestQueues("PubTQ", ChannelPool)

	// Shut down everything.
	publisher1.StopAutoPublish()
	publisher2.StopAutoPublish()
	publisher3.StopAutoPublish()
	publisher4.StopAutoPublish()
	channelPool1.Shutdown()
	channelPool2.Shutdown()
	channelPool3.Shutdown()
	channelPool4.Shutdown()
}

func purgeAllPublisherTestQueues(queuePrefix string, channelPool *pools.ChannelPool) {
	topologer, err := topology.NewTopologer(channelPool)
	if err == nil {
		for i := 0; i < 10; i++ {
			topologer.PurgeQueue(fmt.Sprintf("%s-%d", queuePrefix, i), false)
		}
	}
}
