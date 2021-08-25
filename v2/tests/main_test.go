package main_test

import (
	"os"
	"testing"

	"github.com/houseofcat/turbocookedrabbit/v2/pkg/tcr"
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

	RabbitService, err = tcr.NewRabbitService(Seasoning, "", "", nil, nil)
	if err != nil {
		return
	}

	ConnectionPool = RabbitService.ConnectionPool

	AckableConsumerConfig, err = RabbitService.GetConsumerConfig("TurboCookedRabbitConsumer-Ackable")
	if err != nil {
		return
	}

	ConsumerConfig, err = RabbitService.GetConsumerConfig("TurboCookedRabbitConsumer")
	if err != nil {
		return
	}

	err = RabbitService.Topologer.CreateQueue("TcrTestQueue", false, true, false, false, false, nil)
	if err != nil {
		return
	}

	os.Exit(m.Run())
}

func TestCleanup(t *testing.T) {
	_, _ = RabbitService.Topologer.QueueDelete("TcrTestQueue", false, false, false)
	RabbitService.Shutdown(true)
}

func BenchCleanup(b *testing.B) {
	_, _ = RabbitService.Topologer.QueueDelete("TcrTestQueue", false, false, false)
	RabbitService.Shutdown(true)
}
