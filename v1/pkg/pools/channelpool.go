package pools

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Workiva/go-datastructures/queue"

	"github.com/houseofcat/turbocookedrabbit/v1/pkg/models"
)

// TODO: Investigate the value of Sync.Map instead of map + lock for FlaggedChannels.

// ChannelPool houses the pool of RabbitMQ channels.
type ChannelPool struct {
	Config               models.PoolConfig
	connectionPool       *ConnectionPool
	Initialized          bool
	errors               chan error
	channels             *queue.Queue
	ackChannels          *queue.Queue
	maxChannels          uint64
	maxAckChannels       uint64
	channelID            uint64
	poolLock             *sync.Mutex
	poolRWLock           *sync.RWMutex
	channelLock          int32
	flaggedChannels      map[uint64]bool
	sleepOnErrorInterval time.Duration
	globalQosCount       int
	ackNoWait            bool
}

// NewChannelPool creates hosting structure for the ChannelPool.
func NewChannelPool(
	config *models.PoolConfig,
	connPool *ConnectionPool,
	initializeNow bool) (*ChannelPool, error) {

	if config.ChannelPoolConfig.MaxChannelCount == 0 || config.ChannelPoolConfig.MaxAckChannelCount == 0 {
		return nil, errors.New("channelpool maxchannelcount or maxackchannelcount can't be 0")
	}

	if connPool == nil {
		var err error // If connPool is nil, create one here.
		connPool, err = NewConnectionPool(config, initializeNow)
		if err != nil {
			return nil, err
		}
	}

	cp := &ChannelPool{
		Config:               *config,
		connectionPool:       connPool,
		errors:               make(chan error, config.ChannelPoolConfig.ErrorBuffer),
		maxChannels:          config.ChannelPoolConfig.MaxChannelCount,
		maxAckChannels:       config.ChannelPoolConfig.MaxAckChannelCount,
		channels:             queue.New(int64(config.ChannelPoolConfig.MaxChannelCount)),
		ackChannels:          queue.New(int64(config.ChannelPoolConfig.MaxAckChannelCount)),
		poolLock:             &sync.Mutex{},
		poolRWLock:           &sync.RWMutex{},
		flaggedChannels:      make(map[uint64]bool),
		sleepOnErrorInterval: time.Duration(config.ChannelPoolConfig.SleepOnErrorInterval) * time.Millisecond,
		globalQosCount:       config.ChannelPoolConfig.GlobalQosCount,
		ackNoWait:            config.ChannelPoolConfig.AckNoWait,
	}

	if initializeNow {
		if err := cp.Initialize(); err != nil {
			return nil, err
		}
	}

	return cp, nil
}

// Initialize creates the ConnectionPool based on the config details.
// Blocks on network/communication issues unless overridden by config.
func (cp *ChannelPool) Initialize() error {
	cp.poolLock.Lock()
	defer cp.poolLock.Unlock()

	if !cp.connectionPool.Initialized {
		if err := cp.connectionPool.Initialize(); err != nil {
			return err
		}
	}

	if !cp.Initialized {
		ok := cp.initialize()
		if ok {
			cp.Initialized = true
		} else {
			return errors.New("errors occurred creating channels")
		}
	}

	return nil
}

func (cp *ChannelPool) initialize() bool {

	// Create Channel queue.
	for i := uint64(0); i < cp.maxChannels; i++ {

		channelHost, err := cp.createChannelHost(cp.channelID, false)
		if err != nil {
			cp.channelID = 0
			cp.channels = queue.New(int64(cp.Config.ChannelPoolConfig.MaxChannelCount))
			return false
		}

		cp.channelID++
		if err = cp.channels.Put(channelHost); err != nil {
			cp.channelID = 0
			cp.channels = queue.New(int64(cp.Config.ChannelPoolConfig.MaxChannelCount))
			return false
		}
	}

	// Create AckChannel queue.
	for i := uint64(0); i < cp.maxAckChannels; i++ {

		channelHost, err := cp.createChannelHost(cp.channelID, true)
		if err != nil {
			cp.channelID = 0
			cp.channels = queue.New(int64(cp.Config.ChannelPoolConfig.MaxAckChannelCount))
			return false
		}

		cp.channelID++
		if err = cp.ackChannels.Put(channelHost); err != nil {
			cp.channelID = 0
			cp.channels = queue.New(int64(cp.Config.ChannelPoolConfig.MaxAckChannelCount))
			return false
		}
	}

	return true
}

