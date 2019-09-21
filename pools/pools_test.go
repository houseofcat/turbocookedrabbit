package pools_test

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/assert"

	"github.com/houseofcat/turbocookedrabbit/models"
	"github.com/houseofcat/turbocookedrabbit/pools"
	"github.com/houseofcat/turbocookedrabbit/utils"

	"github.com/streadway/amqp"
)

var Seasoning *models.RabbitSeasoning
var ConnectionPool *pools.ConnectionPool
var ChannelPool *pools.ChannelPool

func TestMain(m *testing.M) { // Load Configuration On Startup
	var err error
	Seasoning, err = utils.ConvertJSONFileToConfig("testpoolseasoning.json")
	if err != nil {
		fmt.Print(err.Error())
		return
	}

	ConnectionPool, err = pools.NewConnectionPool(Seasoning.PoolConfig, true)
	if err != nil {
		fmt.Print(err.Error())
		return
	}

	ChannelPool, err = pools.NewChannelPool(Seasoning.PoolConfig, ConnectionPool, true)
	if err != nil {
		fmt.Print(err.Error())
		return
	}

	os.Exit(m.Run())
}

func TestCreateSingleChannelAndPublish(t *testing.T) {

	startTime := time.Now()
	t.Logf("%s: Benchmark Starts\r\n", startTime)

	messageCount := 100000
	messageSize := 2500
	publishErrors := 0
	published := 0

	letter := utils.CreateMockRandomLetter("ConsumerTestQueue")

	amqpConn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		t.Error(err)
		return
	}
	amqpChan, err := amqpConn.Channel()
	if err != nil {
		t.Error(err)
		return
	}

	for i := 0; i < messageCount; i++ {

		err := amqpChan.Publish(
			letter.Envelope.Exchange,
			letter.Envelope.RoutingKey,
			letter.Envelope.Mandatory,
			letter.Envelope.Immediate,
			amqp.Publishing{
				ContentType: letter.Envelope.ContentType,
				Body:        letter.Body,
			})

		if err != nil {
			publishErrors++
		} else {
			published++
		}
	}

	amqpChan.Close()
	amqpConn.Close()

	duration := time.Since(startTime)

	t.Logf("%s: Benchmark End\r\n", time.Now())
	t.Logf("%s: Time Elapsed %s\r\n", time.Now(), duration)
	t.Logf("%s: Publish Errors %d\r\n", time.Now(), publishErrors)
	t.Logf("%s: Publish Actual %d\r\n", time.Now(), published)
	t.Logf("%s: Msgs/s %f\r\n", time.Now(), float64(messageCount)/duration.Seconds())
	t.Logf("%s: MB/s %f\r\n", time.Now(), (float64(messageSize*published)/duration.Seconds())/1000000)
}

func TestGetSingleChannelFromPoolAndPublish(t *testing.T) {

	startTime := time.Now()
	t.Logf("%s: Benchmark Starts\r\n", startTime)

	messageCount := 100000
	messageSize := 2500
	published := 0

	letter := utils.CreateMockRandomLetter("ConsumerTestQueue")

	channelHost, err := ChannelPool.GetChannel()
	if err != nil {
		t.Error(err)
		return
	}
	publishErrors := 0

	for i := 0; i < messageCount; i++ {
		err := channelHost.Channel.Publish(
			letter.Envelope.Exchange,
			letter.Envelope.RoutingKey,
			letter.Envelope.Mandatory,
			letter.Envelope.Immediate,
			amqp.Publishing{
				ContentType: letter.Envelope.ContentType,
				Body:        letter.Body,
			})

		if err != nil {
			if publishErrors == 0 {
				t.Error(err)
			}
			publishErrors++
		} else {
			published++
		}
	}

	ChannelPool.ReturnChannel(channelHost, false)

	duration := time.Since(startTime)

	t.Logf("%s: Benchmark End\r\n", time.Now())
	t.Logf("%s: Time Elapsed %s\r\n", time.Now(), duration)
	t.Logf("%s: Publish Errors %d\r\n", time.Now(), publishErrors)
	t.Logf("%s: Publish Actual %d\r\n", time.Now(), published)
	t.Logf("%s: Msgs/s %f\r\n", time.Now(), float64(published)/duration.Seconds())
	t.Logf("%s: MB/s %f\r\n", time.Now(), (float64(messageSize*published)/duration.Seconds())/1000000)
}

