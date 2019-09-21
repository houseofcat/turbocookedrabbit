package pools

import (
	"crypto/tls"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Workiva/go-datastructures/queue"

	"github.com/houseofcat/turbocookedrabbit/models"
	"github.com/houseofcat/turbocookedrabbit/utils"
)

// ConnectionPool houses the pool of RabbitMQ connections.
type ConnectionPool struct {
	config                     models.PoolConfig
	Initialized                bool
	connectionName             string
	uri                        string
	enableTLS                  bool
	tlsConfig                  *tls.Config
	errors                     chan error
	heartbeat                  time.Duration
	connectionTimeout          time.Duration
	connections                *queue.Queue
	maxConnections             uint64
	maxChannelPerConnection    uint64
	maxAckChannelPerConnection uint64
	connectionID               uint64
	poolLock                   *sync.Mutex
	poolRWLock                 *sync.RWMutex
	connectionLock             int32
	flaggedConnections         map[uint64]bool
	sleepOnErrorInterval       time.Duration
}

// NewConnectionPool creates hosting structure for the ConnectionPool.
// Needs to be Initialize() afterwards.
func NewConnectionPool(
	config *models.PoolConfig,
	initializeNow bool) (*ConnectionPool, error) {

	var tlsConfig *tls.Config
	var err error

	if config.ConnectionPoolConfig.Heartbeat == 0 || config.ConnectionPoolConfig.ConnectionTimeout == 0 {
		return nil, errors.New("connectionpool heartbeat or connectiontimeout can't be 0")
	}

	if config.ConnectionPoolConfig.MaxConnectionCount == 0 {
		return nil, errors.New("connectionpool maxconnectioncount can't be 0")
	}

	if config.ConnectionPoolConfig.EnableTLS {
		if config.ConnectionPoolConfig.TLSConfig == nil {
			return nil, errors.New("can't enable TLS when TLS config is nil")
		}

		tlsConfig, err = utils.CreateTLSConfig(
			config.ConnectionPoolConfig.TLSConfig.PEMCertLocation,
			config.ConnectionPoolConfig.TLSConfig.LocalCertLocation)
		if err != nil {
			return nil, err
		}
	}

	if config.ConnectionPoolConfig.ErrorBuffer == 0 {
		return nil, errors.New("can't create a ConnectionPool when the ErrorBuffer value is 0")
	}

	maxChannelPerConnection := uint64(1)
	if config.ConnectionPoolConfig.MaxConnectionCount == 1 {
		maxChannelPerConnection = config.ChannelPoolConfig.MaxChannelCount
	} else if config.ChannelPoolConfig.MaxChannelCount > 1 {
		maxChannelPerConnection = config.ChannelPoolConfig.MaxChannelCount/config.ConnectionPoolConfig.MaxConnectionCount + 1
	}

	cp := &ConnectionPool{
		config:                     *config,
		uri:                        config.ConnectionPoolConfig.URI,
		connectionName:             config.ConnectionPoolConfig.ConnectionName,
		enableTLS:                  config.ConnectionPoolConfig.EnableTLS,
		tlsConfig:                  tlsConfig,
		errors:                     make(chan error, config.ConnectionPoolConfig.ErrorBuffer),
		heartbeat:                  time.Duration(config.ConnectionPoolConfig.Heartbeat) * time.Second,
		connectionTimeout:          time.Duration(config.ConnectionPoolConfig.ConnectionTimeout) * time.Second,
		maxConnections:             config.ConnectionPoolConfig.MaxConnectionCount,
		maxChannelPerConnection:    maxChannelPerConnection,
		maxAckChannelPerConnection: config.ChannelPoolConfig.MaxAckChannelCount/config.ConnectionPoolConfig.MaxConnectionCount + 1,
		connections:                queue.New(int64(config.ConnectionPoolConfig.MaxConnectionCount)), // possible overflow error
		poolLock:                   &sync.Mutex{},
		poolRWLock:                 &sync.RWMutex{},
		flaggedConnections:         make(map[uint64]bool),
		sleepOnErrorInterval:       time.Duration(config.ConnectionPoolConfig.SleepOnErrorInterval) * time.Millisecond,
	}

	if initializeNow {
		if err = cp.Initialize(); err != nil {
			return nil, err
		}
	}

	return cp, nil
}

