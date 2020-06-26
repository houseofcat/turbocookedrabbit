package main_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/streadway/amqp"
	"github.com/stretchr/testify/assert"

	"github.com/houseofcat/turbocookedrabbit/pkg/tcr"
)

var Seasoning *tcr.RabbitSeasoning
var ConnectionPool *tcr.ConnectionPool
var AckableConsumerConfig *tcr.ConsumerConfig
var ConsumerConfig *tcr.ConsumerConfig

func TestMain(m *testing.M) {

	var err error
	Seasoning, err = tcr.ConvertJSONFileToConfig("testseasoning.json") // Load Configuration On Startup
	if err != nil {
		return
	}
	ConnectionPool, err = tcr.NewConnectionPool(Seasoning.PoolConfig)
	if err != nil {
		fmt.Print(err.Error())
		return
	}

	if config, ok := Seasoning.ConsumerConfigs["TurboCookedRabbitConsumer-Ackable"]; ok {
		AckableConsumerConfig = config
	}

	if config, ok := Seasoning.ConsumerConfigs["TurboCookedRabbitConsumer"]; ok {
		ConsumerConfig = config
	}

	os.Exit(m.Run())
}

func TestReadConfig(t *testing.T) {
	fileNamePath := "testseasoning.json"

	assert.FileExists(t, fileNamePath)

	config, err := tcr.ConvertJSONFileToConfig(fileNamePath)

	assert.Nil(t, err)
	assert.NotEqual(t, "", config.PoolConfig.ConnectionPoolConfig.URI, "RabbitMQ URI should not be blank.")
}

func TestBasicPublish(t *testing.T) {

	defer leaktest.Check(t)()

	messageCount := 100000

	// Pre-create test messages
	timeStart := time.Now()
	letters := make([]*tcr.Letter, messageCount)

	for i := 0; i < messageCount; i++ {
		letters[i] = tcr.CreateMockLetter(uint64(i), "", fmt.Sprintf("TestQueue-%d", i%10), nil)
	}

	elapsed := time.Since(timeStart)
	t.Logf("Time Elapsed Creating Letters: %s\r\n", elapsed)

	// Test
	timeStart = time.Now()
	amqpConn, err := amqp.Dial(Seasoning.PoolConfig.ConnectionPoolConfig.URI)
	if err != nil {
		return
	}

	amqpChan, err := amqpConn.Channel()
	if err != nil {
		return
	}

	for i := 0; i < messageCount; i++ {
		letter := letters[i]

		err = amqpChan.Publish(
			letter.Envelope.Exchange,
			letter.Envelope.RoutingKey,
			letter.Envelope.Mandatory,
			letter.Envelope.Immediate,
			amqp.Publishing{
				ContentType: letter.Envelope.ContentType,
				Body:        letter.Body,
			})

		if err != nil {
			t.Log(err)
		}

	}

	elapsed = time.Since(timeStart)
	t.Logf("Publish Time: %s\r\n", elapsed)
	t.Logf("Rate: %f msg/s\r\n", float64(messageCount)/elapsed.Seconds())
}

func TestReadTopologyConfig(t *testing.T) {
	fileNamePath := "testtopology.json"

	assert.FileExists(t, fileNamePath)

	config, err := tcr.ConvertJSONFileToTopologyConfig(fileNamePath)

	assert.Nil(t, err)
	assert.NotEqual(t, 0, len(config.Exchanges))
	assert.NotEqual(t, 0, len(config.Queues))
	assert.NotEqual(t, 0, len(config.QueueBindings))
	assert.NotEqual(t, 0, len(config.ExchangeBindings))
}

func TestCreateTopologyFromTopologyConfig(t *testing.T) {

	fileNamePath := "testtopology.json"
	assert.FileExists(t, fileNamePath)

	topologyConfig, err := tcr.ConvertJSONFileToTopologyConfig(fileNamePath)
	assert.NoError(t, err)

	connectionPool, err := tcr.NewConnectionPool(Seasoning.PoolConfig)
	assert.NoError(t, err)

	topologer := tcr.NewTopologer(connectionPool)

	err = topologer.BuildToplogy(topologyConfig, true)
	assert.NoError(t, err)
}

func TestCreateMultipleTopologyFromTopologyConfig(t *testing.T) {

	connectionPool, err := tcr.NewConnectionPool(Seasoning.PoolConfig)
	assert.NoError(t, err)

	topologer := tcr.NewTopologer(connectionPool)

	topologyConfigs := make([]string, 0)
	configRoot := "./"
	err = filepath.Walk(configRoot, func(path string, info os.FileInfo, err error) error {
		if strings.Contains(path, "topology") {
			topologyConfigs = append(topologyConfigs, path)
		}
		return nil
	})
	assert.NoError(t, err)

	for _, filePath := range topologyConfigs {
		topologyConfig, err := tcr.ConvertJSONFileToTopologyConfig(filePath)
		if err != nil {
			assert.NoError(t, err)
		} else {
			err = topologer.BuildToplogy(topologyConfig, false)
			assert.NoError(t, err)
		}
	}
}

func TestUnbindQueue(t *testing.T) {

	connectionPool, err := tcr.NewConnectionPool(Seasoning.PoolConfig)
	assert.NoError(t, err)

	topologer := tcr.NewTopologer(connectionPool)

	err = topologer.UnbindQueue("QueueAttachedToExch01", "RoutingKey1", "MyTestExchange.Child01", nil)
	assert.NoError(t, err)
}

func TestCreateConsumer(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	consumer1, err1 := tcr.NewConsumerFromConfig(AckableConsumerConfig, ConnectionPool)
	assert.NoError(t, err1)
	assert.NotNil(t, consumer1)

	consumer2, err2 := tcr.NewConsumerFromConfig(ConsumerConfig, ConnectionPool)
	assert.NoError(t, err2)
	assert.NotNil(t, consumer2)

	ConnectionPool.Shutdown()
}

func TestStartStopConsumer(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	consumer, err := tcr.NewConsumerFromConfig(ConsumerConfig, ConnectionPool)
	assert.NoError(t, err)
	assert.NotNil(t, consumer)

	consumer.StartConsuming()
	err = consumer.StopConsuming(false, false)
	assert.NoError(t, err)

	ConnectionPool.Shutdown()
}

func TestCreatePublisherAndPublish(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

}