func TestGetMultiChannelFromPoolAndPublish(t *testing.T) {

	startTime := time.Now()
	t.Logf("%s: Benchmark Starts\r\n", startTime)

	poolErrors := 0
	published := int32(0)
	publishErrors := 0
	messageCount := 100000
	messageSize := 2500

	letter := utils.CreateMockRandomLetter("ConsumerTestQueue")

	var wg sync.WaitGroup
	for i := 0; i < messageCount; i++ {
		channelHost, err := ChannelPool.GetChannel()
		if err != nil {
			poolErrors++
			continue
		}

		wg.Add(1)
		go func() {
			wg.Done()
			err = channelHost.Channel.Publish(
				letter.Envelope.Exchange,
				letter.Envelope.RoutingKey,
				letter.Envelope.Mandatory,
				letter.Envelope.Immediate,
				amqp.Publishing{
					ContentType: letter.Envelope.ContentType,
					Body:        letter.Body,
				})

			if err != nil {
				publishErrors++
				ChannelPool.ReturnChannel(channelHost, true)
			} else {
				atomic.AddInt32(&published, 1)
				ChannelPool.ReturnChannel(channelHost, false)
			}
		}()
	}

	wg.Wait()

	duration := time.Since(startTime)

	// Needed for internal library routing delays getting out to server.
	time.Sleep(2 * time.Second)
	t.Logf("%s: Benchmark End\r\n", time.Now())
	t.Logf("%s: Time Elapsed %s\r\n", time.Now(), duration)
	t.Logf("%s: ChannelPool Errors %d\r\n", time.Now(), poolErrors)
	t.Logf("%s: Publish Errors %d\r\n", time.Now(), publishErrors)
	t.Logf("%s: Publish Actual %d\r\n", time.Now(), published)
	t.Logf("%s: Msgs/s %f\r\n", time.Now(), float64(published)/duration.Seconds())
	t.Logf("%s: MB/s %f\r\n", time.Now(), (float64(int32(messageSize)*published)/duration.Seconds())/1000000)
}

func TestCreateConnectionPool(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	connectionPool, err := pools.NewConnectionPool(Seasoning.PoolConfig, false)
	assert.NoError(t, err)

	timeStart := time.Now()

	if !connectionPool.Initialized {
		if err := connectionPool.Initialize(); err != nil {
			t.Error(err)
		}
	}

	elapsed := time.Since(timeStart)
	fmt.Printf("Created %d connection(s) finished in %s.\r\n", connectionPool.ConnectionCount(), elapsed)
	assert.Equal(t, Seasoning.PoolConfig.ConnectionPoolConfig.MaxConnectionCount, uint64(connectionPool.ConnectionCount()))

	connectionPool.FlushErrors()

	connectionPool.Shutdown()
}

func TestCreateConnectionPoolAndShutdown(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	connectionPool, err := pools.NewConnectionPool(Seasoning.PoolConfig, false)
	assert.NoError(t, err)

	timeStart := time.Now()
	if !connectionPool.Initialized {
		if err := connectionPool.Initialize(); err != nil {
			t.Error(err)
		}
	}
	elapsed := time.Since(timeStart)

	fmt.Printf("Created %d connection(s). Finished in %s.\r\n", connectionPool.ConnectionCount(), elapsed)
	assert.Equal(t, Seasoning.PoolConfig.ConnectionPoolConfig.MaxConnectionCount, uint64(connectionPool.ConnectionCount()))

	timeStart = time.Now()
	connectionPool.Shutdown()
	elapsed = time.Since(timeStart)

	fmt.Printf("Shutdown %d connection(s). Finished in %s.\r\n", connectionPool.ConnectionCount(), elapsed)
	assert.Equal(t, int64(0), connectionPool.ConnectionCount())

	connectionPool.FlushErrors()
	connectionPool.Shutdown()
}

