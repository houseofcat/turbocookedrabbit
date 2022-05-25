package tcr

import (
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/Workiva/go-datastructures/queue"
	"github.com/streadway/amqp"
)

// ConnectionPool houses the pool of RabbitMQ connections.
type ConnectionPool struct {
	Config               PoolConfig
	uri                  string
	heartbeatInterval    time.Duration
	connectionTimeout    time.Duration
	connections          *queue.Queue
	channels             chan *ChannelHost
	connectionID         uint64
	poolRWLock           *sync.RWMutex
	flaggedConnections   map[uint64]bool
	sleepOnErrorInterval time.Duration
	errorHandler         func(error)
	unhealthyHandler     func(error)
}

// NewConnectionPool creates hosting structure for the ConnectionPool.
func NewConnectionPool(config *PoolConfig) (*ConnectionPool, error) {
	return NewConnectionPoolWithHandlers(config, nil, nil)
}

// NewConnectionPoolWithErrorHandler creates hosting structure for the ConnectionPool with an error handler.
func NewConnectionPoolWithErrorHandler(config *PoolConfig, errorHandler func(error)) (*ConnectionPool, error) {
	return NewConnectionPoolWithHandlers(config, errorHandler, nil)
}

// NewConnectionPoolWithUnhealthyHandler creates hosting structure for the ConnectionPool with an unhealthy handler.
func NewConnectionPoolWithUnhealthyHandler(config *PoolConfig, unhealthyHandler func(error)) (*ConnectionPool, error) {
	return NewConnectionPoolWithHandlers(config, nil, unhealthyHandler)
}

// NewConnectionPoolWithHandlers creates hosting structure for the ConnectionPool with an error and/or unhealthy handler.
func NewConnectionPoolWithHandlers(config *PoolConfig, errorHandler func(error), unhealthyHandler func(error)) (*ConnectionPool, error) {
	if config.Heartbeat == 0 || config.ConnectionTimeout == 0 {
		return nil, errors.New("connectionpool heartbeat or connectiontimeout can't be 0")
	}

	if config.MaxConnectionCount == 0 {
		return nil, errors.New("connectionpool maxconnectioncount can't be 0")
	}

	cp := &ConnectionPool{
		Config:               *config,
		uri:                  config.URI,
		heartbeatInterval:    time.Duration(config.Heartbeat) * time.Second,
		connectionTimeout:    time.Duration(config.ConnectionTimeout) * time.Second,
		connections:          queue.New(int64(config.MaxConnectionCount)), // possible overflow error
		channels:             make(chan *ChannelHost, config.MaxCacheChannelCount),
		poolRWLock:           &sync.RWMutex{},
		flaggedConnections:   make(map[uint64]bool),
		sleepOnErrorInterval: time.Duration(config.SleepOnErrorInterval) * time.Millisecond,
		errorHandler:         errorHandler,
		unhealthyHandler:     unhealthyHandler,
	}

	if ok := cp.initializeConnections(); !ok {
		return nil, errors.New("initialization failed during connection creation")
	}

	return cp, nil
}

func (cp *ConnectionPool) initializeConnections() bool {

	cp.connectionID = 0
	cp.connections = queue.New(int64(cp.Config.MaxConnectionCount))

	for i := uint64(0); i < cp.Config.MaxConnectionCount; i++ {

		connectionHost, err := NewConnectionHost(
			cp.uri,
			cp.Config.ApplicationName+"-"+strconv.FormatUint(cp.connectionID, 10),
			cp.connectionID,
			cp.heartbeatInterval,
			cp.connectionTimeout,
			cp.Config.TLSConfig)

		if err != nil {
			cp.handleError(err)
			return false
		}

		if err = cp.connections.Put(connectionHost); err != nil {
			cp.handleError(err)
			return false
		}

		cp.connectionID++
	}

	for i := uint64(0); i < cp.Config.MaxCacheChannelCount; i++ {
		cp.channels <- cp.createCacheChannel(i)
	}

	return true
}

