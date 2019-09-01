package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/houseofcat/turbocookedrabbit/utils"
)

func TestReadConfig(t *testing.T) {
	fileNamePath := "seasoning.json"

	assert.FileExists(t, fileNamePath)

	config, err := utils.ConvertJSONFileToConfig(fileNamePath)

	assert.Nil(t, err)
	assert.NotEqual(t, "", config.Pools.URI, "RabbitMQ URI should not be blank.")
}