func TestGetConnectionAfterShutdown(t *testing.T) {
	connectionPool, err := pools.NewConnectionPool(Seasoning.PoolConfig, false)
	assert.NoError(t, err)

	timeStart := time.Now()
	if !connectionPool.Initialized {
		if err := connectionPool.Initialize(); err != nil {
			t.Error(err)
		}
	}
	elapsed := time.Since(timeStart)

	fmt.Printf("Created %d connection(s). Finished in %s.\r\n", connectionPool.ConnectionCount(), elapsed)
	assert.Equal(t, Seasoning.PoolConfig.ConnectionPoolConfig.MaxConnectionCount, uint64(connectionPool.ConnectionCount()))

	connectionPool.FlushErrors()

	connectionCount := connectionPool.ConnectionCount()
	timeStart = time.Now()
	connectionPool.Shutdown()
	elapsed = time.Since(timeStart)

	fmt.Printf("Shutdown %d connection(s). Finished in %s.\r\n", connectionCount, elapsed)
	assert.Equal(t, int64(0), connectionPool.ConnectionCount())

	connectionPool.FlushErrors()

	connHost, err := connectionPool.GetConnection()
	assert.Error(t, err)
	assert.Nil(t, connHost)

	connectionPool.Shutdown()
}

func TestCreateChannelPool(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	connectionPool, err := pools.NewConnectionPool(Seasoning.PoolConfig, false)
	assert.NoError(t, err)

	channelPool, err := pools.NewChannelPool(Seasoning.PoolConfig, connectionPool, false)
	assert.NoError(t, err)

	timeStart := time.Now()
	if !channelPool.Initialized {
		if err := channelPool.Initialize(); err != nil {
			t.Error(err)
		}
	}
	elapsed := time.Since(timeStart)

	fmt.Printf("Created %d connection(s). Created %d channel(s). Finished in %s.\r\n", connectionPool.ConnectionCount(), channelPool.ChannelCount(), elapsed)
	assert.Equal(t, Seasoning.PoolConfig.ConnectionPoolConfig.MaxConnectionCount, uint64(connectionPool.ConnectionCount()))
	assert.Equal(t, Seasoning.PoolConfig.ChannelPoolConfig.MaxChannelCount, uint64(channelPool.ChannelCount()))

	connectionPool.FlushErrors()
	channelPool.FlushErrors()

	connectionPool.Shutdown()
	channelPool.Shutdown()
}

func TestCreateChannelPoolAndShutdown(t *testing.T) {

	connectionPool, err := pools.NewConnectionPool(Seasoning.PoolConfig, false)
	assert.NoError(t, err)

	channelPool, err := pools.NewChannelPool(Seasoning.PoolConfig, connectionPool, false)
	assert.NoError(t, err)

	timeStart := time.Now()
	if !channelPool.Initialized {
		if err := channelPool.Initialize(); err != nil {
			t.Error(err)
		}
	}
	elapsed := time.Since(timeStart)

	fmt.Printf("Created %d connection(s). Created %d channel(s). Finished in %s.\r\n", connectionPool.ConnectionCount(), channelPool.ChannelCount(), elapsed)
	assert.Equal(t, Seasoning.PoolConfig.ConnectionPoolConfig.MaxConnectionCount, uint64(connectionPool.ConnectionCount()))
	assert.Equal(t, Seasoning.PoolConfig.ChannelPoolConfig.MaxChannelCount, uint64(channelPool.ChannelCount()))

	connectionPool.FlushErrors()
	channelPool.FlushErrors()

	channelCount := channelPool.ChannelCount()
	timeStart = time.Now()
	channelPool.Shutdown()
	elapsed = time.Since(timeStart)

	fmt.Printf("Shutdown %d channel(s). Finished in %s.\r\n", channelCount, elapsed)
	assert.Equal(t, int64(0), channelPool.ChannelCount())

	connectionPool.FlushErrors()
	channelPool.FlushErrors()
	connectionPool.Shutdown()
}

