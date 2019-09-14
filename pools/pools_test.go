package pools_test

import (
	"fmt"
	"os"
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

func TestMain(m *testing.M) { // Load Configuration On Startup
	var err error
	Seasoning, err = utils.ConvertJSONFileToConfig("testpoolseasoning.json")
	if err != nil {
		return
	}
	os.Exit(m.Run())
}

func TestCreateConnectionPool(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	connectionPool, err := pools.NewConnectionPool(Seasoning.PoolConfig, false)
	assert.NoError(t, err)

	timeStart := time.Now()

	if !connectionPool.Initialized {
		connectionPool.Initialize()
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
		connectionPool.Initialize()
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
		connectionPool.Initialize()
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
		channelPool.Initialize()
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
		channelPool.Initialize()
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
		channelPool.Initialize()
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
		channelPool.Initialize()
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

			letter := utils.CreateLetter("", "ConsumerTestQueue", nil)
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
		channelPool.ReturnChannel(chanHost)
		iterations++
		time.Sleep(10 * time.Millisecond)
	}

	assert.Equal(t, iterations, maxIterationCount)
	channelPool.Shutdown()
}