// GetConnection gets a connection based on whats in the ConnectionPool (blocking under bad network conditions).
// Flowcontrol (blocking) or transient network outages will pause here until cleared.
// Uses the SleepOnErrorInterval to pause between retries.
func (cp *ConnectionPool) GetConnection() (*ConnectionHost, error) {

	connHost, err := cp.getConnectionFromPool()
	if err != nil { // errors on bad data in the queue
		cp.handleError(err)
		return nil, err
	}

	cp.verifyHealthyConnection(connHost)

	return connHost, nil
}

func (cp *ConnectionPool) getConnectionFromPool() (*ConnectionHost, error) {

	// Pull from the queue.
	// Pauses here indefinitely if the queue is empty.
	structs, err := cp.connections.Get(1)
	if err != nil {
		return nil, err
	}

	connHost, ok := structs[0].(*ConnectionHost)
	if !ok {
		return nil, errors.New("invalid struct type found in ConnectionPool queue")
	}

	return connHost, nil
}

func (cp *ConnectionPool) verifyHealthyConnection(connHost *ConnectionHost) {

	healthy := true
	select {
	case err := <-connHost.Errors:
		healthy = false
		if cp.unhealthyHandler != nil {
			cp.unhealthyHandler(err)
		}
	default:
		break
	}

	flagged := cp.isConnectionFlagged(connHost.ConnectionID)

	// Between these three states we do our best to determine that a connection is dead in the various lifecycles.
	if flagged || !healthy || connHost.Connection.IsClosed( /* atomic */) {
		cp.triggerConnectionRecovery(connHost)
	}

	connHost.PauseOnFlowControl()
}

func (cp *ConnectionPool) triggerConnectionRecovery(connHost *ConnectionHost) {

	// InfiniteLoop: Stay here till we reconnect.
	for {
		ok := connHost.ConnectWithErrorHandler(cp.unhealthyHandler)
		if !ok {
			if cp.sleepOnErrorInterval > 0 {
				time.Sleep(cp.sleepOnErrorInterval)
			}
			continue
		}
		break
	}

	// Flush any pending errors.
	for {
		select {
		case <-connHost.Errors:
		default:
			cp.unflagConnection(connHost.ConnectionID)
			return
		}
	}
}

// ReturnConnection puts the connection back in the queue and flag it for error.
// This helps maintain a Round Robin on Connections and their resources.
func (cp *ConnectionPool) ReturnConnection(connHost *ConnectionHost, flag bool) {

	if flag {
		cp.flagConnection(connHost.ConnectionID)
	}

	_ = cp.connections.Put(connHost)
}

// GetChannelFromPool gets a cached ackable channel from the Pool if they exist or creates a channel.
// A non-acked channel is always a transient channel.
// Blocking if Ackable is true and the cache is empty.
// If you want a transient Ackable channel (un-managed), use CreateChannel directly.
func (cp *ConnectionPool) GetChannelFromPool() *ChannelHost {

	return <-cp.channels
}

// ReturnChannel returns a Channel.
// If Channel is not a cached channel, it is simply closed here.
// If Cache Channel, we check if erred, new Channel is created instead and then returned to the cache.
func (cp *ConnectionPool) ReturnChannel(chanHost *ChannelHost, erred bool) {

	// If called by user with the wrong channel don't add a non-managed channel back to the channel cache.
	if chanHost.CachedChannel {
		if erred {
			cp.reconnectChannel(chanHost) // <- blocking operation
		} else {
			chanHost.FlushConfirms()
		}

		cp.channels <- chanHost
		return
	}

	go func(*ChannelHost) {
		defer func() { _ = recover() }()

		chanHost.Close()
	}(chanHost)
}

func (cp *ConnectionPool) reconnectChannel(chanHost *ChannelHost) {

	// InfiniteLoop: Stay here till we reconnect.
	for {
		cp.verifyHealthyConnection(chanHost.connHost) // <- blocking operation

		err := chanHost.MakeChannel() // Creates a new channel and flushes internal buffers automatically.
		if err != nil {
			cp.handleError(err)
			continue
		}
		break
	}
}

