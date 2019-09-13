package pools

import (
	"crypto/tls"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Workiva/go-datastructures/queue"

	"github.com/houseofcat/turbocookedrabbit/models"
	"github.com/houseofcat/turbocookedrabbit/utils"
)

// ConnectionPool houses the pool of RabbitMQ connections.
type ConnectionPool struct {
	Config               models.ConnectionPoolConfig
	Initialized          bool
	uri                  string
	enableTLS            bool
	tlsConfig            *tls.Config
	errors               chan error
	connections          *queue.Queue
	maxConnections       uint64
	connectionID         uint64
	poolLock             *sync.Mutex
	connectionLock       int32
	flaggedConnections   map[uint64]bool
	sleepOnErrorInterval time.Duration
}

// NewConnectionPool creates hosting structure for the ConnectionPool.
// Needs to be Initialize() afterwards.
func NewConnectionPool(
	config *models.ConnectionPoolConfig,
	initializeNow bool) (*ConnectionPool, error) {

	var tlsConfig *tls.Config
	var err error

	if config.EnableTLS {
		if config.TLSConfig == nil {
			return nil, errors.New("can't enable TLS when TLS config is nil")
		}

		tlsConfig, err = utils.CreateTLSConfig(
			config.TLSConfig.PEMCertLocation,
			config.TLSConfig.LocalCertLocation)
		if err != nil {
			return nil, err
		}
	}

	if config.ErrorBuffer == 0 {
		return nil, errors.New("can't create a ConnectionPool when the ErrorBuffer value is 0")
	}

	cp := &ConnectionPool{
		Config:               *config,
		uri:                  config.URI,
		enableTLS:            config.EnableTLS,
		tlsConfig:            tlsConfig,
		errors:               make(chan error, config.ErrorBuffer),
		maxConnections:       config.ConnectionCount,
		connections:          queue.New(int64(config.ConnectionCount)), // possible overflow error
		poolLock:             &sync.Mutex{},
		flaggedConnections:   make(map[uint64]bool),
		sleepOnErrorInterval: time.Duration(config.SleepOnErrorInterval) * time.Millisecond,
	}

	if initializeNow {
		cp.Initialize()
	}

	return cp, nil
}

// Initialize creates the ConnectionPool based on the config details.
// Blocks on network/communication issues unless overridden by config.
func (cp *ConnectionPool) Initialize() error {
	cp.poolLock.Lock()
	defer cp.poolLock.Unlock()

	if !cp.Initialized {
		ok := false

		if cp.Config.EnableTLS {
			ok = cp.initializeWithTLS()
		} else {
			ok = cp.initialize()
		}

		if ok {
			cp.Initialized = true
		} else {
			return errors.New("initialization failed during creating connections")
		}
	}

	return nil
}

func (cp *ConnectionPool) initialize() bool {

	for i := uint64(0); i < cp.maxConnections; i++ {
		connectionHost, err := cp.createConnectionHost(cp.connectionID)
		if err != nil {
			return false
		}

		cp.connectionID++
		cp.connections.Put(connectionHost)
	}

	return true
}

func (cp *ConnectionPool) initializeWithTLS() bool {

	for i := uint64(0); i < cp.maxConnections; i++ {
		connectionHost, err := cp.createConnectionHostWithTLS(cp.connectionID)
		if err != nil {
			return false
		}

		cp.connectionID++
		cp.connections.Put(connectionHost)
	}

	return true
}

// CreateConnectionHost creates the Connection with RabbitMQ server.
func (cp *ConnectionPool) createConnectionHost(connectionID uint64) (*models.ConnectionHost, error) {

	return models.NewConnectionHost(cp.uri, connectionID)
}

// CreateConnectionHostWithTLS creates the Connection with RabbitMQ server.
func (cp *ConnectionPool) createConnectionHostWithTLS(connectionID uint64) (*models.ConnectionHost, error) {
	if cp.tlsConfig == nil {
		return nil, errors.New("tls enabled but tlsConfig has not been created")
	}

	return models.NewConnectionHostWithTLS(cp.uri, connectionID, cp.tlsConfig)
}

// Errors yields all the internal errs for creating connections.
func (cp *ConnectionPool) Errors() <-chan error {
	return cp.errors
}

