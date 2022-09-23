package main_test

import (
	"fmt"
	"testing"

	"github.com/houseofcat/turbocookedrabbit/v2/pkg/tcr"
	"github.com/stretchr/testify/assert"
)

func TestCreateConsumer(t *testing.T) {
	cfg, closer := InitTestService(t)
	defer closer()

	consumer1 := tcr.NewConsumerFromConfig(cfg.AckableConsumerConfig, cfg.ConnectionPool)
	assert.NotNil(t, consumer1)

	consumer2 := tcr.NewConsumerFromConfig(cfg.ConsumerConfig, cfg.ConnectionPool)
	assert.NotNil(t, consumer2)
}

func TestStartStopConsumer(t *testing.T) {
	cfg, closer := InitTestService(t)
	defer closer()

	consumer := tcr.NewConsumerFromConfig(cfg.ConsumerConfig, cfg.ConnectionPool)
	assert.NotNil(t, consumer)

	consumer.StartConsuming()
	err := consumer.StopConsuming(false, false)
	assert.NoError(t, err)

}

func TestStartWithActionStopConsumer(t *testing.T) {
	cfg, closer := InitTestService(t)
	defer closer()

	consumer := tcr.NewConsumerFromConfig(cfg.ConsumerConfig, cfg.ConnectionPool)
	assert.NotNil(t, consumer)

	consumer.StartConsumingWithAction(
		func(msg *tcr.ReceivedMessage) {
			if err := msg.Acknowledge(); err != nil {
				fmt.Printf("Error acking message: %v\r\n", msg.Delivery.Body)
			}
		})
	err := consumer.StopConsuming(false, false)
	assert.NoError(t, err)

}

func TestConsumerGet(t *testing.T) {
	cfg, closer := InitTestService(t)
	defer closer()

	consumer := tcr.NewConsumerFromConfig(cfg.ConsumerConfig, cfg.ConnectionPool)
	assert.NotNil(t, consumer)

	delivery, err := consumer.Get("TcrTestQueue")
	assert.Nil(t, delivery) // empty queue should be nil
	assert.NoError(t, err)
}
