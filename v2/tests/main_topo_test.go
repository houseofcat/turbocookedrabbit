package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/houseofcat/turbocookedrabbit/v2/pkg/tcr"
	"github.com/streadway/amqp"
	"github.com/stretchr/testify/assert"
)

func TestReadTopologyConfig(t *testing.T) {
	fileNamePath := "testtopology.json"

	assert.FileExists(t, fileNamePath)

	config, err := tcr.ConvertJSONFileToTopologyConfig(fileNamePath)

	assert.Nil(t, err)
	assert.NotEqual(t, 0, len(config.Exchanges))
	assert.NotEqual(t, 0, len(config.Queues))
	assert.NotEqual(t, 0, len(config.QueueBindings))
	assert.NotEqual(t, 0, len(config.ExchangeBindings))
}

func TestCreateTopologyFromTopologyConfig(t *testing.T) {

	fileNamePath := "testtopology.json"
	assert.FileExists(t, fileNamePath)

	topologyConfig, err := tcr.ConvertJSONFileToTopologyConfig(fileNamePath)
	assert.NoError(t, err)

	connectionPool, err := tcr.NewConnectionPool(Seasoning.PoolConfig)
	assert.NoError(t, err)

	topologer := tcr.NewTopologer(connectionPool)

	err = topologer.BuildTopology(topologyConfig, true)
	assert.NoError(t, err)
}

func TestCreateMultipleTopologyFromTopologyConfig(t *testing.T) {

	connectionPool, err := tcr.NewConnectionPool(Seasoning.PoolConfig)
	assert.NoError(t, err)

	topologer := tcr.NewTopologer(connectionPool)

	topologyConfigs := make([]string, 0)
	configRoot := "./"
	err = filepath.Walk(configRoot, func(path string, info os.FileInfo, err error) error {
		if strings.Contains(path, "topology") {
			topologyConfigs = append(topologyConfigs, path)
		}
		return nil
	})
	assert.NoError(t, err)

	for _, filePath := range topologyConfigs {
		topologyConfig, err := tcr.ConvertJSONFileToTopologyConfig(filePath)
		if err != nil {
			assert.NoError(t, err)
		} else {
			err = topologer.BuildTopology(topologyConfig, false)
			assert.NoError(t, err)
		}
	}
}

func TestUnbindQueue(t *testing.T) {

	connectionPool, err := tcr.NewConnectionPool(Seasoning.PoolConfig)
	assert.NoError(t, err)

	topologer := tcr.NewTopologer(connectionPool)

	err = topologer.UnbindQueue("QueueAttachedToExch01", "RoutingKey1", "MyTestExchange.Child01", nil)
	assert.NoError(t, err)
}

func TestCreateQuorumQueue(t *testing.T) {

	connectionPool, err := tcr.NewConnectionPool(Seasoning.PoolConfig)
	assert.NoError(t, err)

	topologer := tcr.NewTopologer(connectionPool)

	err = topologer.CreateQueue("TcrTestQuorumQueue", false, true, false, false, false, amqp.Table{
		"x-queue-type": tcr.QueueTypeQuorum,
	})
	assert.NoError(t, err)

	_, err = topologer.QueueDelete("TcrTestQuorumQueue", false, false, false)
	assert.NoError(t, err)
}
