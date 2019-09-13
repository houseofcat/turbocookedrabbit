package models

import (
	"crypto/tls"
	"fmt"
	"time"

	"github.com/streadway/amqp"
)

// ConnectionHost is an internal representation of amqp.Connection.
type ConnectionHost struct {
	Connection   *amqp.Connection
	ConnectionID uint64
	closeErrors  chan *amqp.Error
}

// NewConnectionHost creates a simple ConnectionHost wrapper for management by end-user developer.
func (ch *ConnectionHost) NewConnectionHost(
	uri string,
	connectionID uint64,
	retryCount uint32) (*ConnectionHost, error) {

	var amqpConn *amqp.Connection
	var err error

	for i := retryCount + 1; i > 0; i-- {
		amqpConn, err = amqp.Dial(uri)
		if err != nil {
			time.Sleep(1 * time.Second)
		}
	}

	if amqpConn == nil {
		return nil, fmt.Errorf("opening connection retries exhausted [last err: %s]", err)
	}

	connectionHost := &ConnectionHost{
		Connection:   amqpConn,
		ConnectionID: connectionID,
		closeErrors:  make(chan *amqp.Error, 1),
	}

	amqpConn.NotifyClose(ch.closeErrors)

	return connectionHost, nil
}

// NewConnectionHostWithTLS creates a simple ConnectionHost wrapper for management by end-user developer.
func (ch *ConnectionHost) NewConnectionHostWithTLS(
	certServerName string,
	connectionID uint64,
	tlsConfig *tls.Config,
	retryCount uint32) (*ConnectionHost, error) {

	var amqpConn *amqp.Connection
	var err error

	for i := retryCount + 1; i > 0; i-- {
		amqpConn, err = amqp.DialTLS("amqps://"+certServerName, tlsConfig)
		if err != nil {
			time.Sleep(1 * time.Second)
		}
	}

	if amqpConn == nil {
		return nil, fmt.Errorf("opening connection retries exhausted [last err: %s]", err)
	}

	connectionHost := &ConnectionHost{
		Connection:   amqpConn,
		ConnectionID: connectionID,
	}

	amqpConn.NotifyClose(ch.closeErrors)

	return connectionHost, nil
}

// CloseErrors allow you to listen for amqp.Error messages.
func (ch *ConnectionHost) CloseErrors() <-chan *amqp.Error {
	return ch.closeErrors
}
