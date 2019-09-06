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
	Config                  *models.RabbitSeasoning
	Initialized             bool
	tlsConfig               *tls.Config
	errors                  chan error
	connections             *queue.Queue
	connectionCount         uint64
	poolLock                *sync.Mutex
	connectionLock          int32
	flaggedConnections      map[uint64]bool
	smallSleep              time.Duration
	initializeErrorCountMax int
}

// NewConnectionPool creates hosting structure for the ConnectionPool.
// Needs to be Initialize() afterwards.
func NewConnectionPool(
	config *models.RabbitSeasoning,
	initializeNow bool) (*ConnectionPool, error) {

	var tlsConfig *tls.Config
	var err error

	if config.TLSConfig.EnableTLS {
		tlsConfig, err = utils.CreateTLSConfig(
			config.TLSConfig.PEMCertLocation,
			config.TLSConfig.LocalCertLocation)
		if err != nil {
			return nil, err
		}
	}

	cp := &ConnectionPool{
		Config:                  config,
		tlsConfig:               tlsConfig,
		errors:                  make(chan error, 1),
		connections:             queue.New(config.PoolConfig.ConnectionCount),
		poolLock:                &sync.Mutex{},
		flaggedConnections:      make(map[uint64]bool),
		smallSleep:              time.Duration(50) * time.Millisecond,
		initializeErrorCountMax: 5,
	}

	if initializeNow {
		cp.Initialize()
	}

	return cp, nil
}

// Initialize creates the ConnectionPool based on the config details.
// Blocks on network/communication issues unless overridden by config.
func (cp *ConnectionPool) Initialize() {
	cp.poolLock.Lock()
	defer cp.poolLock.Unlock()

	if !cp.Initialized {
		if cp.Config.TLSConfig.EnableTLS {
			cp.initializeWithTLS()
		} else {
			cp.initialize()
		}

		cp.Initialized = true
	}
}

func (cp *ConnectionPool) initialize() {
	errCount := 0
	for i := int64(0); i < atomic.LoadInt64(&cp.Config.PoolConfig.ConnectionCount); i++ {
		connectionHost, err := cp.createConnectionHost(atomic.LoadUint64(&cp.connectionCount))
		if err != nil {
			go func() { cp.errors <- err }()
			errCount++

			if cp.Config.PoolConfig.BreakOnError || errCount >= cp.initializeErrorCountMax {
				break
			}

			time.Sleep(cp.smallSleep)
			continue
		}

		atomic.AddUint64(&cp.connectionCount, 1)
		cp.connections.Put(connectionHost)
	}
}

func (cp *ConnectionPool) initializeWithTLS() {
	errCount := 0
	for i := int64(0); i < atomic.LoadInt64(&cp.Config.PoolConfig.ConnectionCount); i++ {
		connectionHost, err := cp.createConnectionHostWithTLS(atomic.LoadUint64(&cp.connectionCount))
		if err != nil {
			go func() { cp.errors <- err }()
			errCount++

			if cp.Config.PoolConfig.BreakOnError || errCount >= cp.initializeErrorCountMax {
				break
			}

			time.Sleep(cp.smallSleep)
			continue
		}

		atomic.AddUint64(&cp.connectionCount, 1)
		cp.connections.Put(connectionHost)
	}
}

// CreateConnectionHost creates the Connection with RabbitMQ server.
func (cp *ConnectionPool) createConnectionHost(connectionID uint64) (*models.ConnectionHost, error) {

	var amqpConn *amqp.Connection
	var err error
	retryCount := atomic.LoadUint32(&cp.Config.PoolConfig.ConnectionRetryCount)

	for i := retryCount + 1; i > 0; i-- {
		amqpConn, err = amqp.Dial(cp.Config.PoolConfig.URI)
		if err != nil {
			go func() { cp.errors <- err }()

			if cp.Config.PoolConfig.BreakOnError {
				break
			}

			time.Sleep(cp.smallSleep)
			continue
		}

		break
	}

	if amqpConn == nil {
		return nil, errors.New("opening connections retries exhausted")
	}

	connectionHost := &models.ConnectionHost{
		Connection:   amqpConn,
		ConnectionID: connectionID,
	}

	return connectionHost, nil
}

// CreateConnectionHostWithTLS creates the Connection with RabbitMQ server.
func (cp *ConnectionPool) createConnectionHostWithTLS(connectionID uint64) (*models.ConnectionHost, error) {
	if cp.tlsConfig == nil {
		return nil, errors.New("tls enabled but tlsConfig has not been created")
	}

	var amqpConn *amqp.Connection
	var err error
	retryCount := atomic.LoadUint32(&cp.Config.PoolConfig.ConnectionRetryCount)

	for i := retryCount + 1; i > 0; i-- {
		amqpConn, err = amqp.DialTLS("amqps://"+cp.Config.TLSConfig.CertServerName, cp.tlsConfig)
		if err != nil {

			go func() { cp.errors <- err }()

			if cp.Config.PoolConfig.BreakOnError {
				break
			}

			time.Sleep(cp.smallSleep)
			continue
		}

		break
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

	// Between these three states we do our best to determine that a connection is dead in the various
	// lifecycles.
	if notifiedClosed || cp.IsConnectionFlagged(connectionHost.ConnectionID) || connectionHost.Connection.IsClosed() {

		var err error

		if cp.Config.TLSConfig.EnableTLS { // Replacement Connection
			connectionHost, err = cp.createConnectionHostWithTLS(connectionHost.ConnectionID)
			if err != nil {
				return nil, err
			}
		} else { // Replacement Connection
			connectionHost, err = cp.createConnectionHost(connectionHost.ConnectionID)
			if err != nil {
				return nil, err
			}
		}

		cp.UnflagConnection(connectionHost.ConnectionID)
	}

	// Puts the connection back in the queue while also returning a pointer to the caller.
	// This creates a Round Robin on Connections and their resources.
	cp.connections.Put(connectionHost)

	return connectionHost, nil
}

// ConnectionCount flags that connection as usable in the future.
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

		cp.connections = queue.New(cp.Config.PoolConfig.ConnectionCount)
		cp.flaggedConnections = make(map[uint64]bool)
		atomic.StoreUint64(&cp.connectionCount, uint64(0))
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
