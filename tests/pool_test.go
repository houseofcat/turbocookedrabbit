package tests

import (
	"fmt"
	"testing"
	"time"

	"github.com/houseofcat/turbocookedrabbit/pools"
	"github.com/stretchr/testify/assert"
)

func TestCreateConnectionPool(t *testing.T) {
	Seasoning.Pools.ConnectionCount = 10
	connectionPool, err := pools.NewConnectionPool(Seasoning, false)
	assert.Nil(t, err)

	now := time.Now()

	if !connectionPool.Initialized {
		connectionPool.Initialize()
	}

	elapsed := time.Since(now)
	fmt.Printf("Created %d connection(s) finished in %s.\r\n", connectionPool.ConnectionCount(), elapsed)
	assert.Equal(t, Seasoning.Pools.ConnectionCount, connectionPool.ConnectionCount())

	// Flush Errors
	select {
	case err = <-connectionPool.Errors():
		fmt.Print(err)
	default:
		break
	}
}

func TestCreateConnectionPoolAndShutdown(t *testing.T) {
	Seasoning.Pools.ConnectionCount = 12
	connectionPool, err := pools.NewConnectionPool(Seasoning, false)
	assert.Nil(t, err)

	now := time.Now()
	if !connectionPool.Initialized {
		connectionPool.Initialize()
	}
	elapsed := time.Since(now)

	fmt.Printf("Created %d connection(s). Finished in %s.\r\n", connectionPool.ConnectionCount(), elapsed)
	assert.Equal(t, Seasoning.Pools.ConnectionCount, connectionPool.ConnectionCount())

	// Flush Errors
	select {
	case err = <-connectionPool.Errors():
		fmt.Print(err)
	default:
		break
	}

	now = time.Now()
	connectionPool.Shutdown()
	elapsed = time.Since(now)

	fmt.Printf("Shutdown %d connection(s). Finished in %s.\r\n", connectionPool.ConnectionCount(), elapsed)
	assert.Equal(t, int64(0), connectionPool.ConnectionCount())

	// Flush Errors
	select {
	case err = <-connectionPool.Errors():
		fmt.Print(err)
	default:
		break
	}
}

func TestGetConnectionAfterShutdown(t *testing.T) {
	Seasoning.Pools.ConnectionCount = 24
	connectionPool, err := pools.NewConnectionPool(Seasoning, false)
	assert.Nil(t, err)

	now := time.Now()
	if !connectionPool.Initialized {
		connectionPool.Initialize()
	}
	elapsed := time.Since(now)

	fmt.Printf("Created %d connection(s). Finished in %s.\r\n", connectionPool.ConnectionCount(), elapsed)
	assert.Equal(t, Seasoning.Pools.ConnectionCount, connectionPool.ConnectionCount())

	// Flush Errors
	select {
	case err = <-connectionPool.Errors():
		fmt.Print(err)
	default:
		break
	}

	connectionCount := connectionPool.ConnectionCount()
	now = time.Now()
	connectionPool.Shutdown()
	elapsed = time.Since(now)

	fmt.Printf("Shutdown %d connection(s). Finished in %s.\r\n", connectionCount, elapsed)
	assert.Equal(t, int64(0), connectionPool.ConnectionCount())

	// Flush Errors
	select {
	case err = <-connectionPool.Errors():
		fmt.Print(err)
	default:
		break
	}

	connHost, err := connectionPool.GetConnection()
	assert.Error(t, err)
	assert.Nil(t, connHost)
}

func TestCreateChannelPool(t *testing.T) {
	Seasoning.Pools.ConnectionCount = 10
	connectionPool, err := pools.NewConnectionPool(Seasoning, false)
	assert.Nil(t, err)

	channelPool, err := pools.NewChannelPool(Seasoning, connectionPool, false)
	assert.Nil(t, err)

	now := time.Now()
	if !channelPool.Initialized {
		channelPool.Initialize()
	}
	elapsed := time.Since(now)

	fmt.Printf("Created %d connection(s). Created %d channel(s). Finished in %s.\r\n", connectionPool.ConnectionCount(), channelPool.ChannelCount(), elapsed)
	assert.Equal(t, Seasoning.Pools.ConnectionCount, connectionPool.ConnectionCount())
	assert.Equal(t, Seasoning.Pools.ChannelCount, channelPool.ChannelCount())

	// Flush Errors
	select {
	case conErr := <-connectionPool.Errors():
		fmt.Print(conErr)
	case chanErr := <-channelPool.Errors():
		fmt.Print(chanErr)
	default:
		break
	}
}