func TestGetChannelAfterShutdown(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	connectionPool, err := pools.NewConnectionPool(Seasoning.PoolConfig, false)
	assert.NoError(t, err)

	channelPool, err := pools.NewChannelPool(Seasoning.PoolConfig, connectionPool, false)
	assert.NoError(t, err)

	timeStart := time.Now()
	if !channelPool.Initialized {
		if err := channelPool.Initialize(); err != nil {
			t.Error(err)
		}
	}
	elapsed := time.Since(timeStart)

	fmt.Printf("Created %d connection(s). Created %d channel(s). Finished in %s.\r\n", connectionPool.ConnectionCount(), channelPool.ChannelCount(), elapsed)
	assert.Equal(t, Seasoning.PoolConfig.ConnectionPoolConfig.MaxConnectionCount, uint64(connectionPool.ConnectionCount()))
	assert.Equal(t, Seasoning.PoolConfig.ChannelPoolConfig.MaxChannelCount, uint64(channelPool.ChannelCount()))

	connectionPool.FlushErrors()
	channelPool.FlushErrors()

	channelCount := channelPool.ChannelCount()
	timeStart = time.Now()
	channelPool.Shutdown()
	elapsed = time.Since(timeStart)

	fmt.Printf("Shutdown %d channel(s). Finished in %s.\r\n", channelCount, elapsed)
	assert.Equal(t, int64(0), channelPool.ChannelCount())

	connectionPool.FlushErrors()
	channelPool.FlushErrors()

	channelHost, err := channelPool.GetChannel()
	assert.Error(t, err)
	assert.Nil(t, channelHost)
}

func TestGetChannelAfterKillingConnectionPool(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	Seasoning.PoolConfig.ConnectionPoolConfig.MaxConnectionCount = 1
	Seasoning.PoolConfig.ChannelPoolConfig.MaxChannelCount = 2

	connectionPool, err := pools.NewConnectionPool(Seasoning.PoolConfig, false)
	assert.NoError(t, err)

	channelPool, err := pools.NewChannelPool(Seasoning.PoolConfig, connectionPool, false)
	assert.NoError(t, err)

	timeStart := time.Now()
	if !channelPool.Initialized {
		if err := channelPool.Initialize(); err != nil {
			t.Error(err)
		}
	}
	elapsed := time.Since(timeStart)

	fmt.Printf("Created %d connection(s). Created %d channel(s). Finished in %s.\r\n", connectionPool.ConnectionCount(), channelPool.ChannelCount(), elapsed)
	assert.Equal(t, Seasoning.PoolConfig.ConnectionPoolConfig.MaxConnectionCount, uint64(connectionPool.ConnectionCount()))
	assert.Equal(t, Seasoning.PoolConfig.ChannelPoolConfig.MaxChannelCount, uint64(channelPool.ChannelCount()))

	connectionPool.FlushErrors()
	channelPool.FlushErrors()

	connectionPool.Shutdown()

	chanHost, err := channelPool.GetChannel()
	assert.Nil(t, chanHost)
	assert.Error(t, err)

	channelPool.Shutdown()
}

func TestCreateChannelPoolSimple(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	Seasoning.PoolConfig.ConnectionPoolConfig.MaxConnectionCount = 1
	Seasoning.PoolConfig.ChannelPoolConfig.MaxChannelCount = 1
	Seasoning.PoolConfig.ChannelPoolConfig.MaxAckChannelCount = 1

	channelPool, err := pools.NewChannelPool(Seasoning.PoolConfig, nil, true)
	assert.NoError(t, err)

	channelPool.FlushErrors()

	chanHost, err := channelPool.GetChannel()
	assert.NotNil(t, chanHost)
	assert.NoError(t, err)

	channelPool.Shutdown()
}

