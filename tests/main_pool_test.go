package main_test

import (
	"testing"

	"github.com/fortytw2/leaktest"
	"github.com/houseofcat/turbocookedrabbit/pkg/tcr"
	"github.com/stretchr/testify/assert"
)

func TestCreateConnectionPoolWithZeroConnections(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	Seasoning.PoolConfig.ConnectionPoolConfig.MaxConnectionCount = 0

	cp, err := tcr.NewConnectionPool(Seasoning.PoolConfig)
	assert.Nil(t, cp)
	assert.Error(t, err)

	ConnectionPool.Shutdown()
}

func TestCreateConnectionPoolAndGetConnection(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	Seasoning.PoolConfig.ConnectionPoolConfig.MaxConnectionCount = 1

	cp, err := tcr.NewConnectionPool(Seasoning.PoolConfig)
	assert.NoError(t, err)

	conHost, err := cp.GetConnection()
	assert.NotNil(t, conHost)
	assert.NoError(t, err)

	cp.ReturnConnection(conHost, false)

	cp.Shutdown()
	ConnectionPool.Shutdown()
}

func TestCreateConnectionPoolAndGetAckableChannel(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	Seasoning.PoolConfig.ConnectionPoolConfig.MaxConnectionCount = 1

	cp, err := tcr.NewConnectionPool(Seasoning.PoolConfig)
	assert.NoError(t, err)

	chanHost := cp.GetChannel(true)
	assert.NotNil(t, chanHost)

	cp.Shutdown()
	ConnectionPool.Shutdown()
}

func TestCreateConnectionPoolAndGetChannel(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	Seasoning.PoolConfig.ConnectionPoolConfig.MaxConnectionCount = 1

	cp, err := tcr.NewConnectionPool(Seasoning.PoolConfig)
	assert.NoError(t, err)

	chanHost := cp.GetChannel(false)
	assert.NotNil(t, chanHost)
	chanHost.Close()

	cp.Shutdown()
	ConnectionPool.Shutdown()
}

func TestConnectionGetChannelAndReturnLoop(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	for i := 0; i < 1000000; i++ {

		chanHost := ConnectionPool.GetChannel(true)

		ConnectionPool.ReturnChannel(chanHost, false)
	}

	ConnectionPool.Shutdown()
}