// Initialize creates the ConnectionPool based on the config details.
// Blocks on network/communication issues unless overridden by config.
func (cp *ConnectionPool) Initialize() error {
	cp.poolLock.Lock()
	defer cp.poolLock.Unlock()

	if !cp.Initialized {
		var ok bool

		if cp.config.ConnectionPoolConfig.EnableTLS {
			ok = cp.initializeWithTLS()
		} else {
			ok = cp.initialize()
		}

		if ok {
			cp.Initialized = true
		} else {
			return errors.New("initialization failed during connection creation")
		}
	}

	return nil
}

func (cp *ConnectionPool) initialize() bool {

	for i := uint64(0); i < cp.maxConnections; i++ {
		connectionHost, err := cp.createConnectionHost(cp.connectionID)
		if err != nil {
			cp.connectionID = 0
			cp.connections = queue.New(int64(cp.config.ConnectionPoolConfig.MaxConnectionCount))
			return false
		}

		cp.connectionID++
		if err = cp.connections.Put(connectionHost); err != nil {
			cp.connectionID = 0
			cp.connections = queue.New(int64(cp.config.ConnectionPoolConfig.MaxConnectionCount))
			return false
		}
	}

	return true
}

func (cp *ConnectionPool) initializeWithTLS() bool {

	for i := uint64(0); i < cp.maxConnections; i++ {
		connectionHost, err := cp.createConnectionHostWithTLS(cp.connectionID)
		if err != nil {
			cp.connectionID = 0
			cp.connections = queue.New(int64(cp.config.ConnectionPoolConfig.MaxConnectionCount))
			return false
		}

		cp.connectionID++
		if err = cp.connections.Put(connectionHost); err != nil {
			cp.connectionID = 0
			cp.connections = queue.New(int64(cp.config.ConnectionPoolConfig.MaxConnectionCount))
			return false
		}
	}

	return true
}

// CreateConnectionHost creates the Connection with RabbitMQ server.
func (cp *ConnectionPool) createConnectionHost(connectionID uint64) (*models.ConnectionHost, error) {

	return models.NewConnectionHost(
		cp.uri,
		cp.connectionName+"-"+strconv.FormatUint(connectionID, 10),
		connectionID,
		cp.heartbeat,
		cp.connectionTimeout,
		cp.maxChannelPerConnection,
		cp.maxAckChannelPerConnection)
}

// CreateConnectionHostWithTLS creates the Connection with RabbitMQ server.
func (cp *ConnectionPool) createConnectionHostWithTLS(connectionID uint64) (*models.ConnectionHost, error) {
	if cp.tlsConfig == nil {
		return nil, errors.New("tls enabled but tlsConfig has not been created")
	}

	return models.NewConnectionHostWithTLS(
		cp.uri,
		cp.connectionName+"-"+strconv.FormatUint(connectionID, 10),
		connectionID,
		cp.heartbeat,
		cp.connectionTimeout,
		cp.maxChannelPerConnection,
		cp.maxAckChannelPerConnection,
		cp.tlsConfig)
}

func (cp *ConnectionPool) handleError(err error) {
	go func() { cp.errors <- err }()
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
		return nil, errors.New("can't get connection - connection pool has been shutdown")
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

			if cp.sleepOnErrorInterval > 0 {
				time.Sleep(cp.sleepOnErrorInterval)
			}

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
		}

		cp.UnflagConnection(replacementConnectionID)
	}

	return connectionHost, nil
}

// ReturnConnection puts the connection back in the queue.
// This helps maintain a Round Robin on Connections and their resources.
func (cp *ConnectionPool) ReturnConnection(connHost *models.ConnectionHost) {
	if err := cp.connections.Put(connHost); err != nil {
		cp.handleError(err)
	}
}

// ConnectionCount flags that connection as usable in the future. Careful, locking call.
func (cp *ConnectionPool) ConnectionCount() int64 {
	return cp.connections.Len() // Locking
}

// UnflagConnection flags that connection as usable in the future.
func (cp *ConnectionPool) UnflagConnection(connectionID uint64) {
	cp.poolRWLock.Lock()
	defer cp.poolRWLock.Unlock()
	cp.flaggedConnections[connectionID] = false
}

// FlagConnection flags that connection as non-usable in the future.
func (cp *ConnectionPool) FlagConnection(connectionID uint64) {
	cp.poolRWLock.Lock()
	defer cp.poolRWLock.Unlock()
	cp.flaggedConnections[connectionID] = true
}

// IsConnectionFlagged checks to see if the connection has been flagged for removal.
func (cp *ConnectionPool) IsConnectionFlagged(connectionID uint64) bool {
	cp.poolRWLock.RLock()
	defer cp.poolRWLock.RUnlock()
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

		cp.FlushErrors()
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
