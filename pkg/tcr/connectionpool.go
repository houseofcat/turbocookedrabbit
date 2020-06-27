package tcr

import (
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Workiva/go-datastructures/queue"
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
	poolLock             *sync.Mutex
	poolRWLock           *sync.RWMutex
	connectionLock       int32
	flaggedConnections   map[uint64]bool
	sleepOnErrorInterval time.Duration
}

// NewConnectionPool creates hosting structure for the ConnectionPool.
func NewConnectionPool(config *PoolConfig) (*ConnectionPool, error) {

	if config.ConnectionPoolConfig.Heartbeat == 0 || config.ConnectionPoolConfig.ConnectionTimeout == 0 {
		return nil, errors.New("connectionpool heartbeat or connectiontimeout can't be 0")
	}

	if config.ConnectionPoolConfig.MaxConnectionCount == 0 {
		return nil, errors.New("connectionpool maxconnectioncount can't be 0")
	}

	cp := &ConnectionPool{
		Config:               *config,
		uri:                  config.ConnectionPoolConfig.URI,
		heartbeatInterval:    time.Duration(config.ConnectionPoolConfig.Heartbeat) * time.Second,
		connectionTimeout:    time.Duration(config.ConnectionPoolConfig.ConnectionTimeout) * time.Second,
		connections:          queue.New(int64(config.ConnectionPoolConfig.MaxConnectionCount)), // possible overflow error
		channels:             make(chan *ChannelHost, config.ConnectionPoolConfig.MaxCacheChannelCount),
		poolLock:             &sync.Mutex{},
		poolRWLock:           &sync.RWMutex{},
		flaggedConnections:   make(map[uint64]bool),
		sleepOnErrorInterval: time.Duration(config.ConnectionPoolConfig.SleepOnErrorInterval) * time.Millisecond,
	}

	if ok := cp.initializeConnections(); !ok {
		return nil, errors.New("initialization failed during connection creation")
	}

	return cp, nil
}

func (cp *ConnectionPool) initializeConnections() bool {

	cp.connectionID = 0
	cp.connections = queue.New(int64(cp.Config.ConnectionPoolConfig.MaxConnectionCount))

	for i := uint64(0); i < cp.Config.ConnectionPoolConfig.MaxConnectionCount; i++ {

		connectionHost, err := NewConnectionHost(
			cp.uri,
			cp.Config.ConnectionPoolConfig.ConnectionName+"-"+strconv.FormatUint(cp.connectionID, 10),
			cp.connectionID,
			cp.heartbeatInterval,
			cp.connectionTimeout,
			cp.Config.ConnectionPoolConfig.TLSConfig)

		if err != nil {
			return false
		}

		if err = cp.connections.Put(connectionHost); err != nil {
			return false
		}

		cp.connectionID++
	}

	for i := uint64(0); i < cp.Config.ConnectionPoolConfig.MaxCacheChannelCount; i++ {
		cp.channels <- cp.createCacheChannel(i)
	}

	return true
}

// GetConnection gets a connection based on whats in the ConnectionPool (blocking under bad network conditions).
// Flowcontrol (blocking) or transient network outages will pause here until cleared.
// Uses the SleepOnErrorInterval to pause between retries.
func (cp *ConnectionPool) GetConnection() (*ConnectionHost, error) {

	var connHost *ConnectionHost
GetConnectionHost:
	for {

		// Pull from the queue.
		// Pauses here indefinitely if the queue is empty.
		structs, err := cp.connections.Get(1)
		if err != nil {
			return nil, err
		}

		var ok bool
		connHost, ok = structs[0].(*ConnectionHost)
		if !ok {
			return nil, errors.New("invalid struct type found in ConnectionPool queue")
		}

		// Pause in-place for flow control.
		for {
			select {
			case blocker := <-connHost.Blockers(): // Check for flow control issues.
				if !blocker.Active {
					break GetConnectionHost // everything's good, continue down.
				}
				time.Sleep(time.Millisecond)
			default:
				break GetConnectionHost // everything's good, continue down.
			}
		}
	}

	healthy := true
	select {
	case <-connHost.Errors():
		healthy = false
	default:
		break
	}

	// Assignment makes debugging easier
	closed := connHost.Connection.IsClosed()
	flagged := cp.IsConnectionFlagged(connHost.ConnectionID)

	// Between these three states we do our best to determine that a connection is dead in the various lifecycles.
	if flagged || !healthy || closed {

		cp.FlagConnection(connHost.ConnectionID)

		var err error
		replacementConnectionID := connHost.ConnectionID
		connHost = nil

		// Do not leave without a good Connection.
		for connHost == nil {

			connHost, err = NewConnectionHost(
				cp.uri,
				cp.Config.ConnectionPoolConfig.ConnectionName+"-"+strconv.FormatUint(replacementConnectionID, 10),
				replacementConnectionID,
				cp.heartbeatInterval,
				cp.connectionTimeout,
				cp.Config.ConnectionPoolConfig.TLSConfig)
			if err != nil {

				if cp.sleepOnErrorInterval > 0 {
					time.Sleep(cp.sleepOnErrorInterval)
				}

				continue
			}
		}

		cp.UnflagConnection(replacementConnectionID)
	}

	return connHost, nil
}

