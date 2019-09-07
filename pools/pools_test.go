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

	connectionPool, err := pools.NewConnectionPool(Seasoning.PoolConfig.ConnectionPoolConfig, false)
	assert.NoError(t, err)

	timeStart := time.Now()

	if !connectionPool.Initialized {
		connectionPool.Initialize()
	}

	elapsed := time.Since(timeStart)
	fmt.Printf("Created %d connection(s) finished in %s.\r\n", connectionPool.ConnectionCount(), elapsed)
	assert.Equal(t, Seasoning.PoolConfig.ConnectionPoolConfig.ConnectionCount, connectionPool.ConnectionCount())

	connectionPool.FlushErrors()

	connectionPool.Shutdown()
}

func TestCreateConnectionPoolAndShutdown(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

	connectionPool, err := pools.NewConnectionPool(Seasoning.PoolConfig.ConnectionPoolConfig, false)
	assert.NoError(t, err)

	timeStart := time.Now()
	if !connectionPool.Initialized {
		connectionPool.Initialize()
	}
	elapsed := time.Since(timeStart)

	fmt.Printf("Created %d connection(s). Finished in %s.\r\n", connectionPool.ConnectionCount(), elapsed)
	assert.Equal(t, Seasoning.PoolConfig.ConnectionPoolConfig.ConnectionCount, connectionPool.ConnectionCount())

	timeStart = time.Now()
	connectionPool.Shutdown()
	elapsed = time.Since(timeStart)

	fmt.Printf("Shutdown %d connection(s). Finished in %s.\r\n", connectionPool.ConnectionCount(), elapsed)
	assert.Equal(t, int64(0), connectionPool.ConnectionCount())

	connectionPool.FlushErrors()
	connectionPool.Shutdown()
}

func TestGetConnectionAfterShutdown(t *testing.T) {
	connectionPool, err := pools.NewConnectionPool(Seasoning.PoolConfig.ConnectionPoolConfig, false)
	assert.NoError(t, err)

	timeStart := time.Now()
	if !connectionPool.Initialized {
		connectionPool.Initialize()
	}
	elapsed := time.Since(timeStart)

	fmt.Printf("Created %d connection(s). Finished in %s.\r\n", connectionPool.ConnectionCount(), elapsed)
	assert.Equal(t, Seasoning.PoolConfig.ConnectionPoolConfig.ConnectionCount, connectionPool.ConnectionCount())

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

	connectionPool, err := pools.NewConnectionPool(Seasoning.PoolConfig.ConnectionPoolConfig, false)
	assert.NoError(t, err)

	channelPool, err := pools.NewChannelPool(Seasoning.PoolConfig, connectionPool, false)
	assert.NoError(t, err)

	timeStart := time.Now()
	if !channelPool.Initialized {
		channelPool.Initialize()
	}
	elapsed := time.Since(timeStart)

	fmt.Printf("Created %d connection(s). Created %d channel(s). Finished in %s.\r\n", connectionPool.ConnectionCount(), channelPool.ChannelCount(), elapsed)
	assert.Equal(t, Seasoning.PoolConfig.ConnectionPoolConfig.ConnectionCount, connectionPool.ConnectionCount())
	assert.Equal(t, Seasoning.PoolConfig.ChannelPoolConfig.ChannelCount, channelPool.ChannelCount())

	connectionPool.FlushErrors()
	channelPool.FlushErrors()

	connectionPool.Shutdown()
	channelPool.Shutdown()
}

func TestCreateChannelPoolAndShutdown(t *testing.T) {

	connectionPool, err := pools.NewConnectionPool(Seasoning.PoolConfig.ConnectionPoolConfig, false)
	assert.NoError(t, err)

	channelPool, err := pools.NewChannelPool(Seasoning.PoolConfig, connectionPool, false)
	assert.NoError(t, err)

	timeStart := time.Now()
	if !channelPool.Initialized {
		channelPool.Initialize()
	}
	elapsed := time.Since(timeStart)

	fmt.Printf("Created %d connection(s). Created %d channel(s). Finished in %s.\r\n", connectionPool.ConnectionCount(), channelPool.ChannelCount(), elapsed)
	assert.Equal(t, Seasoning.PoolConfig.ConnectionPoolConfig.ConnectionCount, connectionPool.ConnectionCount())
	assert.Equal(t, Seasoning.PoolConfig.ChannelPoolConfig.ChannelCount, channelPool.ChannelCount())

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

	connectionPool, err := pools.NewConnectionPool(Seasoning.PoolConfig.ConnectionPoolConfig, false)
	assert.NoError(t, err)

	channelPool, err := pools.NewChannelPool(Seasoning.PoolConfig, connectionPool, false)
	assert.NoError(t, err)

	timeStart := time.Now()
	if !channelPool.Initialized {
		channelPool.Initialize()
	}
	elapsed := time.Since(timeStart)

	fmt.Printf("Created %d connection(s). Created %d channel(s). Finished in %s.\r\n", connectionPool.ConnectionCount(), channelPool.ChannelCount(), elapsed)
	assert.Equal(t, Seasoning.PoolConfig.ConnectionPoolConfig.ConnectionCount, connectionPool.ConnectionCount())
	assert.Equal(t, Seasoning.PoolConfig.ChannelPoolConfig.ChannelCount, channelPool.ChannelCount())

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

	Seasoning.PoolConfig.ConnectionPoolConfig.ConnectionCount = 1
	Seasoning.PoolConfig.ChannelPoolConfig.ChannelCount = 2

	connectionPool, err := pools.NewConnectionPool(Seasoning.PoolConfig.ConnectionPoolConfig, false)
	assert.NoError(t, err)

	channelPool, err := pools.NewChannelPool(Seasoning.PoolConfig, connectionPool, false)
	assert.NoError(t, err)

	timeStart := time.Now()
	if !channelPool.Initialized {
		channelPool.Initialize()
	}
	elapsed := time.Since(timeStart)

	fmt.Printf("Created %d connection(s). Created %d channel(s). Finished in %s.\r\n", connectionPool.ConnectionCount(), channelPool.ChannelCount(), elapsed)
	assert.Equal(t, Seasoning.PoolConfig.ConnectionPoolConfig.ConnectionCount, connectionPool.ConnectionCount())
	assert.Equal(t, Seasoning.PoolConfig.ChannelPoolConfig.ChannelCount, channelPool.ChannelCount())

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

	Seasoning.PoolConfig.ConnectionPoolConfig.ConnectionCount = 1
	Seasoning.PoolConfig.ChannelPoolConfig.ChannelCount = 2

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

	Seasoning.PoolConfig.ConnectionPoolConfig.ConnectionCount = 1
	Seasoning.PoolConfig.ChannelPoolConfig.ChannelCount = 2

	channelPool, err := pools.NewChannelPool(Seasoning.PoolConfig, nil, true)
	assert.NoError(t, err)

	channelPool.FlushErrors()
	channelPool.Shutdown()

	chanHost, err := channelPool.GetChannel()
	assert.Nil(t, chanHost)
	assert.Error(t, err)
}
