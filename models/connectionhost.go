package models

import "github.com/streadway/amqp"

// ConnectionHost is an internal representation of amqp.Connection.
type ConnectionHost struct {
	Connection   *amqp.Connection
	ConnectionID uint64
}
