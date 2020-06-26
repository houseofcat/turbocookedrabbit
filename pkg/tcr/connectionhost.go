package tcr

import (
	"crypto/tls"
	"time"

	"github.com/houseofcat/turbocookedrabbit/pkg/utils"
	"github.com/streadway/amqp"
)

// ConnectionHost is an internal representation of amqp.Connection.
type ConnectionHost struct {
	Connection   *amqp.Connection
	ConnectionID uint64
	errors       chan *amqp.Error
	blockers     chan amqp.Blocking
}

// NewConnectionHost creates a simple ConnectionHost wrapper for management by end-user developer.
func NewConnectionHost(
	uri string,
	connectionName string,
	connectionID uint64,
	heartbeatInterval time.Duration,
	connectionTimeout time.Duration,
	tlsConfig *TLSConfig) (*ConnectionHost, error) {

	var amqpConn *amqp.Connection
	var actualTLSConfig *tls.Config
	var err error

	if tlsConfig != nil && tlsConfig.EnableTLS {

		actualTLSConfig, err = utils.CreateTLSConfig(
			tlsConfig.PEMCertLocation,
			tlsConfig.LocalCertLocation)
		if err != nil {
			return nil, err
		}
	}

	if actualTLSConfig == nil {
		amqpConn, err = amqp.DialConfig(uri, amqp.Config{
			Heartbeat: heartbeatInterval,
			Dial:      amqp.DefaultDial(connectionTimeout),
			Properties: amqp.Table{
				"connection_name": connectionName,
			},
		})
	} else {
		amqpConn, err = amqp.DialConfig("amqps://"+tlsConfig.CertServerName, amqp.Config{
			Heartbeat:       heartbeatInterval,
			Dial:            amqp.DefaultDial(connectionTimeout),
			TLSClientConfig: actualTLSConfig,
			Properties: amqp.Table{
				"connection_name": connectionName,
			},
		})
	}
	if err != nil {
		return nil, err
	}

	connectionHost := &ConnectionHost{
		Connection:   amqpConn,
		ConnectionID: connectionID,
		errors:       make(chan *amqp.Error),
		blockers:     make(chan amqp.Blocking),
	}

	connectionHost.Connection.NotifyClose(connectionHost.errors)
	connectionHost.Connection.NotifyBlocked(connectionHost.blockers)

	return connectionHost, nil
}

// Errors allow you to listen for amqp.Error messages (connection closed).
func (ch *ConnectionHost) Errors() <-chan *amqp.Error {
	return ch.errors
}

// Blockers allow you to listen for amqp.Blocking messages (flow control).
func (ch *ConnectionHost) Blockers() <-chan amqp.Blocking {
	return ch.blockers
}
