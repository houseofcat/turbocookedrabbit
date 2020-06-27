package tcr

import (
	"crypto/tls"
	"time"

	"github.com/streadway/amqp"
)

// ConnectionHost is an internal representation of amqp.Connection.
type ConnectionHost struct {
	Connection         *amqp.Connection
	ConnectionID       uint64
	CachedChannelCount uint64
	uri                string
	connectionName     string
	heartbeatInterval  time.Duration
	connectionTimeout  time.Duration
	tlsConfig          *TLSConfig
	Errors             chan *amqp.Error
	Blockers           chan amqp.Blocking
}

// NewConnectionHost creates a simple ConnectionHost wrapper for management by end-user developer.
func NewConnectionHost(
	uri string,
	connectionName string,
	connectionID uint64,
	heartbeatInterval time.Duration,
	connectionTimeout time.Duration,
	tlsConfig *TLSConfig) (*ConnectionHost, error) {

	connHost := &ConnectionHost{
		uri:               uri,
		connectionName:    connectionName,
		ConnectionID:      connectionID,
		heartbeatInterval: heartbeatInterval,
		connectionTimeout: connectionTimeout,
		tlsConfig:         tlsConfig,
		Errors:            make(chan *amqp.Error, 1000),
		Blockers:          make(chan amqp.Blocking, 1000),
	}

	err := connHost.Connect()
	if err != nil {
		return nil, err
	}

	return connHost, nil
}

// Connect tries to connect (or reconnect) to the provided properties of the host.
func (ch *ConnectionHost) Connect() error {
	var amqpConn *amqp.Connection
	var actualTLSConfig *tls.Config
	var err error

	if ch.tlsConfig != nil && ch.tlsConfig.EnableTLS {

		actualTLSConfig, err = CreateTLSConfig(
			ch.tlsConfig.PEMCertLocation,
			ch.tlsConfig.LocalCertLocation)
		if err != nil {
			return err
		}
	}

	if actualTLSConfig == nil {
		amqpConn, err = amqp.DialConfig(ch.uri, amqp.Config{
			Heartbeat: ch.heartbeatInterval,
			Dial:      amqp.DefaultDial(ch.connectionTimeout),
			Properties: amqp.Table{
				"connection_name": ch.connectionName,
			},
		})
	} else {
		amqpConn, err = amqp.DialConfig("amqps://"+ch.tlsConfig.CertServerName, amqp.Config{
			Heartbeat:       ch.heartbeatInterval,
			Dial:            amqp.DefaultDial(ch.connectionTimeout),
			TLSClientConfig: actualTLSConfig,
			Properties: amqp.Table{
				"connection_name": ch.connectionName,
			},
		})
	}
	if err != nil {
		return err
	}

	ch.Connection = amqpConn

	ch.flush()

	ch.Connection.NotifyClose(ch.Errors)
	ch.Connection.NotifyBlocked(ch.Blockers)

	return nil
}

// Flush removes all previous errors and blockers still pending processing.
func (ch *ConnectionHost) flush() {
	for {
		select {
		case <-ch.Errors:
		case <-ch.Blockers:
		default:
			return
		}
	}
}
