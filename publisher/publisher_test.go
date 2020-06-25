package publisher_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/fortytw2/leaktest"

	"github.com/houseofcat/turbocookedrabbit/models"
	"github.com/houseofcat/turbocookedrabbit/pools"
	"github.com/houseofcat/turbocookedrabbit/utils"
)

var Seasoning *models.RabbitSeasoning
var ConnectionPool *pools.ConnectionPool

func TestMain(m *testing.M) { // Load Configuration On Startup
	var err error
	Seasoning, err = utils.ConvertJSONFileToConfig("testpublisherseasoning.json")
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

func TestCreatePublisherAndPublish(t *testing.T) {
	defer leaktest.Check(t)() // Fail on leaked goroutines.

}
