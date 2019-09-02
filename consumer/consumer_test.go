package consumer_test

import (
	"os"
	"testing"

	"github.com/houseofcat/turbocookedrabbit/models"
	"github.com/houseofcat/turbocookedrabbit/utils"
)

var Seasoning *models.RabbitSeasoning

func TestMain(m *testing.M) { // Load Configuration On Startup
	var err error
	Seasoning, err = utils.ConvertJSONFileToConfig("seasoning.json")
	if err != nil {
		return
	}
	os.Exit(m.Run())
}

func TestConsumer(t *testing.T) {

}