func TestCreateChannelPoolAndShutdown(t *testing.T) {
	Seasoning.Pools.ConnectionCount = 10
	connectionPool, err := pools.NewConnectionPool(Seasoning, false)
	assert.Nil(t, err)

	channelPool, err := pools.NewChannelPool(Seasoning, connectionPool, false)
	assert.Nil(t, err)

	now := time.Now()
	if !channelPool.Initialized {
		channelPool.Initialize()
	}
	elapsed := time.Since(now)

	fmt.Printf("Created %d connection(s). Created %d channel(s). Finished in %s.\r\n", connectionPool.ConnectionCount(), channelPool.ChannelCount(), elapsed)
	assert.Equal(t, Seasoning.Pools.ConnectionCount, connectionPool.ConnectionCount())
	assert.Equal(t, Seasoning.Pools.ChannelCount, channelPool.ChannelCount())

	// Flush Errors
	select {
	case conErr := <-connectionPool.Errors():
		fmt.Print(conErr)
	case chanErr := <-channelPool.Errors():
		fmt.Print(chanErr)
	default:
		break
	}

	channelCount := channelPool.ChannelCount()
	now = time.Now()
	channelPool.Shutdown()
	elapsed = time.Since(now)

	fmt.Printf("Shutdown %d channel(s). Finished in %s.\r\n", channelCount, elapsed)
	assert.Equal(t, int64(0), channelPool.ChannelCount())

	// Flush Errors
	select {
	case conErr := <-connectionPool.Errors():
		fmt.Print(conErr)
	case chanErr := <-channelPool.Errors():
		fmt.Print(chanErr)
	default:
		break
	}
}

func TestGetChannelAfterShutdown(t *testing.T) {
	Seasoning.Pools.ConnectionCount = 10
	connectionPool, err := pools.NewConnectionPool(Seasoning, false)
	assert.Nil(t, err)

	channelPool, err := pools.NewChannelPool(Seasoning, connectionPool, false)
	assert.Nil(t, err)

	now := time.Now()
	if !channelPool.Initialized {
		channelPool.Initialize()
	}
	elapsed := time.Since(now)

	fmt.Printf("Created %d connection(s). Created %d channel(s). Finished in %s.\r\n", connectionPool.ConnectionCount(), channelPool.ChannelCount(), elapsed)
	assert.Equal(t, Seasoning.Pools.ConnectionCount, connectionPool.ConnectionCount())
	assert.Equal(t, Seasoning.Pools.ChannelCount, channelPool.ChannelCount())

	// Flush Errors
	select {
	case conErr := <-connectionPool.Errors():
		fmt.Print(conErr)
	case chanErr := <-channelPool.Errors():
		fmt.Print(chanErr)
	default:
		break
	}

	channelCount := channelPool.ChannelCount()
	now = time.Now()
	channelPool.Shutdown()
	elapsed = time.Since(now)

	fmt.Printf("Shutdown %d channel(s). Finished in %s.\r\n", channelCount, elapsed)
	assert.Equal(t, int64(0), channelPool.ChannelCount())

	// Flush Errors
	select {
	case conErr := <-connectionPool.Errors():
		fmt.Print(conErr)
	case chanErr := <-channelPool.Errors():
		fmt.Print(chanErr)
	default:
		break
	}

	channelHost, err := channelPool.GetChannel()
	assert.Error(t, err)
	assert.Nil(t, channelHost)
}

func TestGetChannelAfterKillingConnection(t *testing.T) {
	Seasoning.Pools.ConnectionCount = 1
	Seasoning.Pools.ChannelCount = 2
	connectionPool, err := pools.NewConnectionPool(Seasoning, false)
	assert.Nil(t, err)

	channelPool, err := pools.NewChannelPool(Seasoning, connectionPool, false)
	assert.Nil(t, err)

	now := time.Now()
	if !channelPool.Initialized {
		channelPool.Initialize()
	}
	elapsed := time.Since(now)

	fmt.Printf("Created %d connection(s). Created %d channel(s). Finished in %s.\r\n", connectionPool.ConnectionCount(), channelPool.ChannelCount(), elapsed)
	assert.Equal(t, Seasoning.Pools.ConnectionCount, connectionPool.ConnectionCount())
	assert.Equal(t, Seasoning.Pools.ChannelCount, channelPool.ChannelCount())

	// Flush Errors
	select {
	case conErr := <-connectionPool.Errors():
		fmt.Print(conErr)
	case chanErr := <-channelPool.Errors():
		fmt.Print(chanErr)
	default:
		break
	}

	// Breakpoint here: Kill all the connections server side before proceeding.
	chanHost, err := channelPool.GetChannel()
	assert.NotNil(t, chanHost)
	assert.Nil(t, err)

	err = chanHost.Channel.Close()
	assert.Nil(t, err)
}
