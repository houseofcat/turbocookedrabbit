package tests

import (
	"os"
	"testing"

	"github.com/houseofcat/turbocookedrabbit/models"
	"github.com/houseofcat/turbocookedrabbit/utils"
)

var Seasoning *models.RabbitSeasoning

func TestMain(m *testing.M) {
	var err error
	Seasoning, err = utils.ConvertJSONFileToConfig("seasoning.json")
	if err != nil {
		return
	}
	os.Exit(m.Run())
}
