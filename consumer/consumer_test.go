package consumer_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/fortytw2/leaktest"
	"github.com/houseofcat/turbocookedrabbit/consumer"
	"github.com/houseofcat/turbocookedrabbit/models"
	"github.com/houseofcat/turbocookedrabbit/pools"
	"github.com/houseofcat/turbocookedrabbit/utils"
	"github.com/stretchr/testify/assert"
)

var Seasoning *models.RabbitSeasoning
var ConnectionPool *pools.ConnectionPool
var AckableConsumerConfig *models.ConsumerConfig
var ConsumerConfig *models.ConsumerConfig

func TestMain(m *testing.M) {

	var err error
	Seasoning, err = utils.ConvertJSONFileToConfig("testconsumerseasoning.json") // Load Configuration On Startup
	if err != nil {
		return
	}
	ConnectionPool, err = pools.NewConnectionPool(Seasoning.PoolConfig)
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

func TestCreateConsumer(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	consumer1, err1 := consumer.NewConsumerFromConfig(AckableConsumerConfig, ConnectionPool)
	assert.NoError(t, err1)
	assert.NotNil(t, consumer1)

	consumer2, err2 := consumer.NewConsumerFromConfig(ConsumerConfig, ConnectionPool)
	assert.NoError(t, err2)
	assert.NotNil(t, consumer2)

	ConnectionPool.Shutdown()
}

func TestStartStopConsumer(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	consumer, err := consumer.NewConsumerFromConfig(ConsumerConfig, ConnectionPool)
	assert.NoError(t, err)
	assert.NotNil(t, consumer)

	consumer.StartConsuming()
	err = consumer.StopConsuming(false, false)
	assert.NoError(t, err)

	ConnectionPool.Shutdown()
}
