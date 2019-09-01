package tests

import (
	"fmt"
	"testing"
	"time"

	"github.com/houseofcat/turbocookedrabbit/pools"
	"github.com/houseofcat/turbocookedrabbit/utils"
	"github.com/stretchr/testify/assert"
)

func TestCreateConnectionPool(t *testing.T) {
	seasoning, err := utils.ConvertJSONFileToConfig("seasoning.json")
	assert.Nil(t, err)

	connectionPool, poolErr := pools.NewConnectionPool(seasoning, false)
	assert.Nil(t, poolErr)

	now := time.Now()

	if !connectionPool.Initialized {
		connectionPool.Initialize()
	}

	elapsed := time.Since(now)
	fmt.Printf("Creating %d connection(s) finished in %s.\r\n", connectionPool.ConnectionCount(), elapsed)
	assert.Equal(t, seasoning.Pools.ConnectionCount, connectionPool.ConnectionCount())

	select {
	case err = <-connectionPool.Errors():
		fmt.Print(err)
	default:
		break
	}
}

func TestCreateChannelPool(t *testing.T) {

}
