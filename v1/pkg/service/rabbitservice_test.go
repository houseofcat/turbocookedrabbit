package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/houseofcat/turbocookedrabbit/models"
	"github.com/houseofcat/turbocookedrabbit/utils"
	"github.com/stretchr/testify/assert"
)

type AnonData struct {
	Data        []byte
	UTCDateTime string
}

var Config *models.RabbitSeasoning
var Service *RabbitService

func TestMain(m *testing.M) { // Load Configuration On Startup

	var err error
	Config, err = utils.ConvertJSONFileToConfig("testseasoning.json")
	if err != nil {
		fmt.Print(err)
		return
	}

	Service, err = NewRabbitService(Config)
	if err != nil {
		fmt.Print(err)
		return
	}

	Service.StartService(false)

	os.Exit(m.Run())
}

func TestSetHashForEncryption(t *testing.T) {

	password := "SuperStreetFighter2Turbo"
	salt := "MBisonDidNothingWrong"

	Service.SetHashForEncryption(password, salt)

	assert.NotEqual(t, nil, Service.Config.EncryptionConfig.Hashkey)
	assert.NotEqual(t, 0, Service.Config.EncryptionConfig.Hashkey)
}

func TestPublishWithoutWrap(t *testing.T) {

	Config.EncryptionConfig.Enabled = false
	Config.CompressionConfig.Enabled = false

	unmodifiedPayload := []byte("\x68\x65\x6c\x6c\x6f\x20\x77\x6f\x72\x6c\x64")
	anonData := struct {
		Data []byte
		Time string
	}{
		unmodifiedPayload,
		time.UTC.String(),
	}

	err := Service.Publish(anonData, "", "ServiceTestQueue", false, "")
	assert.NoError(t, err)
	time.Sleep(1 * time.Second)
}

func TestPublishWithWrap(t *testing.T) {

	Service.Config.EncryptionConfig.Enabled = false
	Service.Config.CompressionConfig.Enabled = false

	unmodifiedPayload := []byte("\x68\x65\x6c\x6c\x6f\x20\x77\x6f\x72\x6c\x64")
	anonData := &AnonData{
		unmodifiedPayload,
		time.UTC.String(),
	}

	err := Service.Publish(anonData, "", "ServiceTestQueue", true, "TestMetaData")
	assert.NoError(t, err)
	time.Sleep(1 * time.Second)
}

func TestPublishCompressionEncryptionWithoutWrap(t *testing.T) {

	password := "SuperStreetFighter2Turbo"
	salt := "MBisonDidNothingWrong"

	Service.SetHashForEncryption(password, salt)

	unmodifiedPayload := []byte("\x68\x65\x6c\x6c\x6f\x20\x77\x6f\x72\x6c\x64")
	anonData := &AnonData{
		unmodifiedPayload,
		time.UTC.String(),
	}

	err := Service.Publish(anonData, "", "ServiceTestQueue", false, "")
	assert.NoError(t, err)
	time.Sleep(1 * time.Second)
}

func TestPublishCompressionEncryptionWithWrap(t *testing.T) {

	password := "SuperStreetFighter2Turbo"
	salt := "MBisonDidNothingWrong"

	Service.SetHashForEncryption(password, salt)

	unmodifiedPayload := []byte("\x68\x65\x6c\x6c\x6f\x20\x77\x6f\x72\x6c\x64")
	anonData := &AnonData{
		unmodifiedPayload,
		time.UTC.String(),
	}

	err := Service.Publish(anonData, "", "ServiceTestQueue", true, "TestMetaData")
	assert.NoError(t, err)
	time.Sleep(1 * time.Second)
}

func TestMultiPublishCompressionEncryptionWithWrap(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	configureEncryption()

	done := make(chan bool, 1)
	go serviceMonitor(done, Service)

	payloads := createPayloads(t, 5000)
	publishPayloads(t, payloads, true)

	Service.Shutdown(true)
	done <- true
}

func TestMultiPublishEncryptionWithWrap(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	Service.Config.CompressionConfig.Enabled = false

	configureEncryption()

	done := make(chan bool, 1)
	go serviceMonitor(done, Service)

	payloads := createPayloads(t, 5000)
	publishPayloads(t, payloads, true)
	Service.Shutdown(true)
	done <- true
}

func TestMultiPublishCompressionWithWrap(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	Service.Config.EncryptionConfig.Enabled = false

	configureEncryption()

	done := make(chan bool, 1)
	go serviceMonitor(done, Service)

	payloads := createPayloads(t, 5000)
	publishPayloads(t, payloads, true)

	Service.Shutdown(true)
	done <- true
}