// GetConnection gets a connection based on whats in the ConnectionPool (blocking under bad network conditions).
// Outages/transient network outages block until success connecting.
// Uses the SleepOnErrorInterval to pause between retries.
func (cp *ConnectionPool) GetConnection() (*models.ConnectionHost, error) {
	if atomic.LoadInt32(&cp.connectionLock) > 0 {
		return nil, errors.New("can't get connection - connection pool is being shutdown")
	}

	if !cp.Initialized {
		return nil, errors.New("can't get connection - connection pool has not been initialized")
	}

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

	notifiedClosed := false
	select {
	case <-connectionHost.CloseErrors():
		notifiedClosed = true
	default:
		break
	}

	// Makes debugging easier
	connectionClosed := connectionHost.Connection.IsClosed()
	connectionFlagged := cp.IsConnectionFlagged(connectionHost.ConnectionID)

	// Between these three states we do our best to determine that a connection is dead in the various
	// lifecycles.
	if notifiedClosed || connectionClosed || connectionFlagged {

		cp.FlagConnection(connectionHost.ConnectionID)

		var err error
		replacementConnectionID := connectionHost.ConnectionID
		connectionHost = nil

		// Do not leave without a good Connection.
		for connectionHost == nil {

			if cp.enableTLS { // Replacement Connection
				connectionHost, err = cp.createConnectionHostWithTLS(replacementConnectionID)
				if err != nil {
					continue
				}
			} else { // Replacement Connection
				connectionHost, err = cp.createConnectionHost(replacementConnectionID)
				if err != nil {
					continue
				}
			}

			if cp.sleepOnErrorInterval > 0 {
				time.Sleep(cp.sleepOnErrorInterval)
			}
		}

		cp.UnflagConnection(replacementConnectionID)
	}

	// Puts the connection back in the queue while also returning a pointer to the caller.
	// This creates a Round Robin on Connections and their resources.
	cp.connections.Put(connectionHost)

	return connectionHost, nil
}

// ConnectionCount flags that connection as usable in the future. Careful, locking call.
func (cp *ConnectionPool) ConnectionCount() int64 {
	return cp.connections.Len() // Locking
}

// UnflagConnection flags that connection as usable in the future.
func (cp *ConnectionPool) UnflagConnection(connectionID uint64) {
	cp.poolLock.Lock()
	defer cp.poolLock.Unlock()
	cp.flaggedConnections[connectionID] = false
}

// FlagConnection flags that connection as non-usable in the future.
func (cp *ConnectionPool) FlagConnection(connectionID uint64) {
	cp.poolLock.Lock()
	defer cp.poolLock.Unlock()
	cp.flaggedConnections[connectionID] = true
}

// IsConnectionFlagged checks to see if the connection has been flagged for removal.
func (cp *ConnectionPool) IsConnectionFlagged(connectionID uint64) bool {
	cp.poolLock.Lock()
	defer cp.poolLock.Unlock()
	if flagged, ok := cp.flaggedConnections[connectionID]; ok {
		return flagged
	}

	return false
}

// Shutdown closes all connections in the ConnectionPool and resets the Pool to pre-initialized state.
func (cp *ConnectionPool) Shutdown() {
	cp.poolLock.Lock()
	defer cp.poolLock.Unlock()

	// Create connection lock (> 0)
	atomic.AddInt32(&cp.connectionLock, 1)

	if cp.Initialized {
		cp.shutdownConnections()

		cp.connections = queue.New(int64(cp.maxConnections))
		cp.flaggedConnections = make(map[uint64]bool)
		cp.connectionID = 0
		cp.Initialized = false
	}

	// Release connection lock (0)
	atomic.StoreInt32(&cp.connectionLock, 0)
}

// ShutdownConnections actually closes all the connections.
func (cp *ConnectionPool) shutdownConnections() {
	for !cp.connections.Empty() {
		items, _ := cp.connections.Get(cp.connections.Len())

		for _, item := range items {
			connectionHost := item.(*models.ConnectionHost)
			if !connectionHost.Connection.IsClosed() {
				connectionHost.Connection.Close()
			}
		}
	}
}

// FlushErrors empties all current errors in the error channel.
func (cp *ConnectionPool) FlushErrors() {

FlushLoop:
	for {
		select {
		case <-cp.Errors():
		default:
			break FlushLoop
		}
	}
}
