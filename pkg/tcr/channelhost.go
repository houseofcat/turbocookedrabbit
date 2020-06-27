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
	connHost      *ConnectionHost
}

// NewChannelHost creates a simple ConnectionHost wrapper for management by end-user developer.
func NewChannelHost(
	connHost *ConnectionHost,
	id uint64,
	connectionID uint64,
	ackable, cached bool) (*ChannelHost, error) {

	if connHost.Connection.IsClosed() {
		return nil, errors.New("can't open a channel - connection is already closed")
	}

	chanHost := &ChannelHost{
		ID:            id,
		ConnectionID:  connectionID,
		Ackable:       ackable,
		CachedChannel: cached,
		Confirmations: make(chan amqp.Confirmation, 1000),
		Errors:        make(chan *amqp.Error, 1000),
		connHost:      connHost,
	}

	err := chanHost.Connect()
	if err != nil {
		return nil, err
	}

	if ackable {
		err = chanHost.Channel.Confirm(false)
		if err != nil {
			return nil, err
		}
	}

	return chanHost, nil
}

// Close allows for manual close of Amqp Channel kept internally.
func (ch *ChannelHost) Close() {
	ch.Channel.Close()
}

// Connect tries to create (or re-recreate) the channel from the ConnectionHost its attached to.
func (ch *ChannelHost) Connect() error {
	var err error
	ch.Channel, err = ch.connHost.Connection.Channel()
	if err != nil {
		return err
	}

	ch.flush()

	ch.Channel.NotifyClose(ch.Errors)
	ch.Channel.NotifyPublish(ch.Confirmations)
	return nil
}

// Flush removes all previous errors and confirmations pending to process.
func (ch *ChannelHost) flush() {
	for {
		select {
		case <-ch.Errors:
		case <-ch.Confirmations:
		default:
			return
		}
	}
}

// FlushConfirms removes all previous confirmations pending processing.
func (ch *ChannelHost) FlushConfirms() {
	for {
		select {
		case <-ch.Confirmations:
		default:
			return
		}
	}
}