func TestMultiPublishAltCompressionWithWrap(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	Service.Config.EncryptionConfig.Enabled = false
	Service.Config.CompressionConfig.Type = "zstd"

	configureEncryption()

	done := make(chan bool, 1)
	go serviceMonitor(done, Service)

	payloads := createPayloads(t, 5000)
	publishPayloads(t, payloads, true)

	Service.Shutdown(true)
	done <- true
}

func TestMultiPublishWithWrap(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	Service.Config.EncryptionConfig.Enabled = false
	Service.Config.CompressionConfig.Enabled = false

	configureEncryption()

	done := make(chan bool, 1)
	go serviceMonitor(done, Service)

	payloads := createPayloads(t, 10000)
	publishPayloads(t, payloads, true)

	Service.Shutdown(true)
	done <- true
}

func TestMultiPublishWithoutWrap(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	Service.Config.EncryptionConfig.Enabled = false
	Service.Config.CompressionConfig.Enabled = false

	configureEncryption()

	done := make(chan bool, 1)
	go serviceMonitor(done, Service)

	payloads := createPayloads(t, 10000)
	publishPayloads(t, payloads, false)

	Service.Shutdown(true)
	done <- true
}

func configureEncryption() {
	password := "SuperStreetFighter2Turbo"
	salt := "MBisonDidNothingWrong"

	Service.SetHashForEncryption(password, salt)
}

func createPayloads(t *testing.T, count int) []*AnonData {

	messageCount := count
	payloads := make([]*AnonData, count)
	timeStart := time.Now()
	for i := 0; i < messageCount; i++ {
		anonData := &AnonData{
			utils.RepeatedBytes(2500, 2500),
			time.Now().UTC().Format(time.RFC3339),
		}
		payloads[i] = anonData

	}
	elapsedTime := time.Since(timeStart)
	t.Logf("%s: Creating Payloads Finished in %f s", time.Now(), elapsedTime.Seconds())

	return payloads
}

func publishPayloads(t *testing.T, payloads []*AnonData, wrap bool) {

	timeStart := time.Now()
	for i := 0; i < len(payloads); i++ {
		err := Service.Publish(payloads[i], "", "ServiceTestQueue", wrap, "")
		if err != nil {
			t.Error(err)
		}
	}
	elapsedTime := time.Since(timeStart)
	t.Logf("%s: Publishing Payloads Finished in %f s", time.Now(), elapsedTime.Seconds())
	t.Logf("%s: Published %.3f msg/s", time.Now(), float64(len(payloads))/elapsedTime.Seconds())
}

func serviceMonitor(done chan bool, service *RabbitService) {
	go func() {
	MonitorLoop:
		for {
			select {
			case <-done:
				break MonitorLoop
			case <-service.CentralErr():
			}
		}

	}()
}

func TestPublishCompressionEncryptionWithWrapAndConsume(t *testing.T) {

	password := "SuperStreetFighter2Turbo"
	salt := "MBisonDidNothingWrong"

	Service.SetHashForEncryption(password, salt)

	consumer, err := Service.GetConsumer("TCR-Compcryption")
	if err != nil {
		t.Error(err)
	}

	err = consumer.StartConsuming()
	if err != nil {
		t.Error(err)
	}

	unmodifiedPayload := []byte("\x68\x65\x6c\x6c\x6f\x20\x77\x6f\x72\x6c\x64")
	timeStamp := time.UTC.String()
	anonData := &AnonData{
		unmodifiedPayload,
		timeStamp,
	}

	err = Service.Publish(anonData, "", "ServiceTestQueue", true, "TestMetaData")
	if err != nil {
		t.Error(err)
	}

	message := <-consumer.Messages()
	modLetter := &models.ModdedLetter{}
	err = json.Unmarshal(message.Body, modLetter) // unmarshal as ModdedLetter
	if err != nil {
		t.Error(err)
	}

	if modLetter.Body == nil {
		t.Error(errors.New("body didn't deserialize"))
	}

	buffer := bytes.NewBuffer(modLetter.Body.Data)
	err = utils.ReadPayload(buffer, Service.Config.CompressionConfig, Service.Config.EncryptionConfig)
	if err != nil {
		t.Error(err)
	}

	returnedData := &AnonData{}
	err = json.Unmarshal(buffer.Bytes(), returnedData) // unmarshal inner (now modified) bytes to an actual type!
	assert.NoError(t, err)
	assert.Equal(t, timeStamp, returnedData.UTCDateTime)
	assert.Equal(t, unmodifiedPayload, returnedData.Data)
}
