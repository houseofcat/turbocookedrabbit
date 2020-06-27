package tcr

import (
	"errors"

	"github.com/streadway/amqp"
)

// ChannelHost is an internal representation of amqp.Connection.
type ChannelHost struct {
	Channel       *amqp.Channel
	ID            uint64
	ConnectionID  uint64
	Ackable       bool
	CachedChannel bool
	Confirmations chan amqp.Confirmation
	Errors        chan *amqp.Error
}

// NewChannelHost creates a simple ConnectionHost wrapper for management by end-user developer.
func NewChannelHost(
	amqpConn *amqp.Connection,
	id uint64,
	connectionID uint64,
	ackable, cached bool) (*ChannelHost, error) {

	if amqpConn.IsClosed() {
		return nil, errors.New("can't open a channel - connection is already closed")
	}

	amqpChan, err := amqpConn.Channel()
	if err != nil {
		return nil, err
	}

	channelHost := &ChannelHost{
		Channel:       amqpChan,
		ID:            id,
		ConnectionID:  connectionID,
		Ackable:       ackable,
		CachedChannel: cached,
		Confirmations: make(chan amqp.Confirmation, 100),
		Errors:        make(chan *amqp.Error, 100),
	}

	channelHost.Channel.NotifyClose(channelHost.Errors)

	if ackable {
		if err = channelHost.Channel.Confirm(false); err != nil {
			return nil, err
		}
	}

	return channelHost, nil
}

// Close allows for manual close of Amqp Channel kept internally.
func (ch *ChannelHost) Close() {
	ch.Channel.Close()
}
