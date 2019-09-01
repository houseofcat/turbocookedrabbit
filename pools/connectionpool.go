package pools

import (
	"crypto/tls"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Workiva/go-datastructures/queue"
	"github.com/streadway/amqp"

	"github.com/houseofcat/turbocookedrabbit/models"
	"github.com/houseofcat/turbocookedrabbit/utils"
)

// ConnectionPool houses the pool of RabbitMQ connections.
type ConnectionPool struct {
	Config             *models.RabbitSeasoning
	tlsConfig          *tls.Config
	errors             chan error
	connections        *queue.Queue
	connectionCount    uint64
	flagLock           *sync.Mutex
	flaggedConnections map[uint64]bool
}

// New creates hosting structure for the ConnectionPool.
func New(seasoning *models.RabbitSeasoning) (*ConnectionPool, error) {

	var tlsConfig *tls.Config
	var err error

	if seasoning.TLSConfig.EnableTLS {
		tlsConfig, err = utils.CreateTLSConfig(
			seasoning.TLSConfig.PEMCertLocation,
			seasoning.TLSConfig.LocalCertLocation)
		if err != nil {
			return nil, err
		}
	}

	return &ConnectionPool{
		Config:      seasoning,
		tlsConfig:   tlsConfig,
		errors:      make(chan error, 1),
		connections: queue.New(seasoning.Pools.ConnectionCount),
	}, nil
}

// Initialize creates the ConnectionPool based on the config details.
// Blocks on network/communication issues unless overridden by config.
func (cp *ConnectionPool) Initialize() {
	if cp.Config.TLSConfig.EnableTLS {
		cp.initializeWithTLS()
	} else {
		cp.initialize()
	}
}

func (cp *ConnectionPool) initialize() {
	for i := int64(0); i < atomic.LoadInt64(&cp.Config.Pools.ConnectionCount); i++ {
		connectionHost, err := cp.createConnectionHost()
		if err != nil {
			if cp.Config.Pools.BreakOnError {
				break
			}

			cp.errors <- err
			time.Sleep(1 * time.Second)
			continue
		}

		cp.connections.Put(connectionHost)
	}
}

func (cp *ConnectionPool) initializeWithTLS() {
	for i := int64(0); i < atomic.LoadInt64(&cp.Config.Pools.ConnectionCount); i++ {
		connectionHost, err := cp.createConnectionHostWithTLS()
		if err != nil {
			if cp.Config.Pools.BreakOnError {
				break
			}

			cp.errors <- err
			time.Sleep(1 * time.Second)
			continue
		}

		cp.connections.Put(connectionHost)
	}
}

// CreateConnectionHost creates the Connection with RabbitMQ server.
func (cp *ConnectionPool) createConnectionHost() (*models.ConnectionHost, error) {
	var amqpConn *amqp.Connection
	var innerError error
	retryCount := atomic.LoadInt32(&cp.Config.Pools.ConnectionRetryCount)

	for i := retryCount; i > 0; i-- {
		amqpConn, innerError = amqp.Dial(cp.Config.Pools.URI)
		if innerError != nil {
			if cp.Config.Pools.BreakOnError {
				break
			}

			cp.errors <- innerError
			time.Sleep(1 * time.Second)
			continue
		}
	}

	if amqpConn == nil {
		return nil, errors.New("opening connections retries exhausted")
	}

	connectionHost := &models.ConnectionHost{
		Connection:   amqpConn,
		ConnectionID: atomic.LoadUint64(&cp.connectionCount),
	}

	atomic.AddUint64(&cp.connectionCount, 1)

	return connectionHost, nil
}

// CreateConnectionHostWithTLS creates the Connection with RabbitMQ server.
func (cp *ConnectionPool) createConnectionHostWithTLS() (*models.ConnectionHost, error) {
	if cp.tlsConfig == nil {
		return nil, errors.New("tls enabled but tlsConfig has not been created")
	}

	var amqpConn *amqp.Connection
	var err error
	retryCount := atomic.LoadInt32(&cp.Config.Pools.ConnectionRetryCount)

	for i := retryCount; i > 0; i-- {
		amqpConn, err = amqp.DialTLS("amqps://"+cp.Config.TLSConfig.CertServerName, cp.tlsConfig)
		if err != nil {
			if cp.Config.Pools.BreakOnError {
				break
			}

			cp.errors <- err
			time.Sleep(1 * time.Second)
			continue
		}
	}

	if amqpConn == nil {
		return nil, errors.New("opening connections retries exhausted")
	}

	connectionHost := &models.ConnectionHost{
		Connection:   amqpConn,
		ConnectionID: atomic.LoadUint64(&cp.connectionCount),
	}

	atomic.AddUint64(&cp.connectionCount, 1)

	return connectionHost, nil
}

// Errors yields all the internal errs for creating connections.
func (cp *ConnectionPool) Errors() <-chan error {
	return cp.errors
}

// GetConnection gets a connection based on whats available in ConnectionPool queue.
func (cp *ConnectionPool) GetConnection() (*models.ConnectionHost, error) {

	// Pull from the queue.
	// Pauses here if the queue is empty.
	structs, err := cp.connections.Get(1)
	if err != nil {
		return nil, err
	}

	connectionHost, ok := structs[0].(*models.ConnectionHost)
	if !ok {
		return nil, errors.New("invalid struct type found in ConnectionPool queue")
	}

	if cp.IsConnectionFlagged(connectionHost.ConnectionID) || connectionHost.Connection.IsClosed() {

		var newHost *models.ConnectionHost
		var err error

		if cp.Config.TLSConfig.EnableTLS {
			newHost, err = cp.createConnectionHostWithTLS()
			if err != nil {
				return nil, err
			}
		} else {
			newHost, err = cp.createConnectionHost()
			if err != nil {
				return nil, err
			}
		}

		connectionHost = newHost
	}

	// Puts the connection back in the queue while also returning a pointer to the caller.
	// This creates a Round Robin on Connections and their resources.
	cp.connections.Put(connectionHost)

	return connectionHost, nil
}

// UnflagConnection flags that connection as usable in the future.
func (cp *ConnectionPool) UnflagConnection(connectionID uint64) {
	cp.flagLock.Lock()
	defer cp.flagLock.Unlock()
	cp.flaggedConnections[connectionID] = false
}

// FlagConnection flags that connection as non-usable in the future.
func (cp *ConnectionPool) FlagConnection(connectionID uint64) {
	cp.flagLock.Lock()
	defer cp.flagLock.Unlock()
	cp.flaggedConnections[connectionID] = true
}

// IsConnectionFlagged checks to see if the connection has been flagged for removal.
func (cp *ConnectionPool) IsConnectionFlagged(connectionID uint64) bool {
	cp.flagLock.Lock()
	defer cp.flagLock.Unlock()
	_, ok := cp.flaggedConnections[connectionID]
	return ok
}