// createCacheChannel allows you create a cached ChannelHost which helps wrap Amqp Channel functionality.
func (cp *ConnectionPool) createCacheChannel(id uint64) *ChannelHost {

	// InfiniteLoop: Stay till we have a good channel.
	for {
		connHost, err := cp.GetConnection()
		if err != nil {
			cp.handleError(err)
			continue
		}

		chanHost, err := NewChannelHost(connHost, id, connHost.ConnectionID, true, true)
		if err != nil {
			cp.handleError(err)
			cp.ReturnConnection(connHost, true)
			continue
		}

		cp.ReturnConnection(connHost, false)
		return chanHost
	}
}

// GetTransientChannel allows you create an unmanaged amqp Channel with the help of the ConnectionPool.
func (cp *ConnectionPool) GetTransientChannel(ackable bool) *amqp.Channel {

	// InfiniteLoop: Stay till we have a good channel.
	for {
		connHost, err := cp.GetConnection()
		if err != nil {
			cp.handleError(err)
			continue
		}

		channel, err := connHost.Connection.Channel()
		if err != nil {
			cp.handleError(err)
			cp.ReturnConnection(connHost, true)
			continue
		}

		cp.ReturnConnection(connHost, false)

		if ackable {
			err := channel.Confirm(false)
			if err != nil {
				cp.handleError(err)
				continue
			}
		}
		return channel
	}
}

// UnflagConnection flags that connection as usable in the future.
func (cp *ConnectionPool) unflagConnection(connectionID uint64) {
	cp.poolRWLock.Lock()
	defer cp.poolRWLock.Unlock()
	cp.flaggedConnections[connectionID] = false
}

// FlagConnection flags that connection as non-usable in the future.
func (cp *ConnectionPool) flagConnection(connectionID uint64) {
	cp.poolRWLock.Lock()
	defer cp.poolRWLock.Unlock()
	cp.flaggedConnections[connectionID] = true
}

// IsConnectionFlagged checks to see if the connection has been flagged for removal.
func (cp *ConnectionPool) isConnectionFlagged(connectionID uint64) bool {
	cp.poolRWLock.RLock()
	defer cp.poolRWLock.RUnlock()
	if flagged, ok := cp.flaggedConnections[connectionID]; ok {
		return flagged
	}

	return false
}

// Shutdown closes all connections in the ConnectionPool and resets the Pool to pre-initialized state.
func (cp *ConnectionPool) Shutdown() {

	if cp == nil {
		return
	}

	wg := &sync.WaitGroup{}
ChannelFlushLoop:
	for {
		select {
		case chanHost := <-cp.channels:
			wg.Add(1)
			// Started receiving panics on Channel.Close()
			go func(*ChannelHost) {
				defer wg.Done()
				defer func() { _ = recover() }()

				chanHost.Close()
			}(chanHost)

		default:
			break ChannelFlushLoop
		}
	}
	wg.Wait()

	for !cp.connections.Empty() {
		items, _ := cp.connections.Get(cp.connections.Len())

		for _, item := range items {
			wg.Add(1)

			connectionHost := item.(*ConnectionHost)

			// Started receiving panics on Connection.Close()
			go func(*ConnectionHost) {
				defer wg.Done()
				defer func() { _ = recover() }()

				if !connectionHost.Connection.IsClosed() {
					connectionHost.Connection.Close()
				}
			}(connectionHost)

		}
	}

	wg.Wait()

	cp.connections = queue.New(int64(cp.Config.MaxConnectionCount))
	cp.flaggedConnections = make(map[uint64]bool)
	cp.connectionID = 0
}

func (cp *ConnectionPool) handleError(err error) {
	if cp.errorHandler != nil {
		cp.errorHandler(err)
	}
	if cp.sleepOnErrorInterval > 0 {
		time.Sleep(cp.sleepOnErrorInterval)
	}
}
