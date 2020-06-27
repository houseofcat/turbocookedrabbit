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

func TestConnectionGetConnectionAndReturnLoop(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	for i := 0; i < 1000000; i++ {

		connHost, err := ConnectionPool.GetConnection()
		assert.NoError(t, err)

		ConnectionPool.ReturnConnection(connHost, false)
	}

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
	TestCleanup()
}

// TestConnectionGetConnectionAndReturnSlowLoop is similar to the above. It is designed to be slow test connection recovery by severing all connections
// and then verify connections and channels properly (and evenly over connections) restore.
func TestConnectionGetChannelAndReturnSlowLoop(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	body := []byte("\x68\x65\x6c\x6c\x6f\x20\x77\x6f\x72\x6c\x64")

	wg := &sync.WaitGroup{}
	semaphore := make(chan bool, 100)
	for i := 0; i < 10000; i++ {

		wg.Add(1)
		semaphore <- true
		go func() {
			defer wg.Done()

			chanHost := ConnectionPool.GetChannel(true)

			time.Sleep(time.Millisecond * 100)

			err := chanHost.Channel.Publish("", "TcrTestQueue", false, false, amqp.Publishing{
				ContentType:  "plaintext/text",
				Body:         body,
				DeliveryMode: 2,
			})

			ConnectionPool.ReturnChannel(chanHost, err != nil)

			<-semaphore
		}()
	}

	wg.Wait()
	TestCleanup()
}
