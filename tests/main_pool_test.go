package main_test

import (
	"sync"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/houseofcat/turbocookedrabbit/pkg/tcr"
	"github.com/streadway/amqp"
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

	chanHost := cp.GetChannelFromPool()
	assert.NotNil(t, chanHost)

	cp.Shutdown()
	TestCleanup(t)
}

func TestCreateConnectionPoolAndGetChannel(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	Seasoning.PoolConfig.MaxConnectionCount = 1

	cp, err := tcr.NewConnectionPool(Seasoning.PoolConfig)
	assert.NoError(t, err)

	chanHost := cp.GetChannelFromPool()
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

		chanHost := ConnectionPool.GetChannelFromPool()

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

// TestConnectionGetConnectionAndReturnSlowLoop is similar to the above. It is designed to be slow test connection recovery by severing all connections
// and then verify connections and channels properly (and evenly over connections) restore.
func TestConnectionGetChannelAndReturnSlowLoop(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	body := []byte("\x68\x65\x6c\x6c\x6f\x20\x77\x6f\x72\x6c\x64")

	wg := &sync.WaitGroup{}
	semaphore := make(chan bool, 100) // at most 100 requests a time
	for i := 0; i < 10000; i++ {      // total request to try

		wg.Add(1)
		semaphore <- true
		go func() {
			defer wg.Done()

			chanHost := ConnectionPool.GetChannelFromPool()

			time.Sleep(time.Millisecond * 100) // artificially create channel poool contention by long exposure

			err := chanHost.Channel.Publish("", "TcrTestQueue", false, false, amqp.Publishing{
				ContentType:  "plaintext/text",
				Body:         body,
				DeliveryMode: 2,
			})

			ConnectionPool.ReturnChannel(chanHost, err != nil)

			<-semaphore
		}()
	}

	wg.Wait() // wait for the final batch of requests to finish
	ConnectionPool.Shutdown()
}
