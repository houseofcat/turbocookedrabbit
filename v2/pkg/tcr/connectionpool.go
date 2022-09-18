package tcr

import (
	"errors"
	"fmt"
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

	shutdownChan chan struct{}
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
		shutdownChan:         make(chan struct{}),
	}

	err := cp.initializeConnections()
	if err != nil {
		// we do not "handle" errors that are properly returned
		return nil, fmt.Errorf("initialization failed during connection creation: %w", err)
	}

	return cp, nil
}

func (cp *ConnectionPool) initializeConnections() error {

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
			return err
		}

		if err = cp.connections.Put(connectionHost); err != nil {
			return fmt.Errorf("%w: %v", ErrConnectionPoolClosed, err)
		}

		cp.connectionID++
	}

	for i := uint64(0); i < cp.Config.MaxCacheChannelCount; i++ {
		ch, err := cp.createCacheChannel(i)
		if err != nil {
			return err
		}
		cp.channels <- ch
	}

	return nil
}

// GetConnection gets a connection based on whats in the ConnectionPool (blocking under bad network conditions).
// Flowcontrol (blocking) or transient network outages will pause here until cleared.
// Uses the SleepOnErrorInterval to pause between retries.
func (cp *ConnectionPool) GetConnection() (*ConnectionHost, error) {

	connHost, err := cp.getConnectionFromPool()
	if err != nil { // errors upon shutdown
		return nil, err
	}

	cp.verifyHealthyConnection(connHost)

	return connHost, nil
}

func (cp *ConnectionPool) getConnectionFromPool() (*ConnectionHost, error) {

	// Pull from the queue.
	// Pauses here indefinitely if the queue is empty.
	// The only exception to this pausing is when the
	// pool has been shut down asyncronously and the queue
	// has been disposed of.
	structs, err := cp.connections.Get(1)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrConnectionPoolClosed, err)
	}

	connHost, ok := structs[0].(*ConnectionHost)
	if !ok {
		// library programming error that cannot be handled
		panic("invalid struct type found in ConnectionPool queue")
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
	if flagged || !healthy || connHost.Connection.IsClosed( /* atomic */ ) {
		cp.triggerConnectionRecovery(connHost)
	}

	connHost.PauseOnFlowControl()
}

func (cp *ConnectionPool) triggerConnectionRecovery(connHost *ConnectionHost) {

	// InfiniteLoop: Stay here till we reconnect.
	for {
		if cp.isShutdown() {
			return
		}

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

	// upon shutdown this would return an error.
	// at this point the (e.g. http request) processing would have
	// already finished, so the user does not need to handle this error
	_ = cp.connections.Put(connHost)
}

// GetChannelFromPool gets a cached ackable channel from the Pool if they exist or creates a channel.
// A non-acked channel is always a transient channel.
// Blocking if Ackable is true and the cache is empty.
// If you want a transient Ackable channel (un-managed), use CreateChannel directly.
// In case a connection pool shutdown is triggered asynchronously, this
// method returns an error
func (cp *ConnectionPool) GetChannelFromPool() (*ChannelHost, error) {
	select {
	case <-cp.catchShutdown():
		return nil, fmt.Errorf("failed to get channel: %w", ErrConnectionPoolClosed)
	case ch := <-cp.channels:
		return ch, nil
	}
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

func (cp *ConnectionPool) reconnectChannel(chanHost *ChannelHost) error {

	// InfiniteLoop: Stay here till we reconnect.
	for {
		if cp.isShutdown() {
			// received a shutdown signal while a user held
			// the cached chanHost that he tries to return
			// back to the connetion pool
			wg := &sync.WaitGroup{}
			go func(ch *ChannelHost) {
				defer wg.Done()
				defer func() { _ = recover() }()
				ch.Close()
			}(chanHost)

			wg.Wait()
			return fmt.Errorf("aborting channel reconnection: %w", ErrConnectionPoolClosed)
		}

		cp.verifyHealthyConnection(chanHost.connHost) // <- blocking operation

		err := chanHost.MakeChannel() // Creates a new channel and flushes internal buffers automatically.
		if err != nil {
			cp.handleError(err)
			continue
		}
		break
	}
	return nil
}

// createCacheChannel allows you create a cached ChannelHost which helps wrap Amqp Channel functionality.
// Only returns an error in case a connection pool shutdown was requested.
// The returned error is a wrapped ErrConnectionPoolClosed.
func (cp *ConnectionPool) createCacheChannel(id uint64) (*ChannelHost, error) {

	// InfiniteLoop: Stay till we have a good channel.
	for {
		connHost, err := cp.GetConnection()
		if err != nil {
			if errors.Is(err, ErrConnectionPoolClosed) {
				return nil, err
			}
			// must not be called in case an error is returned.
			// this handling is only needed for errors that are not returned.
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
		return chanHost, nil
	}
}

// GetTransientChannel allows you create an unmanaged amqp Channel with the help of the ConnectionPool.
// Only returns an error in case a connection pool shutdown was requested.
// The returned error is a wrapped ErrConnectionPoolClosed.
func (cp *ConnectionPool) GetTransientChannel(ackable bool) (*amqp.Channel, error) {

	// InfiniteLoop: Stay till we have a good channel.
	for {
		connHost, err := cp.GetConnection()
		if err != nil {
			if errors.Is(err, ErrConnectionPoolClosed) {
				return nil, err
			}
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
		return channel, nil
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

			connectionHost, ok := item.(*ConnectionHost)
			if !ok {

			}

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

	// signal shutdown
	close(cp.shutdownChan)
	// any further queue access yields an error
	cp.connections.Dispose()

	wg.Wait()

	// TODO: data race, as shutdown is more of a reset than
	// a one time shutdown, does it make sense to shutdown only once
	// and not reset the internal state for reuse?
	// any of the below variables may be accessed asynchronously
	// while we change them here
	cp.connections = queue.New(int64(cp.Config.MaxConnectionCount))
	cp.flaggedConnections = make(map[uint64]bool)
	cp.connectionID = 0
	cp.shutdownChan = make(chan struct{})
}

func (cp *ConnectionPool) isShutdown() bool {
	select {
	case <-cp.shutdownChan:
		return true
	default:
		return false
	}
}

func (cp *ConnectionPool) catchShutdown() <-chan struct{} {
	return cp.shutdownChan
}

func (cp *ConnectionPool) handleError(err error) {
	if cp.errorHandler != nil {
		cp.errorHandler(err)
	}
	if cp.sleepOnErrorInterval > 0 {
		time.Sleep(cp.sleepOnErrorInterval)
	}
}
