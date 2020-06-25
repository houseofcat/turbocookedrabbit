package publisher_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/houseofcat/turbocookedrabbit/models"
	"github.com/houseofcat/turbocookedrabbit/publisher"
	"github.com/houseofcat/turbocookedrabbit/utils"
)

type TestStruct struct {
	PropertyString1 string `json:"PropertyString1"`
	PropertyString2 string `json:"PropertyString2"`
	PropertyString3 string `json:"PropertyString3"`
	PropertyString4 string `json:"PropertyString4"`
}

func BenchmarkAutoPublishRandomLetters(b *testing.B) {
	b.ReportAllocs()

}

func BenchmarkAutoPublishRandomEncryptedLetters(b *testing.B) {
	b.ReportAllocs()

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
			case notification := <-pub.PublishReceipts():
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
