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
	"github.com/houseofcat/turbocookedrabbit/utils"
)

var Seasoning *models.RabbitSeasoning

func TestMain(m *testing.M) { // Load Configuration On Startup
	var err error
	Seasoning, err = utils.ConvertJSONFileToConfig("testpublisherseasoning.json")
	if err != nil {
		return
	}
	os.Exit(m.Run())
}

func TestCreatePublisher(t *testing.T) {
	channelPool, err := pools.NewChannelPool(Seasoning, nil, true)
	assert.NoError(t, err)

	channelPool.FlushErrors()

	publisher, err := publisher.NewPublisher(Seasoning, channelPool, nil, 0)
	assert.NoError(t, err)
	assert.NotNil(t, publisher)
}

func TestCreatePublisherAndPublish(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.
	channelPool, err := pools.NewChannelPool(Seasoning, nil, true)
	assert.NoError(t, err)

	channelPool.FlushErrors()

	publisher, err := publisher.NewPublisher(Seasoning, channelPool, nil, 0)
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
	channelPool, err := pools.NewChannelPool(Seasoning, nil, true)
	assert.NoError(t, err)

	channelPool.FlushErrors()

	publisher, err := publisher.NewPublisher(Seasoning, channelPool, nil, 1)
	assert.NoError(t, err)

	letter := utils.CreateLetter("", "TestQueue", nil)

	publisher.StartAutoPublish(false)

	err = publisher.QueueLetter(letter)
	assert.NoError(t, err)

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

	channelPool, err := pools.NewChannelPool(Seasoning, nil, true)
	assert.NoError(t, err)

	channelPool.FlushErrors()

	publisher, err := publisher.NewPublisher(Seasoning, channelPool, nil, 1)
	assert.NoError(t, err)

	// Pre-create test messages
	timeStart := time.Now()
	letters := make([]*models.Letter, messageCount)

	for i := 0; i < messageCount; i++ {
		letters[i] = utils.CreateLetter("", fmt.Sprintf("TestQueue-%d", i%10), nil)
	}

	elapsed := time.Since(timeStart)
	fmt.Printf("Time Elapsed Creating Letters: %s\r\n", elapsed)

	timeStart = time.Now()
	publisher.StartAutoPublish(false)

	go func() {

		for _, letter := range letters {
			err = publisher.QueueLetter(letter)
			assert.NoError(t, err)
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

	// Shut down everything.
	publisher.StopAutoPublish()
	channelPool.Shutdown()
}

func TestTwoAutoPublishSameChannelPool(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.
	messageCount := 50000
	publisherMultiple := 2

	channelPool, err := pools.NewChannelPool(Seasoning, nil, true)
	assert.NoError(t, err)

	channelPool.FlushErrors()

	publisher1, p1Err := publisher.NewPublisher(Seasoning, channelPool, nil, 1)
	assert.NoError(t, p1Err)

	publisher2, p2Err := publisher.NewPublisher(Seasoning, channelPool, nil, 1)
	assert.NoError(t, p2Err)

	// Pre-create test messages
	timeStart := time.Now()
	letters := make([]*models.Letter, messageCount)

	for i := 0; i < messageCount; i++ {
		letters[i] = utils.CreateLetter("", fmt.Sprintf("TestQueue-%d", i%10), nil)
	}

	elapsed := time.Since(timeStart)
	fmt.Printf("Time Elapsed Creating Letters: %s\r\n", elapsed)

	timeStart = time.Now()
	publisher1.StartAutoPublish(false)
	publisher2.StartAutoPublish(false)

	go func() {

		for _, letter := range letters {
			err = publisher1.QueueLetter(letter)
			assert.NoError(t, err)

			err = publisher2.QueueLetter(letter)
			assert.NoError(t, err)
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
		case notification := <-publisher1.Notifications():
			if notification.Success {
				successCount++
			} else {
				failureCount++
			}

			if successCount+failureCount == publisherMultiple*messageCount {
				break ListeningForNotificationsLoop
			}

			break
		case notification := <-publisher2.Notifications():
			if notification.Success {
				successCount++
			} else {
				failureCount++
			}

			if successCount+failureCount == publisherMultiple*messageCount {
				break ListeningForNotificationsLoop
			}

			break
		default:
			time.Sleep(1 * time.Millisecond)
			break
		}
	}

	elapsed = time.Since(timeStart)

	assert.Equal(t, publisherMultiple*messageCount, successCount+failureCount)
	fmt.Printf("All Messages Accounted For: %d\r\n", successCount)
	fmt.Printf("Success Count: %d\r\n", successCount)
	fmt.Printf("Failure Count: %d\r\n", failureCount)
	fmt.Printf("Time Elapsed: %s\r\n", elapsed)
	fmt.Printf("Rate: %f msg/s\r\n", float64(publisherMultiple*messageCount)/elapsed.Seconds())

	// Shut down everything.
	publisher1.StopAutoPublish()
	publisher2.StopAutoPublish()
	channelPool.Shutdown()
}

func TestFourAutoPublishSameChannelPool(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.
	messageCount := 50000
	publisherMultiple := 4

	channelPool, err := pools.NewChannelPool(Seasoning, nil, true)
	assert.NoError(t, err)

	channelPool.FlushErrors()

	publisher1, p1Err := publisher.NewPublisher(Seasoning, channelPool, nil, 1)
	assert.NoError(t, p1Err)

	publisher2, p2Err := publisher.NewPublisher(Seasoning, channelPool, nil, 1)
	assert.NoError(t, p2Err)

	publisher3, p3Err := publisher.NewPublisher(Seasoning, channelPool, nil, 1)
	assert.NoError(t, p3Err)

	publisher4, p4Err := publisher.NewPublisher(Seasoning, channelPool, nil, 1)
	assert.NoError(t, p4Err)

	// Pre-create test messages
	timeStart := time.Now()
	letters := make([]*models.Letter, messageCount)

	for i := 0; i < messageCount; i++ {
		letters[i] = utils.CreateLetter("", fmt.Sprintf("TestQueue-%d", i%10), nil)
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
			err = publisher1.QueueLetter(letter)
			assert.NoError(t, err)

			err = publisher2.QueueLetter(letter)
			assert.NoError(t, err)

			err = publisher3.QueueLetter(letter)
			assert.NoError(t, err)

			err = publisher4.QueueLetter(letter)
			assert.NoError(t, err)
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
		case notification := <-publisher1.Notifications():
			if notification.Success {
				successCount++
			} else {
				failureCount++
			}

			if successCount+failureCount == publisherMultiple*messageCount {
				break ListeningForNotificationsLoop
			}

			break
		case notification := <-publisher2.Notifications():
			if notification.Success {
				successCount++
			} else {
				failureCount++
			}

			if successCount+failureCount == publisherMultiple*messageCount {
				break ListeningForNotificationsLoop
			}

			break
		case notification := <-publisher3.Notifications():
			if notification.Success {
				successCount++
			} else {
				failureCount++
			}

			if successCount+failureCount == publisherMultiple*messageCount {
				break ListeningForNotificationsLoop
			}

			break
		case notification := <-publisher4.Notifications():
			if notification.Success {
				successCount++
			} else {
				failureCount++
			}

			if successCount+failureCount == publisherMultiple*messageCount {
				break ListeningForNotificationsLoop
			}

			break
		default:
			time.Sleep(1 * time.Millisecond)
			break
		}
	}

	elapsed = time.Since(timeStart)

	assert.Equal(t, publisherMultiple*messageCount, successCount+failureCount)
	fmt.Printf("All Messages Accounted For: %d\r\n", successCount)
	fmt.Printf("Success Count: %d\r\n", successCount)
	fmt.Printf("Failure Count: %d\r\n", failureCount)
	fmt.Printf("Time Elapsed: %s\r\n", elapsed)
	fmt.Printf("Rate: %f msg/s\r\n", float64(publisherMultiple*messageCount)/elapsed.Seconds())

	// Shut down everything.
	publisher1.StopAutoPublish()
	publisher2.StopAutoPublish()
	publisher3.StopAutoPublish()
	publisher4.StopAutoPublish()
	channelPool.Shutdown()
}

func TestFourAutoPublishFourChannelPool(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.
	Seasoning.PoolConfig.ConnectionCount = 3
	Seasoning.PoolConfig.ChannelCount = 12
	messageCount := 50000
	publisherMultiple := 4

	channelPool1, err := pools.NewChannelPool(Seasoning, nil, true)
	assert.NoError(t, err)

	channelPool2, err := pools.NewChannelPool(Seasoning, nil, true)
	assert.NoError(t, err)

	channelPool3, err := pools.NewChannelPool(Seasoning, nil, true)
	assert.NoError(t, err)

	channelPool4, err := pools.NewChannelPool(Seasoning, nil, true)
	assert.NoError(t, err)

	channelPool1.FlushErrors()

	publisher1, p1Err := publisher.NewPublisher(Seasoning, channelPool1, nil, 1)
	assert.NoError(t, p1Err)

	publisher2, p2Err := publisher.NewPublisher(Seasoning, channelPool2, nil, 1)
	assert.NoError(t, p2Err)

	publisher3, p3Err := publisher.NewPublisher(Seasoning, channelPool3, nil, 1)
	assert.NoError(t, p3Err)

	publisher4, p4Err := publisher.NewPublisher(Seasoning, channelPool4, nil, 1)
	assert.NoError(t, p4Err)

	// Pre-create test messages
	timeStart := time.Now()
	letters := make([]*models.Letter, messageCount)

	for i := 0; i < messageCount; i++ {
		letters[i] = utils.CreateLetter("", fmt.Sprintf("TestQueue-%d", i%10), nil)
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
			err = publisher1.QueueLetter(letter)
			assert.NoError(t, err)

			err = publisher2.QueueLetter(letter)
			assert.NoError(t, err)

			err = publisher3.QueueLetter(letter)
			assert.NoError(t, err)

			err = publisher4.QueueLetter(letter)
			assert.NoError(t, err)
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
		case notification := <-publisher1.Notifications():
			if notification.Success {
				successCount++
			} else {
				failureCount++
			}

			if successCount+failureCount == publisherMultiple*messageCount {
				break ListeningForNotificationsLoop
			}

			break
		case notification := <-publisher2.Notifications():
			if notification.Success {
				successCount++
			} else {
				failureCount++
			}

			if successCount+failureCount == publisherMultiple*messageCount {
				break ListeningForNotificationsLoop
			}

			break
		case notification := <-publisher3.Notifications():
			if notification.Success {
				successCount++
			} else {
				failureCount++
			}

			if successCount+failureCount == publisherMultiple*messageCount {
				break ListeningForNotificationsLoop
			}

			break
		case notification := <-publisher4.Notifications():
			if notification.Success {
				successCount++
			} else {
				failureCount++
			}

			if successCount+failureCount == publisherMultiple*messageCount {
				break ListeningForNotificationsLoop
			}

			break
		default:
			time.Sleep(1 * time.Millisecond)
			break
		}
	}

	elapsed = time.Since(timeStart)

	assert.Equal(t, publisherMultiple*messageCount, successCount+failureCount)
	fmt.Printf("All Messages Accounted For: %d\r\n", successCount)
	fmt.Printf("Success Count: %d\r\n", successCount)
	fmt.Printf("Failure Count: %d\r\n", failureCount)
	fmt.Printf("Time Elapsed: %s\r\n", elapsed)
	fmt.Printf("Rate: %f msg/s\r\n", float64(publisherMultiple*messageCount)/elapsed.Seconds())

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
