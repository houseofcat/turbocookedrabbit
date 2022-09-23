package main_test

import (
	"testing"

	"github.com/houseofcat/turbocookedrabbit/v2/pkg/tcr"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

type Config struct {
	Seasoning             *tcr.RabbitSeasoning
	AckableConsumerConfig *tcr.ConsumerConfig
	ConsumerConfig        *tcr.ConsumerConfig
	ConnectionPool        *tcr.ConnectionPool
	RabbitService         *tcr.RabbitService
}

func (cfg *Config) Close() {
	cfg.RabbitService.Shutdown(false)
	cfg.ConnectionPool.Shutdown()
}

func InitTestService(t *testing.T) (c *Config, closer func()) {
	var cfg Config
	var err error
	cfg.Seasoning, err = tcr.ConvertJSONFileToConfig("testseasoning.json") // Load Configuration On Startup
	if err != nil {
		t.Fatal(err)
	}

	cfg.RabbitService, err = tcr.NewRabbitService(cfg.Seasoning, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	cfg.ConnectionPool = cfg.RabbitService.ConnectionPool

	cfg.AckableConsumerConfig, err = cfg.RabbitService.GetConsumerConfig("TurboCookedRabbitConsumer-Ackable")
	if err != nil {
		t.Fatal(err)
	}

	cfg.ConsumerConfig, err = cfg.RabbitService.GetConsumerConfig("TurboCookedRabbitConsumer")
	if err != nil {
		t.Fatal(err)
	}

	err = cfg.RabbitService.Topologer.CreateQueue("TcrTestQueue", false, true, false, false, false, nil)
	if err != nil {
		t.Fatal(err)
	}

	return &cfg, func() {
		_, _ = cfg.RabbitService.Topologer.QueueDelete("TcrTestQueue", false, false, false)
		cfg.Close()
	}
}

func InitBenchService(b *testing.B) (c *Config, closer func()) {
	var cfg Config
	var err error
	cfg.Seasoning, err = tcr.ConvertJSONFileToConfig("testseasoning.json") // Load Configuration On Startup
	if err != nil {
		b.Fatal(err)
	}

	cfg.RabbitService, err = tcr.NewRabbitService(cfg.Seasoning, "", "", nil, nil)
	if err != nil {
		b.Fatal(err)
	}

	cfg.ConnectionPool = cfg.RabbitService.ConnectionPool

	cfg.AckableConsumerConfig, err = cfg.RabbitService.GetConsumerConfig("TurboCookedRabbitConsumer-Ackable")
	if err != nil {
		b.Fatal(err)
	}

	cfg.ConsumerConfig, err = cfg.RabbitService.GetConsumerConfig("TurboCookedRabbitConsumer")
	if err != nil {
		b.Fatal(err)
	}

	err = cfg.RabbitService.Topologer.CreateQueue("TcrTestQueue", false, true, false, false, false, nil)
	if err != nil {
		b.Fatal(err)
	}

	return &cfg, func() {
		_, _ = cfg.RabbitService.Topologer.QueueDelete("TcrTestQueue", false, false, false)
		cfg.Close()
	}
}
