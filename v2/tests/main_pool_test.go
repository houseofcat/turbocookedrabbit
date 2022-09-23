package main_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/houseofcat/turbocookedrabbit/v2/pkg/tcr"
	"github.com/stretchr/testify/assert"
)

func TestCreateConnectionPoolWithZeroConnections(t *testing.T) {
	cfg, closer := InitTestService(t)
	defer closer()

	cfg.Seasoning.PoolConfig.MaxConnectionCount = 0

	cp, err := tcr.NewConnectionPool(cfg.Seasoning.PoolConfig)
	assert.Nil(t, cp)
	assert.Error(t, err)

}

func TestCreateConnectionPoolWithErrorHandler(t *testing.T) {
	seasoning, err := tcr.ConvertJSONFileToConfig("badtest.json")
	if err != nil {
		return
	}

	cp, err := tcr.NewConnectionPoolWithErrorHandler(seasoning.PoolConfig, errorHandler)
	assert.Nil(t, cp)
	assert.Error(t, err)

	cp.Shutdown()
}

func errorHandler(err error) {
	fmt.Println(err)
}

func TestCreateConnectionPoolAndGetConnection(t *testing.T) {
	cfg, closer := InitTestService(t)
	defer closer()

	cfg.Seasoning.PoolConfig.MaxConnectionCount = 1

	cp, err := tcr.NewConnectionPool(cfg.Seasoning.PoolConfig)
	assert.NoError(t, err)

	conHost, err := cp.GetConnection()
	assert.NotNil(t, conHost)
	assert.NoError(t, err)

	cp.ReturnConnection(conHost, false)

	cp.Shutdown()
}

func TestCreateConnectionPoolAndGetAckableChannel(t *testing.T) {
	cfg, closer := InitTestService(t)
	defer closer()

	cfg.Seasoning.PoolConfig.MaxConnectionCount = 1

	cp, err := tcr.NewConnectionPool(cfg.Seasoning.PoolConfig)
	assert.NoError(t, err)

	chanHost, err := cp.GetChannelFromPool()
	assert.NoError(t, err)
	assert.NotNil(t, chanHost)

	cp.Shutdown()
}

func TestCreateConnectionPoolAndGetChannel(t *testing.T) {
	cfg, closer := InitTestService(t)
	defer closer()

	cfg.Seasoning.PoolConfig.MaxConnectionCount = 1

	cp, err := tcr.NewConnectionPool(cfg.Seasoning.PoolConfig)
	assert.NoError(t, err)

	chanHost, err := cp.GetChannelFromPool()
	assert.NoError(t, err)
	assert.NotNil(t, chanHost)
	chanHost.Close()

	cp.Shutdown()
}

func TestConnectionGetConnectionAndReturnLoop(t *testing.T) {
	cfg, closer := InitTestService(t)
	defer closer()

	for i := 0; i < 1000000; i++ {

		connHost, err := cfg.ConnectionPool.GetConnection()
		assert.NoError(t, err)

		cfg.ConnectionPool.ReturnConnection(connHost, false)
	}

}

func TestConnectionGetChannelAndReturnLoop(t *testing.T) {
	cfg, closer := InitTestService(t)
	defer closer()

	for i := 0; i < 1000000; i++ {

		chanHost, err := cfg.ConnectionPool.GetChannelFromPool()
		assert.NoError(t, err)

		cfg.ConnectionPool.ReturnChannel(chanHost, false)
	}
}

// TestConnectionGetConnectionAndReturnSlowLoop is designed to be slow test connection recovery by severing all connections
// and then verify connections properly restore.
func TestConnectionGetConnectionAndReturnSlowLoop(t *testing.T) {
	cfg, closer := InitTestService(t)
	defer closer()

	wg := &sync.WaitGroup{}
	semaphore := make(chan bool, 100)
	for i := 0; i < 10000; i++ {

		wg.Add(1)
		semaphore <- true
		go func() {
			defer wg.Done()

			connHost, err := cfg.ConnectionPool.GetConnection()
			assert.NoError(t, err)

			time.Sleep(time.Millisecond * 20)

			cfg.ConnectionPool.ReturnConnection(connHost, false)

			<-semaphore
		}()
	}

	wg.Wait()
}
