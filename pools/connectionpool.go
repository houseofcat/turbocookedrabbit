package pools

import (
	"time"

	"github.com/houseofcat/turbocookedrabbit/models"
	"github.com/streadway/amqp"
)

// ConnectionPool houses the pool of RabbitMQ connections.
type ConnectionPool struct {
	Config      *models.RabbitSeasoning
	Connections []models.ConnectionHost
	Errors      chan error
}

// NewConnectionPool creates hosting structure for the ConnectionPool.
func NewConnectionPool(config *models.RabbitSeasoning) *ConnectionPool {

	return &ConnectionPool{
		Config: config,
		Errors: make(chan error, 1),
	}
}

// Initialize creates the ConnectionPool based on the config details.
func (cp *ConnectionPool) Initialize() {
	for {
		amqpConn, err := amqp.Dial(cp.Config.Pools.URI)
		if err != nil {
			cp.Errors <- err
			time.Sleep(1 * time.Second)
			continue
		}

		connectionHost := &models.ConnectionHost{
			Connection: amqpConn,
		}
	}
}
