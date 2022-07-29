package tcr

import (
	"sync"
	"time"
)

type ConnectionPool struct {
	Config               PoolConfig
	uri                  string
	heartbeatInterval    time.Duration
	connectionTimeout    time.Duration
	connections          chan *ConnectionHost
	channels             chan *ChannelHost
	connectionID         uint64
	poolRWLock           *sync.RWMutex
	flaggedConnections   map[uint64]bool
	sleepOnErrorInterval time.Duration
	errorHandler         func(error)
	unhealthyHandler     func(error)
}
