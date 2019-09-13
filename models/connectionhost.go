package models

import (
	"crypto/tls"
	"errors"
	"sync"

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
	chLock             *sync.Mutex
}

// NewConnectionHost creates a simple ConnectionHost wrapper for management by end-user developer.
func NewConnectionHost(
	uri string,
	connectionID uint64,
	maxChannel uint64,
	maxAckChannelCount uint64) (*ConnectionHost, error) {

	var amqpConn *amqp.Connection
	var err error

	amqpConn, err = amqp.Dial(uri)
	if err != nil {
		return nil, err
	}

	connectionHost := &ConnectionHost{
		Connection:      amqpConn,
		ConnectionID:    connectionID,
		closeErrors:     make(chan *amqp.Error, 1),
		chLock:          &sync.Mutex{},
		maxChannelCount: maxChannel,
	}

	connectionHost.Connection.NotifyClose(connectionHost.closeErrors)

	return connectionHost, nil
}

// NewConnectionHostWithTLS creates a simple ConnectionHost wrapper for management by end-user developer.
func NewConnectionHostWithTLS(
	certServerName string,
	connectionID uint64,
	maxChannel uint64,
	maxAckChannelCount uint64,
	tlsConfig *tls.Config) (*ConnectionHost, error) {

	var amqpConn *amqp.Connection
	var err error

	amqpConn, err = amqp.DialTLS("amqps://"+certServerName, tlsConfig)
	if err != nil {
		return nil, err
	}

	connectionHost := &ConnectionHost{
		Connection:      amqpConn,
		ConnectionID:    connectionID,
		closeErrors:     make(chan *amqp.Error, 1),
		chLock:          &sync.Mutex{},
		maxChannelCount: maxChannel,
	}

	connectionHost.Connection.NotifyClose(connectionHost.closeErrors)

	return connectionHost, nil
}

// CloseErrors allow you to listen for amqp.Error messages.
func (ch *ConnectionHost) CloseErrors() <-chan *amqp.Error {
	return ch.closeErrors
}

// AddChannel increments the count of currentChannels
func (ch *ConnectionHost) AddChannel() error {
	ch.chLock.Lock()
	defer ch.chLock.Unlock()

	if ch.channelCount >= ch.maxChannelCount {
		return errors.New("can't add any more channels to this connection host")
	}

	ch.channelCount++

	return nil
}

// RemoveChannel decrements the count of currentChannels.
func (ch *ConnectionHost) RemoveChannel() error {
	ch.chLock.Lock()
	defer ch.chLock.Unlock()

	if ch.channelCount <= 0 {
		return errors.New("can't remove any more channels from this connection host")
	}

	ch.channelCount--

	return nil
}

// CanAddAckChannel provides a true or false based on whether this connection host can handle more channels on it's connection (based on initialization).
func (ch *ConnectionHost) CanAddAckChannel() bool {
	ch.chLock.Lock()
	defer ch.chLock.Unlock()

	return ch.ackChannelCount < ch.maxChannelCount
}

// AddAckChannel increments the count of currentChannels
func (ch *ConnectionHost) AddAckChannel() error {
	ch.chLock.Lock()
	defer ch.chLock.Unlock()

	if ch.ackChannelCount >= ch.maxChannelCount {
		return errors.New("can't add any more channels to this connection host")
	}

	ch.ackChannelCount++

	return nil
}

// RemoveAckChannel decrements the count of currentChannels.
func (ch *ConnectionHost) RemoveAckChannel() error {
	ch.chLock.Lock()
	defer ch.chLock.Unlock()

	if ch.ackChannelCount <= 0 {
		return errors.New("can't remove any more channels from this connection host")
	}

	ch.ackChannelCount--

	return nil
}

// CanAddChannel provides a true or false based on whether this connection host can handle more channels on it's connection (based on initialization).
func (ch *ConnectionHost) CanAddChannel() bool {
	ch.chLock.Lock()
	defer ch.chLock.Unlock()

	return ch.channelCount < ch.maxChannelCount
}
