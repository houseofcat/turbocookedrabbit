package publisher_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/houseofcat/turbocookedrabbit/v1/pkg/models"
	"github.com/houseofcat/turbocookedrabbit/v1/pkg/publisher"
	"github.com/houseofcat/turbocookedrabbit/v1/pkg/utils"
)

type TestStruct struct {
	PropertyString1 string `json:"PropertyString1"`
	PropertyString2 string `json:"PropertyString2"`
	PropertyString3 string `json:"PropertyString3"`
	PropertyString4 string `json:"PropertyString4"`
}

func BenchmarkAutoPublishRandomLetters(b *testing.B) {
	b.ReportAllocs()

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
	//purgeAllPublisherTestQueues(testQueuePrefix, ChannelPool)
}

func BenchmarkAutoPublishRandomEncryptedLetters(b *testing.B) {
	b.ReportAllocs()

	testQueuePrefix := "PubTQ"
	b.Logf("%s: Purging Queues...", time.Now())
	purgeAllPublisherTestQueues(testQueuePrefix, ChannelPool)

	messageCount := 100
	letters := make([]*models.Letter, messageCount)

	pub, err := publisher.NewPublisher(Seasoning, ChannelPool, ConnectionPool)
	if err != nil {
		b.Log(err.Error())
	}

	monitorLoopFinished := make(chan bool)
	pub.StartAutoPublish(false)

	// Build letters
	buildLetters(b, messageCount, testQueuePrefix, letters)
	go monitorLoop(b, monitorLoopFinished, pub, messageCount)

	// Queue letters
	startTime := time.Now()
	b.Logf("%s: Queueing Letters", time.Now())
	for i := 0; i < messageCount; i++ {
		pub.QueueLetter(letters[i])
	}
	b.Logf("%s: Finished Queueing letters after %s", time.Now(), time.Since(startTime))

	<-monitorLoopFinished

	b.Logf("%s: Purging Queues...", time.Now())
	//purgeAllPublisherTestQueues(testQueuePrefix, ChannelPool)
}

func buildLetters(b *testing.B, messageCount int, testQueuePrefix string, letters []*models.Letter) {

	totalBytes := uint64(0)
	eo, co, testStructure := setupEncryption()

	b.Logf("%s: Building Letters", time.Now())
	for i := 0; i < messageCount; i++ {
		body, err := utils.CreatePayload(testStructure, co, eo)
		if err != nil {
			b.Error(err)
		}

		letters[i] = utils.CreateLetter(1, "", fmt.Sprintf("%s-%d", testQueuePrefix, i%10), body)
		letters[i].Envelope.DeliveryMode = 2
		totalBytes += uint64(len(letters[i].Body))
	}
	b.Logf("%s: Finished Building Letters", time.Now())
	b.Logf("%s: Total Size Created: %f MB", time.Now(), float64(totalBytes)/1000000.0)
}

func monitorLoop(b *testing.B, monitorLoopFinished chan bool, pub *publisher.Publisher, messageCount int) {
	go func() {

		startTime := time.Now()
		messagesPublished := 0
		messgaesFailedToPublish := 0

	MonitorLoop:
		for {
			select {
			case notification := <-pub.Notifications():
				if notification.Success {
					messagesPublished++
				} else {
					messgaesFailedToPublish++
				}

				if messagesPublished+messgaesFailedToPublish == messageCount {
					break MonitorLoop

				}
				break
			default:
				break
			}
		}
		testDuration := time.Since(startTime)

		b.Logf("%s: Messages Published: %d\r\n", time.Now(), messagesPublished)
		b.Logf("%s: Messages Failed To Publish: %d\r\n", time.Now(), messgaesFailedToPublish)
		b.Logf("%s: %f Msg/s", time.Now(), float64(messageCount)/testDuration.Seconds())

		monitorLoopFinished <- true
		pub.StopAutoPublish()
	}()
}

func setupEncryption() (*models.EncryptionConfig, *models.CompressionConfig, *TestStruct) {

	password := "SuperStreetFighter2Turbo"
	salt := "MBisonDidNothingWrong"

	hashy := utils.GetHashWithArgon(password, salt, 1, 12, 64, 32)

	encrypt := &models.EncryptionConfig{
		Enabled:           true,
		Hashkey:           hashy,
		Type:              "aes",
		TimeConsideration: 1,
		Threads:           2,
	}

	compression := &models.CompressionConfig{
		Enabled: true,
		Type:    "zstd",
	}

	test := &TestStruct{
		PropertyString1: utils.RandomString(1000),
		PropertyString2: utils.RandomString(1000),
	}

	return encrypt, compression, test
}
