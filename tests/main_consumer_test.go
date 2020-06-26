package main_test

import (
	"testing"

	"github.com/fortytw2/leaktest"
	"github.com/houseofcat/turbocookedrabbit/pkg/tcr"
	"github.com/stretchr/testify/assert"
)

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