// ReturnConnection puts the connection back in the queue and flag it for error.
// This helps maintain a Round Robin on Connections and their resources.
func (cp *ConnectionPool) ReturnConnection(connHost *ConnectionHost, flag bool) {

	if flag {
		cp.FlagConnection(connHost.ConnectionID)
	}

	cp.connections.Put(connHost)
}

// GetChannel gets a cached ackable channel from the Pool if they exist or creates a channel.
// A non-acked channel is always a transient channel.
// Blocking if Ackable is true and the cache is empty.
// If you want a transient Ackable channel (un-managed), use CreateChannel directly.
func (cp *ConnectionPool) GetChannel(ackable bool) *ChannelHost {
	if ackable && cp.Config.ConnectionPoolConfig.MaxCacheChannelCount > 0 {
		return <-cp.channels
	}

	return cp.CreateTransientChannel(ackable)
}

// ReturnChannel returns a Channel.
// If Channel is not a cached channel, it is simply closed here.
// If Cache Channel, we check if erred, new Channel is created instead and then returned to the cache.
func (cp *ConnectionPool) ReturnChannel(channelHost *ChannelHost, erred bool) {

	// If called by user with the wrong channel don't add a non-managed channel back to the channel cache.
	if channelHost.CachedChannel {
		if erred {
			var currentID = channelHost.ID
			channelHost = cp.createCacheChannel(currentID)
			cp.channels <- channelHost
			return
		}

		cp.channels <- channelHost
		return
	}

	go func() {
		defer func() { _ = recover() }()
		channelHost.Close()
	}()
}

// createCacheChannel allows you create a ChannelHost which helps wrap Amqp Channel functionality.
func (cp *ConnectionPool) createCacheChannel(id uint64) *ChannelHost {

	for {
		connHost, err := cp.GetConnection()
		if err != nil {
			if cp.sleepOnErrorInterval > 0 {
				time.Sleep(cp.sleepOnErrorInterval)
			}
			continue
		}

		chanHost, err := NewChannelHost(connHost.Connection, id, connHost.ConnectionID, true, true)
		if err != nil {
			if cp.sleepOnErrorInterval > 0 {
				time.Sleep(cp.sleepOnErrorInterval)
			}
			cp.ReturnConnection(connHost, true)
			continue
		}

		cp.ReturnConnection(connHost, false)
		return chanHost
	}
}

// CreateTransientChannel allows you create an unmanaged ChannelHost which helps wrap Amqp Channel functionality.
func (cp *ConnectionPool) CreateTransientChannel(ackable bool) *ChannelHost {

	for {
		connHost, err := cp.GetConnection()
		if err != nil {
			if cp.sleepOnErrorInterval > 0 {
				time.Sleep(cp.sleepOnErrorInterval)
			}
			continue
		}

		chanHost, err := NewChannelHost(connHost.Connection, 10000, connHost.ConnectionID, ackable, false)
		if err != nil {
			if cp.sleepOnErrorInterval > 0 {
				time.Sleep(cp.sleepOnErrorInterval)
			}
			cp.ReturnConnection(connHost, true)
			continue
		}

		cp.ReturnConnection(connHost, false)
		return chanHost
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

	cp.connections = queue.New(int64(cp.Config.ConnectionPoolConfig.MaxConnectionCount))
	cp.flaggedConnections = make(map[uint64]bool)
	cp.connectionID = 0

	// Release connection lock (0)
	atomic.StoreInt32(&cp.connectionLock, 0)
}
