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
		Confirmations: make(chan amqp.Confirmation, 100),
		Errors:        make(chan *amqp.Error, 100),
		connHost:      connHost,
	}

	err := chanHost.MakeChannel()
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

// MakeChannel tries to create (or re-recreate) the channel from the ConnectionHost its attached to.
func (ch *ChannelHost) MakeChannel() (err error) {

	ch.Channel, err = ch.connHost.Connection.Channel()
	if err != nil {
		return err
	}

	ch.Confirmations = make(chan amqp.Confirmation, 100)
	ch.Errors = make(chan *amqp.Error, 100)

	ch.Channel.NotifyClose(ch.Errors)
	ch.Channel.NotifyPublish(ch.Confirmations)
	return nil
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

// PauseForFlowControl allows you to wait till flow control issue is resolved.
func (ch *ChannelHost) PauseForFlowControl() {

	ch.connHost.PauseOnFlowControl()
}
