package main_test

import (
	"testing"

	"github.com/houseofcat/turbocookedrabbit/v2/pkg/tcr"
	"github.com/stretchr/testify/assert"
)

func TestReadConfig(t *testing.T) {
	fileNamePath := "testseasoning.json"

	assert.FileExists(t, fileNamePath)

	config, err := tcr.ConvertJSONFileToConfig(fileNamePath)

	assert.Nil(t, err)
	assert.NotEqual(t, "", config.PoolConfig.URI, "RabbitMQ URI should not be blank.")
}
