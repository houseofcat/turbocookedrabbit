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
}

// NewConnectionHost creates a simple ConnectionHost wrapper for management by end-user developer.
func (ch *ConnectionHost) NewConnectionHost(
	uri string,
	connectionID uint64,
	retryCount int) (*ConnectionHost, error) {

	var amqpConn *amqp.Connection
	var err error

	for i := retryCount; i > 0; i-- {
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
	}

	return connectionHost, nil
}

// NewConnectionHostWithTLS creates a simple ConnectionHost wrapper for management by end-user developer.
func (ch *ConnectionHost) NewConnectionHostWithTLS(
	certServerName string,
	connectionID uint64,
	tlsConfig *tls.Config,
	retryCount int) (*ConnectionHost, error) {

	var amqpConn *amqp.Connection
	var err error

	for i := retryCount; i > 0; i-- {
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

	return connectionHost, nil
}
