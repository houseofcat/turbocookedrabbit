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
	Config                  models.PoolConfig
	connectionPool          *ConnectionPool
	Initialized             bool
	errors                  chan error
	channels                *queue.Queue
	ackChannels             *queue.Queue
	maxChannels             uint64
	maxAckChannels          uint64
	channelID               uint64
	poolLock                *sync.Mutex
	channelLock             int32
	flaggedChannels         map[uint64]bool
	createChannelRetryCount uint16
	breakOnInitializeError  bool
	sleepOnErrorInterval    time.Duration
	maxInitializeErrorCount uint16
	globalQosCount          int
}

// NewChannelPool creates hosting structure for the ChannelPool.
func NewChannelPool(
	config *models.PoolConfig,
	connPool *ConnectionPool,
	initializeNow bool) (*ChannelPool, error) {

	if connPool == nil {
		var err error // If connPool is nil, create one here.
		connPool, err = NewConnectionPool(config.ConnectionPoolConfig, true)
		if err != nil {
			return nil, err
		}
	}

	cp := &ChannelPool{
		Config:                  *config,
		connectionPool:          connPool,
		errors:                  make(chan error, config.ChannelPoolConfig.ErrorBuffer),
		maxChannels:             config.ChannelPoolConfig.ChannelCount,
		maxAckChannels:          config.ChannelPoolConfig.AckChannelCount,
		channels:                queue.New(int64(config.ChannelPoolConfig.ChannelCount)),
		ackChannels:             queue.New(int64(config.ChannelPoolConfig.AckChannelCount)),
		poolLock:                &sync.Mutex{},
		flaggedChannels:         make(map[uint64]bool),
		createChannelRetryCount: config.ChannelPoolConfig.CreateChannelRetryCount,
		sleepOnErrorInterval:    time.Duration(config.ChannelPoolConfig.SleepOnErrorInterval) * time.Millisecond,
		breakOnInitializeError:  config.ChannelPoolConfig.BreakOnInitializeError,
		maxInitializeErrorCount: config.ChannelPoolConfig.MaxInitializeErrorCount,
		globalQosCount:          config.ChannelPoolConfig.GlobalQosCount,
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

	if !cp.connectionPool.Initialized {
		cp.connectionPool.Initialize()
	}

	if !cp.Initialized {
		cp.initialize()
		cp.Initialized = true
	}
}

func (cp *ChannelPool) initialize() {

	errCount := uint16(0)

	// Create Channel queue.
	for i := uint64(0); i < cp.maxChannels; i++ {

		channelHost, err := cp.createChannelHost(cp.channelID)
		if err != nil {
			cp.handleError(err)
			errCount++

			if cp.breakOnInitializeError || errCount >= cp.maxInitializeErrorCount {
				break
			}

			continue
		}

		cp.channelID++
		cp.channels.Put(channelHost)
	}

	errCount = 0

	// Create AckChannel queue.
	for i := uint64(0); i < cp.maxAckChannels; i++ {

		channelHost, err := cp.createChannelHost(cp.channelID)
		if err != nil {
			cp.handleError(err)
			errCount++

			if cp.breakOnInitializeError || errCount >= cp.maxInitializeErrorCount {
				break
			}

			continue
		}

		cp.channelID++
		cp.ackChannels.Put(channelHost)
	}
}

// CreateChannelHost creates the Channel (backed by a Connection) with RabbitMQ server.
func (cp *ChannelPool) createChannelHost(channelID uint64) (*models.ChannelHost, error) {

	var amqpChan *amqp.Channel
	var connHost *models.ConnectionHost
	var err error

	for i := cp.createChannelRetryCount + 1; i > 0; i-- {

		connHost, err = cp.connectionPool.GetConnection()
		if err != nil {
			cp.handleError(fmt.Errorf("opening channel failed - could not get connection [err: %s]", err))
			continue
		}

		if connHost.Connection.IsClosed() {
			cp.connectionPool.FlagConnection(connHost.ConnectionID)
			cp.handleError(fmt.Errorf("opening channel failed - connection %q was marked as closed [err: %s]", connHost.ConnectionID, err))
			continue
		}

		amqpChan, err = connHost.Connection.Channel()
		if err != nil {
			cp.connectionPool.FlagConnection(connHost.ConnectionID)
			cp.handleError(err)
			continue
		}

		break
	}

	if amqpChan == nil {
		return nil, errors.New("opening channel retries exhausted")
	}

	if cp.globalQosCount > 0 {
		amqpChan.Qos(cp.globalQosCount, 0, true)
	}

	channelHost := &models.ChannelHost{
		Channel:          amqpChan,
		ChannelID:        channelID,
		ConnectionClosed: connHost.Connection.IsClosed,
	}

	return channelHost, nil
}

func (cp *ChannelPool) handleError(err error) {
	go func() { cp.errors <- err }()

	if cp.sleepOnErrorInterval > 0 {
		time.Sleep(cp.sleepOnErrorInterval)
	}
}

// Errors yields all the internal errs for creating connections.
func (cp *ChannelPool) Errors() <-chan error {
	return cp.errors
}

// GetChannel gets a channel based on whats available in ChannelPool queue.
func (cp *ChannelPool) GetChannel() (*models.ChannelHost, error) {
	if atomic.LoadInt32(&cp.channelLock) > 0 {
		return nil, errors.New("can't get channel - channel pool has been shutdown")
	}

	if !cp.Initialized {
		return nil, errors.New("can't get channel - channel pool has not been initialized")
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

	notifiedClosed := false
	select {
	case <-channelHost.CloseErrors():
		notifiedClosed = true
	default:
		break
	}

	// Between these three states we do our best to determine that a channel is dead in the various
	// lifecycles.
	if notifiedClosed || channelHost.ConnectionClosed() || cp.IsChannelFlagged(channelHost.ChannelID) {

		channelHost, err = cp.createChannelHost(channelHost.ChannelID)
		if err != nil {
			return nil, err
		}

		cp.UnflagChannel(channelHost.ChannelID)
	}

	// Puts the connection back in the queue while also returning a pointer to the caller.
	// This creates a Round Robin on Connections and their resources.
	cp.channels.Put(channelHost)

	return channelHost, nil
}

// GetAckableChannel gets an ackable channel based on whats available in AckChannelPool queue.
func (cp *ChannelPool) GetAckableChannel(noWait bool) (*models.ChannelHost, error) {
	if atomic.LoadInt32(&cp.channelLock) > 0 {
		return nil, errors.New("can't get channel - channel pool has been shutdown")
	}

	if !cp.Initialized {
		return nil, errors.New("can't get channel - channel pool has not been initialized")
	}

	// Pull from the queue.
	// Pauses here if the queue is empty.
	structs, err := cp.ackChannels.Get(1)
	if err != nil {
		return nil, err
	}

	channelHost, ok := structs[0].(*models.ChannelHost)
	if !ok {
		return nil, errors.New("invalid struct type found in ChannelPool queue")
	}

	notifiedClosed := false
	select {
	case <-channelHost.CloseErrors():
		notifiedClosed = true
	default:
		break
	}

	// Between these three states we do our best to determine that a channel is dead in the various
	// lifecycles.
	if notifiedClosed || channelHost.ConnectionClosed() || cp.IsChannelFlagged(channelHost.ChannelID) {

		channelHost, err = cp.createChannelHost(channelHost.ChannelID)
		if err != nil {
			return nil, err
		}

		cp.UnflagChannel(channelHost.ChannelID)

		channelHost.Channel.Confirm(noWait)
	}

	// Puts the connection back in the queue while also returning a pointer to the caller.
	// This creates a Round Robin on Connections and their resources.
	cp.ackChannels.Put(channelHost)

	return channelHost, nil
}

// ChannelCount lets you know how many non-ackable channels you have to use.
func (cp *ChannelPool) ChannelCount() int64 {
	return cp.channels.Len() // Locking
}

// AckChannelCount lets you know how many ackable channels you have to use.
func (cp *ChannelPool) AckChannelCount() int64 {
	return cp.ackChannels.Len() // Locking
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
		done1 := make(chan bool, 1)
		done2 := make(chan bool, 1)

		go cp.shutdownChannels(done1)
		go cp.shutdownAckChannels(done2)

		<-done1
		<-done2

		cp.channels = queue.New(int64(cp.maxChannels))
		cp.channels = queue.New(int64(cp.maxAckChannels))
		cp.flaggedChannels = make(map[uint64]bool)
		cp.channelID = 0
		cp.Initialized = false

		cp.connectionPool.Shutdown()
	}

	// Release channel lock (0)
	atomic.StoreInt32(&cp.channelLock, 0)
}

func (cp *ChannelPool) shutdownChannels(done chan bool) {
	for !cp.channels.Empty() {
		items, _ := cp.channels.Get(cp.channels.Len())

		for _, item := range items {
			channelHost := item.(*models.ChannelHost)
			channelHost.Channel.Close()
		}
	}

	done <- true
}

func (cp *ChannelPool) shutdownAckChannels(done chan bool) {
	for !cp.ackChannels.Empty() {
		items, _ := cp.ackChannels.Get(cp.ackChannels.Len())

		for _, item := range items {
			channelHost := item.(*models.ChannelHost)
			channelHost.Channel.Close()
		}
	}

	done <- true
}

// FlushErrors empties all current errors in the error channel.
func (cp *ChannelPool) FlushErrors() {

FlushLoop:
	for {
		select {
		case <-cp.Errors():
		default:
			break FlushLoop
		}
	}
}
