package models

import (
	"crypto/tls"

	"github.com/streadway/amqp"
)

// ConnectionHost is an internal representation of amqp.Connection.
type ConnectionHost struct {
	Connection   *amqp.Connection
	ConnectionID uint64
	closeErrors  chan *amqp.Error
}

// NewConnectionHost creates a simple ConnectionHost wrapper for management by end-user developer.
func NewConnectionHost(
	uri string,
	connectionID uint64) (*ConnectionHost, error) {

	var amqpConn *amqp.Connection
	var err error

	amqpConn, err = amqp.Dial(uri)
	if err != nil {
		return nil, err
	}

	connectionHost := &ConnectionHost{
		Connection:   amqpConn,
		ConnectionID: connectionID,
		closeErrors:  make(chan *amqp.Error, 1),
	}

	connectionHost.Connection.NotifyClose(connectionHost.closeErrors)

	return connectionHost, nil
}

// NewConnectionHostWithTLS creates a simple ConnectionHost wrapper for management by end-user developer.
func NewConnectionHostWithTLS(
	certServerName string,
	connectionID uint64,
	tlsConfig *tls.Config) (*ConnectionHost, error) {

	var amqpConn *amqp.Connection
	var err error

	amqpConn, err = amqp.DialTLS("amqps://"+certServerName, tlsConfig)
	if err != nil {
		return nil, err
	}

	connectionHost := &ConnectionHost{
		Connection:   amqpConn,
		ConnectionID: connectionID,
	}

	connectionHost.Connection.NotifyClose(connectionHost.closeErrors)

	return connectionHost, nil
}

// CloseErrors allow you to listen for amqp.Error messages.
func (ch *ConnectionHost) CloseErrors() <-chan *amqp.Error {
	return ch.closeErrors
}