func TestGetChannelAfterKillingChannelPool(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	Seasoning.PoolConfig.ConnectionPoolConfig.MaxConnectionCount = 1
	Seasoning.PoolConfig.ChannelPoolConfig.MaxChannelCount = 2

	channelPool, err := pools.NewChannelPool(Seasoning.PoolConfig, nil, true)
	assert.NoError(t, err)

	channelPool.FlushErrors()
	channelPool.Shutdown()

	chanHost, err := channelPool.GetChannel()
	assert.Nil(t, chanHost)
	assert.Error(t, err)
}

func TestGetConnectionDuringOutage(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	connectionPool, err := pools.NewConnectionPool(Seasoning.PoolConfig, true)
	assert.NoError(t, err)

	iterations := 0
	maxIterationCount := 10

	// Shutdown RabbitMQ server after entering loop, then start it again, to test reconnectivity.
	for iterations < maxIterationCount {
		connHost, err := connectionPool.GetConnection()
		if err != nil {
			fmt.Printf("%s: Error - GetConnectionHost: %s\r\n", time.Now(), err)
		} else {
			fmt.Printf("%s: GotConnectionHost\r\n", time.Now())
			amqpChan, err := connHost.Connection.Channel()
			if err != nil {
				fmt.Printf("%s: Error - CreateChannel: %s\r\n", time.Now(), err)
			} else {

				fmt.Printf("%s: ChannelCreated\r\n", time.Now())
				time.Sleep(250 * time.Millisecond)

				err = amqpChan.Close()
				if err != nil {
					fmt.Printf("%s: Error - CloseChannel: %s\r\n", time.Now(), err)
				} else {
					fmt.Printf("%s: ChannelClosed\r\n", time.Now())
				}
			}
		}
		connectionPool.ReturnConnection(connHost)

		iterations++
		time.Sleep(1 * time.Second)
	}

	assert.Equal(t, iterations, maxIterationCount)
	connectionPool.Shutdown()
}

func TestGetChannelDuringOutage(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	channelPool, err := pools.NewChannelPool(Seasoning.PoolConfig, nil, true)
	assert.NoError(t, err)

	iterations := 0
	maxIterationCount := 100000

	// Shutdown RabbitMQ server after entering loop, then start it again, to test reconnectivity.
	for iterations < maxIterationCount {

		chanHost, err := channelPool.GetChannel()
		if err != nil {
			fmt.Printf("%s: Error - GetChannelHost: %s\r\n", time.Now(), err)
		} else {
			fmt.Printf("%s: GotChannelHost\r\n", time.Now())

			select {
			case <-chanHost.CloseErrors():
				fmt.Printf("%s: Error - ChannelClose: %s\r\n", time.Now(), err)
			default:
				break
			}

			letter := utils.CreateMockRandomLetter("ConsumerTestQueue")
			err := chanHost.Channel.Publish(
				letter.Envelope.Exchange,
				letter.Envelope.RoutingKey,
				letter.Envelope.Mandatory, // publish doesn't appear to work when true
				letter.Envelope.Immediate, // publish doesn't appear to work when true
				amqp.Publishing{
					ContentType: letter.Envelope.ContentType,
					Body:        letter.Body,
				},
			)

			if err != nil {
				fmt.Printf("%s: Error - ChannelPublish: %s\r\n", time.Now(), err)
				channelPool.FlagChannel(chanHost.ChannelID)
				fmt.Printf("%s: ChannelFlaggedForRemoval\r\n", time.Now())
			} else {
				fmt.Printf("%s: ChannelPublishSuccess\r\n", time.Now())
			}
		}
		channelPool.ReturnChannel(chanHost, false)
		iterations++
		time.Sleep(10 * time.Millisecond)
	}

	assert.Equal(t, iterations, maxIterationCount)
	channelPool.Shutdown()
}