// CreateChannelHost creates the Channel (backed by a Connection) with RabbitMQ server.
func (cp *ChannelPool) createChannelHost(channelID uint64, ackable bool) (*ChannelHost, error) {

	getConnectionCounter := 0
GetNewConnection:
	if getConnectionCounter > 3 { // we give up if we find 3 full connections in a row (means the ChannelPool may already be healed and we are in a race condition)
		return nil, errors.New("-1")
	}

	connHost, err := cp.connectionPool.GetConnection()
	if err != nil {
		return nil, err
	}

	if (ackable && !connHost.CanAddAckChannel()) || (!ackable && !connHost.CanAddChannel()) {
		getConnectionCounter++
		cp.connectionPool.ReturnConnection(connHost) // return connection or lose them
		goto GetNewConnection
	}

	channelHost, err := NewChannelHost(connHost.Connection, channelID, connHost.ConnectionID, ackable)
	if err != nil {
		cp.connectionPool.FlagConnection(connHost.ConnectionID) // flag connection as a problem
		cp.connectionPool.ReturnConnection(connHost)            // return connection or lose them
		return nil, err
	}

	if ackable {
		connHost.AddAckChannel()
	} else {
		connHost.AddChannel()
	}

	if cp.globalQosCount > 0 {
		if err = channelHost.Channel.Qos(cp.globalQosCount, 0, true); err != nil {
			cp.handleError(err)
		}
	}

	if ackable {
		if err = channelHost.Channel.Confirm(cp.ackNoWait); err != nil {
			cp.handleError(err)
		}
	}

	cp.connectionPool.ReturnConnection(connHost)

	return channelHost, nil
}

func (cp *ChannelPool) handleError(err error) {
	go func() { cp.errors <- err }()
}

// Errors yields all the internal err chan for managing the ChannelPool.
func (cp *ChannelPool) Errors() <-chan error {
	return cp.errors
}

// GetChannel gets a channel based on whats ChannelPool queue (blocking under bad network conditions).
// Outages/transient network outages block until success connecting.
// Uses the SleepOnErrorInterval to pause between retries.
func (cp *ChannelPool) GetChannel() (*ChannelHost, error) {
	if atomic.LoadInt32(&cp.channelLock) > 0 {
		return nil, errors.New("can't get channel - channel pool has been shutdown")
	}

	if !cp.Initialized {
		time.Sleep(cp.sleepOnErrorInterval)
		return nil, errors.New("can't get channel - channel pool has not been initialized")
	}

	// Pull from the queue.
	// Pauses here if the queue is empty.
DequeueChannel:
	structs, err := cp.channels.Get(1)
	if err != nil {
		return nil, err
	}

	channelHost, ok := structs[0].(*ChannelHost)
	if !ok {
		return nil, errors.New("invalid struct type found in ChannelPool queue")
	}

	healthy := true
	select {
	case <-channelHost.CloseErrors():
		healthy = false
	default:
		break
	}

	// Between these two states we do our best to determine that a channel is dead in the various
	// lifecycles.
	if cp.IsChannelFlagged(channelHost.ChannelID) || !healthy {

		replacementChannelID := channelHost.ChannelID
		var newChannelHost *ChannelHost

		// Do not leave without a good ChannelHost.
		for newChannelHost == nil {

			if cp.sleepOnErrorInterval > 0 {
				time.Sleep(cp.sleepOnErrorInterval)
			}

			newChannelHost, err = cp.createChannelHost(replacementChannelID, false)
			if err != nil {
				if err.Error() == "-1" { // A control error of "-1" indicates we are at max channels for 3 separate connections. Try with a new channel.
					cp.ReturnChannel(channelHost, true) // return the bad channel since we don't want to lose our pool overtime
					goto DequeueChannel
				}
				continue
			}

			channelHost = newChannelHost
		}

		cp.UnflagChannel(replacementChannelID)
	}

	return channelHost, nil
}

