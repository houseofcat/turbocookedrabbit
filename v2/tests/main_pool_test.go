package main_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/houseofcat/turbocookedrabbit/v2/pkg/tcr"
	"github.com/stretchr/testify/assert"
)

func TestCreateConnectionPoolWithZeroConnections(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	Seasoning.PoolConfig.MaxConnectionCount = 0

	cp, err := tcr.NewConnectionPool(Seasoning.PoolConfig)
	assert.Nil(t, cp)
	assert.Error(t, err)

	TestCleanup(t)
}

func TestCreateConnectionPoolWithErrorHandler(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	seasoning, err := tcr.ConvertJSONFileToConfig("badtest.json")
	if err != nil {
		return
	}

	cp, err := tcr.NewConnectionPoolWithErrorHandler(seasoning.PoolConfig, errorHandler)
	assert.Nil(t, cp)
	assert.Error(t, err)

	cp.Shutdown()
	TestCleanup(t)
}

func errorHandler(err error) {
	fmt.Println(err)
}

func TestCreateConnectionPoolAndGetConnection(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	Seasoning.PoolConfig.MaxConnectionCount = 1

	cp, err := tcr.NewConnectionPool(Seasoning.PoolConfig)
	assert.NoError(t, err)

	conHost, err := cp.GetConnection()
	assert.NotNil(t, conHost)
	assert.NoError(t, err)

	cp.ReturnConnection(conHost, false)

	cp.Shutdown()
	TestCleanup(t)
}

func TestCreateConnectionPoolAndGetAckableChannel(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	Seasoning.PoolConfig.MaxConnectionCount = 1

	cp, err := tcr.NewConnectionPool(Seasoning.PoolConfig)
	assert.NoError(t, err)

	chanHost, err := cp.GetChannelFromPool()
	assert.NoError(t, err)
	assert.NotNil(t, chanHost)

	cp.Shutdown()
	TestCleanup(t)
}

func TestCreateConnectionPoolAndGetChannel(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	Seasoning.PoolConfig.MaxConnectionCount = 1

	cp, err := tcr.NewConnectionPool(Seasoning.PoolConfig)
	assert.NoError(t, err)

	chanHost, err := cp.GetChannelFromPool()
	assert.NoError(t, err)
	assert.NotNil(t, chanHost)
	chanHost.Close()

	cp.Shutdown()
	TestCleanup(t)
}

func TestConnectionGetConnectionAndReturnLoop(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	for i := 0; i < 1000000; i++ {

		connHost, err := ConnectionPool.GetConnection()
		assert.NoError(t, err)

		ConnectionPool.ReturnConnection(connHost, false)
	}

	TestCleanup(t)
}

func TestConnectionGetChannelAndReturnLoop(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	for i := 0; i < 1000000; i++ {

		chanHost, err := ConnectionPool.GetChannelFromPool()
		assert.NoError(t, err)

		ConnectionPool.ReturnChannel(chanHost, false)
	}

	TestCleanup(t)
}

// TestConnectionGetConnectionAndReturnSlowLoop is designed to be slow test connection recovery by severing all connections
// and then verify connections properly restore.
func TestConnectionGetConnectionAndReturnSlowLoop(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	wg := &sync.WaitGroup{}
	semaphore := make(chan bool, 100)
	for i := 0; i < 10000; i++ {

		wg.Add(1)
		semaphore <- true
		go func() {
			defer wg.Done()

			connHost, err := ConnectionPool.GetConnection()
			assert.NoError(t, err)

			time.Sleep(time.Millisecond * 20)

			ConnectionPool.ReturnConnection(connHost, false)

			<-semaphore
		}()
	}

	wg.Wait()
	TestCleanup(t)
}
