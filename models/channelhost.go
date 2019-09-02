package models

import (
	"errors"
	"fmt"
	"time"

	"github.com/streadway/amqp"
)

// ChannelHost is an internal representation of amqp.Connection.
type ChannelHost struct {
	Channel          *amqp.Channel
	Connection       *amqp.Connection
	ChannelID        uint64
	ConnectionClosed func() bool // super unreliable
}

// NewChannelHost creates a simple ConnectionHost wrapper for management by end-user developer.
func (ch *ChannelHost) NewChannelHost(
	amqpConn *amqp.Connection,
	channelID uint64,
	retryCount int32) (*ChannelHost, error) {

	if amqpConn.IsClosed() {
		return nil, errors.New("can not open a channel - connection is already closed")
	}

	var amqpChan *amqp.Channel
	var err error

	for i := retryCount; i > 0; i-- {
		amqpChan, err = amqpConn.Channel()
		if err != nil {
			time.Sleep(50 * time.Millisecond)
		}
	}
	if amqpChan == nil {
		return nil, fmt.Errorf("opening channel retries exhausted [last err: %s]", err)
	}

	channelHost := &ChannelHost{
		Channel:          amqpChan,
		ChannelID:        channelID,
		ConnectionClosed: amqpConn.IsClosed,
	}

	return channelHost, nil
}
