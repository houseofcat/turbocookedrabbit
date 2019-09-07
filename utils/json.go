package utils

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/houseofcat/turbocookedrabbit/models"
)

// ConvertJSONFileToConfig opens a file.json and converts to RabbitSeasoning.
func ConvertJSONFileToConfig(fileNamePath string) (*models.RabbitSeasoning, error) {

	jsonFile, err := os.Open(fileNamePath)
	if err != nil {
		return nil, err
	}

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}

	config := &models.RabbitSeasoning{}
	err = json.Unmarshal(byteValue, config)

	return config, err
}

// ConvertJSONFileToTopologyConfig opens a file.json and converts to Topology.
func ConvertJSONFileToTopologyConfig(fileNamePath string) (*models.TopologyConfig, error) {

	jsonFile, err := os.Open(fileNamePath)
	if err != nil {
		return nil, err
	}

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}

	config := &models.TopologyConfig{}
	err = json.Unmarshal(byteValue, config)

	return config, err
}

// ReadJSONFileToInterface opens a file.json and converts to interface{}.
func ReadJSONFileToInterface(fileNamePath string) (interface{}, error) {

	jsonFile, err := os.Open(fileNamePath)
	if err != nil {
		return nil, err
	}

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}

	var data interface{}
	err = json.Unmarshal(byteValue, data)

	return &data, err
}