// ReturnChannel puts the connection back in the queue.
// Developer has to manually return the Channel and helps maintain a Round Robin on Channels and their resources.
// Optional parameter allows you to flag a Channel as dead.
func (cp *ChannelPool) ReturnChannel(chanHost *ChannelHost, flagChannel bool) {
	if chanHost.IsAckable() {
		if err := cp.ackChannels.Put(chanHost); err != nil {
			cp.handleError(err)
		}
	} else {
		if err := cp.channels.Put(chanHost); err != nil {
			cp.handleError(err)
		}
	}

	if flagChannel {
		cp.FlagChannel(chanHost.ChannelID)
	}
}

// GetAckableChannel gets an ackable channel based on whats available in AckChannelPool queue.
func (cp *ChannelPool) GetAckableChannel() (*ChannelHost, error) {
	if atomic.LoadInt32(&cp.channelLock) > 0 {
		return nil, errors.New("can't get channel - channel pool has been shutdown")
	}

	if !cp.Initialized {
		time.Sleep(cp.sleepOnErrorInterval)
		return nil, errors.New("can't get channel - channel pool has not been initialized")
	}

	// Pull from the queue.
	// Pauses here if the queue is empty.
	structs, err := cp.ackChannels.Get(1)
	if err != nil {
		return nil, err
	}

	channelHost, ok := structs[0].(*ChannelHost)
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

	// Between these two states we do our best to determine that a channel is dead in the various
	// lifecycles.
	if notifiedClosed || cp.IsChannelFlagged(channelHost.ChannelID) {

		cp.connectionPool.FlagConnection(channelHost.ConnectionID)

		replacementChannelID := channelHost.ChannelID
		channelHost = nil

		for channelHost == nil {

			channelHost, err = cp.createChannelHost(replacementChannelID, true)
			if err != nil {
				if cp.sleepOnErrorInterval > 0 {
					time.Sleep(cp.sleepOnErrorInterval)
				}
				continue
			}
		}

		cp.UnflagChannel(replacementChannelID)
	}

	// Puts the connection back in the queue while also returning a pointer to the caller.
	// This creates a Round Robin on Connections and their resources.
	if err := cp.ackChannels.Put(channelHost); err != nil {
		cp.handleError(err)
	}

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

// UnflagChannel flags that channel as usable in the future.
func (cp *ChannelPool) UnflagChannel(channelID uint64) {
	cp.poolRWLock.Lock()
	defer cp.poolRWLock.Unlock()
	cp.flaggedChannels[channelID] = false
}

// FlagChannel flags that channel as non-usable in the future.
func (cp *ChannelPool) FlagChannel(channelID uint64) {
	cp.poolRWLock.Lock()
	defer cp.poolRWLock.Unlock()
	cp.flaggedChannels[channelID] = true
}

// IsChannelFlagged checks to see if the channel has been flagged for removal.
func (cp *ChannelPool) IsChannelFlagged(channelID uint64) bool {
	cp.poolRWLock.RLock()
	defer cp.poolRWLock.RUnlock()
	if flagged, ok := cp.flaggedChannels[channelID]; ok {
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
			channelHost := item.(*ChannelHost)
			channelHost.Channel.Close()
		}
	}

	done <- true
}

func (cp *ChannelPool) shutdownAckChannels(done chan bool) {
	for !cp.ackChannels.Empty() {
		items, _ := cp.ackChannels.Get(cp.ackChannels.Len())

		for _, item := range items {
			channelHost := item.(*ChannelHost)
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
