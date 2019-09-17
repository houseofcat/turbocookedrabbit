package models

import (
	"crypto/tls"
	"errors"
	"sync"
	"time"

	"github.com/streadway/amqp"
)

// ConnectionHost is an internal representation of amqp.Connection.
type ConnectionHost struct {
	Connection         *amqp.Connection
	ConnectionID       uint64
	maxChannelCount    uint64
	maxAckChannelCount uint64
	channelCount       uint64
	ackChannelCount    uint64
	closeErrors        chan *amqp.Error
	chanRWLock         *sync.RWMutex
	ackChanRWLock      *sync.RWMutex
}

// NewConnectionHost creates a simple ConnectionHost wrapper for management by end-user developer.
func NewConnectionHost(
	uri string,
	connectionID uint64,
	heartbeat time.Duration,
	connectionTimeout time.Duration,
	maxChannel uint64,
	maxAckChannelCount uint64) (*ConnectionHost, error) {

	var amqpConn *amqp.Connection
	var err error

	amqpConn, err = amqp.DialConfig(uri, amqp.Config{
		Heartbeat: heartbeat,
		Dial:      amqp.DefaultDial(connectionTimeout),
	})
	if err != nil {
		return nil, err
	}

	connectionHost := &ConnectionHost{
		Connection:         amqpConn,
		ConnectionID:       connectionID,
		closeErrors:        make(chan *amqp.Error, 1),
		chanRWLock:         &sync.RWMutex{},
		ackChanRWLock:      &sync.RWMutex{},
		maxChannelCount:    maxChannel,
		maxAckChannelCount: maxAckChannelCount,
	}

	connectionHost.Connection.NotifyClose(connectionHost.closeErrors)

	return connectionHost, nil
}

// NewConnectionHostWithTLS creates a simple ConnectionHost wrapper for management by end-user developer.
func NewConnectionHostWithTLS(
	certServerName string,
	connectionID uint64,
	heartbeat time.Duration,
	connectionTimeout time.Duration,
	maxChannel uint64,
	maxAckChannelCount uint64,
	tlsConfig *tls.Config) (*ConnectionHost, error) {

	var amqpConn *amqp.Connection
	var err error

	amqpConn, err = amqp.DialConfig("amqps://"+certServerName, amqp.Config{
		Heartbeat:       heartbeat,
		Dial:            amqp.DefaultDial(connectionTimeout),
		TLSClientConfig: tlsConfig,
	})
	if err != nil {
		return nil, err
	}

	connectionHost := &ConnectionHost{
		Connection:      amqpConn,
		ConnectionID:    connectionID,
		closeErrors:     make(chan *amqp.Error, 1),
		chanRWLock:      &sync.RWMutex{},
		ackChanRWLock:   &sync.RWMutex{},
		maxChannelCount: maxChannel,
	}

	connectionHost.Connection.NotifyClose(connectionHost.closeErrors)

	return connectionHost, nil
}

// CloseErrors allow you to listen for amqp.Error messages.
func (ch *ConnectionHost) CloseErrors() <-chan *amqp.Error {
	return ch.closeErrors
}

// CanAddChannel provides a true or false based on whether this connection host can handle more channels on it's connection (based on initialization).
func (ch *ConnectionHost) CanAddChannel() bool {
	ch.chanRWLock.RLock()
	defer ch.chanRWLock.RUnlock()

	return ch.channelCount < ch.maxChannelCount
}

// AddChannel increments the count of currentChannels
func (ch *ConnectionHost) AddChannel() {
	ch.chanRWLock.Lock()
	defer ch.chanRWLock.Unlock()

	ch.channelCount++
}

// RemoveChannel decrements the count of currentChannels.
func (ch *ConnectionHost) RemoveChannel() error {
	ch.chanRWLock.Lock()
	defer ch.chanRWLock.Unlock()

	if ch.channelCount <= 0 {
		return errors.New("can't remove any more channels from this connection host")
	}

	ch.channelCount--

	return nil
}

// CanAddAckChannel provides a true or false based on whether this connection host can handle more channels on it's connection (based on initialization).
func (ch *ConnectionHost) CanAddAckChannel() bool {
	ch.ackChanRWLock.RLock()
	defer ch.ackChanRWLock.RUnlock()

	return ch.ackChannelCount < ch.maxAckChannelCount
}

// AddAckChannel increments the count of currentChannels
func (ch *ConnectionHost) AddAckChannel() {
	ch.ackChanRWLock.Lock()
	defer ch.ackChanRWLock.Unlock()

	ch.ackChannelCount++
}

// RemoveAckChannel decrements the count of currentChannels.
func (ch *ConnectionHost) RemoveAckChannel() error {
	ch.ackChanRWLock.Lock()
	defer ch.ackChanRWLock.Unlock()

	if ch.ackChannelCount <= 0 {
		return errors.New("can't remove any more channels from this connection host")
	}

	ch.ackChannelCount--

	return nil
}
