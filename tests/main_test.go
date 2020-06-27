package main_test

import (
	"os"
	"testing"

	"github.com/houseofcat/turbocookedrabbit/pkg/tcr"
)

var Seasoning *tcr.RabbitSeasoning
var ConnectionPool *tcr.ConnectionPool
var RabbitService *tcr.RabbitService
var AckableConsumerConfig *tcr.ConsumerConfig
var ConsumerConfig *tcr.ConsumerConfig

func TestMain(m *testing.M) {

	var err error
	Seasoning, err = tcr.ConvertJSONFileToConfig("testseasoning.json") // Load Configuration On Startup
	if err != nil {
		return
	}

	RabbitService, err = tcr.NewRabbitService(Seasoning, "", "", nil)
	ConnectionPool = RabbitService.ConnectionPool
	AckableConsumerConfig, _ = RabbitService.GetConsumerConfig("TurboCookedRabbitConsumer-Ackable")
	ConsumerConfig, _ = RabbitService.GetConsumerConfig("TurboCookedRabbitConsumer")

	RabbitService.Topologer.CreateQueue("TcrTestQueue", false, true, false, false, false, nil)

	os.Exit(m.Run())
}
