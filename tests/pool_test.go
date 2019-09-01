package tests

import (
	"fmt"
	"testing"
	"time"

	"github.com/houseofcat/turbocookedrabbit/pools"
	"github.com/stretchr/testify/assert"
)

func TestCreateConnectionPool(t *testing.T) {
	Seasoning.Pools.ConnectionCount += 10
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
	Seasoning.Pools.ConnectionCount += 12
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
	Seasoning.Pools.ConnectionCount += 24
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

	connHost, err := connectionPool.GetConnection()
	assert.Error(t, err)
	assert.Nil(t, connHost)
}
