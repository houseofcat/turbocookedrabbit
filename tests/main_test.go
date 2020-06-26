package main_test

import (
	"fmt"
	"os"
	"testing"

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
