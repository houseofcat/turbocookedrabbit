package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/houseofcat/turbocookedrabbit/utils"
)

func TestReadConfig(t *testing.T) {
	filePath := "seasoning.json"

	assert.FileExists(t, filePath)

	config, err := utils.ConvertJSONFileToConfig(filePath)

	assert.Nil(t, err)
	assert.NotEmpty(config.ConnectionFactory.URI, "")
}
