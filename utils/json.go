package utils

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/houseofcat/turbocookedrabbit/models"
)

// ConvertJSONFileToConfig opens a file.json and converts to interface{}.
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
