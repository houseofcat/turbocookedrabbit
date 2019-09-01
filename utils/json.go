package utils

import (
	"io/ioutil"
	"os"

	jsoniter "github.com/json-iterator/go"

	"github.com/houseofcat/turbocookedrabbit/models"
)

// ConvertJSONFileToConfig opens a file.json and converts to interface{}.
func ConvertJSONFileToConfig(filename string) (*models.RabbitSeasoning, error) {
	jsonFile, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)

	var json = jsoniter.ConfigFastest
	config := &models.RabbitSeasoning{}
	deserialErr := json.Unmarshal(byteValue, &config)

	return config, deserialErr
}
