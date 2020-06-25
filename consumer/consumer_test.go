package consumer_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/houseofcat/turbocookedrabbit/models"
	"github.com/houseofcat/turbocookedrabbit/pools"
	"github.com/houseofcat/turbocookedrabbit/utils"
)

var Seasoning *models.RabbitSeasoning
var ConnectionPool *pools.ConnectionPool

func TestMain(m *testing.M) {

	var err error
	Seasoning, err = utils.ConvertJSONFileToConfig("testconsumerseasoning.json") // Load Configuration On Startup
	if err != nil {
		return
	}
	ConnectionPool, err = pools.NewConnectionPool(Seasoning.PoolConfig)
	if err != nil {
		fmt.Print(err.Error())
		return
	}

	os.Exit(m.Run())
}

func TestCreateConsumer(t *testing.T) {

}
