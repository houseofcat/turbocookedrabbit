package pools

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Workiva/go-datastructures/queue"
	"github.com/streadway/amqp"

	"github.com/houseofcat/turbocookedrabbit/models"
)

// ChannelPool houses the pool of RabbitMQ channels.
type ChannelPool struct {
	Config                  *models.RabbitSeasoning
	ConnectionPool          *ConnectionPool
	Initialized             bool
	errors                  chan error
	channels                *queue.Queue
	channelCount            uint64
	poolLock                *sync.Mutex
	channelLock             int32
	flaggedChannels         map[uint64]bool
	smallSleep              time.Duration
	initializeErrorCountMax int
}

// NewChannelPool creates hosting structure for the ChannelPool.
func NewChannelPool(seasoning *models.RabbitSeasoning, connPool *ConnectionPool, initializeNow bool) (*ChannelPool, error) {

	if connPool == nil {
		var err error // If connPool is nil, create one here.
		connPool, err = NewConnectionPool(seasoning, true)
		if err != nil {
			return nil, err
		}
	}

	cp := &ChannelPool{
		Config:                  seasoning,
		ConnectionPool:          connPool,
		errors:                  make(chan error, 1),
		channels:                queue.New(seasoning.Pools.ChannelCount),
		poolLock:                &sync.Mutex{},
		flaggedChannels:         make(map[uint64]bool),
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
func (cp *ChannelPool) Initialize() {
	cp.poolLock.Lock()
	defer cp.poolLock.Unlock()

	if !cp.ConnectionPool.Initialized {
		cp.ConnectionPool.Initialize()
	}

	if !cp.Initialized {
		cp.initialize()
		cp.Initialized = true
	}
}

func (cp *ChannelPool) initialize() {
	errCount := 0
	for i := int64(0); i < atomic.LoadInt64(&cp.Config.Pools.ChannelCount); i++ {
		channelHost, err := cp.createChannelHost(atomic.LoadUint64(&cp.channelCount))
		if err != nil {
			go func() { cp.errors <- err }()
			errCount++

			if cp.Config.Pools.BreakOnError || errCount >= cp.initializeErrorCountMax {
				break
			}

			time.Sleep(cp.smallSleep)
			continue
		}

		atomic.AddUint64(&cp.channelCount, 1)
		cp.channels.Put(channelHost)
	}
}

// CreateChannelHost creates the Channel (backed by a Connection) with RabbitMQ server.
func (cp *ChannelPool) createChannelHost(channelID uint64) (*models.ChannelHost, error) {

	var amqpChan *amqp.Channel
	var connHost *models.ConnectionHost
	var err error

	retryCount := atomic.LoadUint32(&cp.Config.Pools.ChannelRetryCount)
	connHost, err = cp.ConnectionPool.GetConnection()

	if connHost == nil {
		return nil, fmt.Errorf("opening channel failed - could not get connection [last err: %s]", err)
	}

	for i := retryCount + 1; i > 0; i-- {
		amqpChan, err = connHost.Connection.Channel()
		if err != nil {
			if cp.Config.Pools.BreakOnError {
				break
			}

			go func() { cp.errors <- err }()
			time.Sleep(cp.smallSleep)
			continue
		}

		break
	}

	if amqpChan == nil {
		return nil, errors.New("opening channel retries exhausted")
	}

	channelHost := &models.ChannelHost{
		Channel:          amqpChan,
		ChannelID:        channelID,
		ConnectionClosed: connHost.Connection.IsClosed,
	}

	return channelHost, nil
}

// Errors yields all the internal errs for creating connections.
func (cp *ChannelPool) Errors() <-chan error {
	return cp.errors
}

// GetChannel gets a connection based on whats available in ChannelPool queue.
func (cp *ChannelPool) GetChannel() (*models.ChannelHost, error) {
	if atomic.LoadInt32(&cp.channelLock) > 0 {
		return nil, errors.New("can not get channel - channel pool has been shutdown")
	}

	if !cp.Initialized {
		return nil, errors.New("can not get channel - channel pool has not been initialized")
	}

	// Pull from the queue.
	// Pauses here if the queue is empty.
	structs, err := cp.channels.Get(1)
	if err != nil {
		return nil, err
	}

	channelHost, ok := structs[0].(*models.ChannelHost)
	if !ok {
		return nil, errors.New("invalid struct type found in ChannelPool queue")
	}

	if channelHost.ConnectionClosed() || cp.IsChannelFlagged(channelHost.ChannelID) {

		var newHost *models.ChannelHost
		var err error

		newHost, err = cp.createChannelHost(channelHost.ChannelID)
		if err != nil {
			return nil, err
		}

		cp.UnflagChannel(channelHost.ChannelID)
		channelHost = newHost
	}

	// Puts the connection back in the queue while also returning a pointer to the caller.
	// This creates a Round Robin on Connections and their resources.
	cp.channels.Put(channelHost)

	return channelHost, nil
}

// ChannelCount flags that connection as usable in the future.
func (cp *ChannelPool) ChannelCount() int64 {
	return cp.channels.Len() // Locking
}

// UnflagChannel flags that connection as usable in the future.
func (cp *ChannelPool) UnflagChannel(connectionID uint64) {
	cp.poolLock.Lock()
	defer cp.poolLock.Unlock()
	cp.flaggedChannels[connectionID] = false
}

// FlagChannel flags that connection as non-usable in the future.
func (cp *ChannelPool) FlagChannel(connectionID uint64) {
	cp.poolLock.Lock()
	defer cp.poolLock.Unlock()
	cp.flaggedChannels[connectionID] = true
}

// IsChannelFlagged checks to see if the connection has been flagged for removal.
func (cp *ChannelPool) IsChannelFlagged(connectionID uint64) bool {
	cp.poolLock.Lock()
	defer cp.poolLock.Unlock()
	if flagged, ok := cp.flaggedChannels[connectionID]; ok {
		return flagged
	}

	return false
}

// Shutdown closes all channels and all connections.
func (cp *ChannelPool) Shutdown() {
	cp.poolLock.Lock()
	defer cp.poolLock.Unlock()

	// Create channel lock (> 0)
	atomic.AddInt32(&cp.channelLock, 1)

	if cp.Initialized {
		for !cp.channels.Empty() {
			items, _ := cp.channels.Get(cp.channels.Len())

			for _, item := range items {
				channelHost := item.(*models.ChannelHost)
				err := channelHost.Channel.Close()
				if err != nil {
					go func() { cp.errors <- err }()
				}
			}
		}

		cp.channels = queue.New(cp.Config.Pools.ChannelCount)
		cp.flaggedChannels = make(map[uint64]bool)
		atomic.StoreUint64(&cp.channelCount, uint64(0))
		cp.Initialized = false

		cp.ConnectionPool.Shutdown()
	}

	// Release channel lock (0)
	atomic.StoreInt32(&cp.channelLock, 0)
}
