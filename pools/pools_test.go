package pools_test

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

func TestMain(m *testing.M) { // Load Configuration On Startup
	var err error
	Seasoning, err = utils.ConvertJSONFileToConfig("testpoolseasoning.json")
	if err != nil {
		fmt.Print(err.Error())
		return
	}

	ConnectionPool, err = pools.NewConnectionPool(Seasoning.PoolConfig)
	if err != nil {
		fmt.Print(err.Error())
		return
	}

	os.Exit(m.Run())
}

func TestCreateSingleChannelAndPublish(t *testing.T) {

}
