package tcr

import (
	"errors"
	"sync"

	"github.com/streadway/amqp"
)

// ChannelHost is an internal representation of amqp.Channel.
type ChannelHost struct {
	Channel       *amqp.Channel
	ID            uint64
	ConnectionID  uint64
	Ackable       bool
	CachedChannel bool
	Confirmations chan amqp.Confirmation
	Errors        chan *amqp.Error
	connHost      *ConnectionHost
	chanLock      *sync.Mutex
}

// NewChannelHost creates a simple ChannelHost wrapper for management by end-user developer.
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
		connHost:      connHost,
		chanLock:      &sync.Mutex{},
	}

	err := chanHost.MakeChannel()
	if err != nil {
		return nil, err
	}

	return chanHost, nil
}

// Close allows for manual close of Amqp Channel kept internally.
func (ch *ChannelHost) Close() {
	ch.Channel.Close()
}

// MakeChannel tries to create (or re-create) the channel from the ConnectionHost its attached to.
func (ch *ChannelHost) MakeChannel() (err error) {
	ch.chanLock.Lock()
	defer ch.chanLock.Unlock()

	ch.Channel, err = ch.connHost.Connection.Channel()
	if err != nil {
		return err
	}

	if ch.Ackable {
		err = ch.Channel.Confirm(false)
		if err != nil {
			return err
		}

		ch.Confirmations = make(chan amqp.Confirmation, 100)
		ch.Channel.NotifyPublish(ch.Confirmations)
	}

	ch.Errors = make(chan *amqp.Error, 100)
	ch.Channel.NotifyClose(ch.Errors)

	return nil
}

// FlushConfirms removes all previous confirmations pending processing.
func (ch *ChannelHost) FlushConfirms() {
	ch.chanLock.Lock()
	defer ch.chanLock.Unlock()

	for {
		if ch.connHost.Connection.IsClosed() {
			return
		}

		// Some weird use case where the Channel is being flooded with confirms after connection disruption
		// It lead to an infinite loop when this method was called.
		select {
		case <-ch.Confirmations:
			return // return prevents the infinite loop here
		default:
			return
		}
	}
}

// PauseForFlowControl allows you to wait and sleep while receiving flow control messages.
func (ch *ChannelHost) PauseForFlowControl() {

	ch.connHost.PauseOnFlowControl()
}
