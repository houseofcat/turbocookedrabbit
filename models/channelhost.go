package models

import (
	"errors"

	"github.com/streadway/amqp"
)

// ChannelHost is an internal representation of amqp.Connection.
type ChannelHost struct {
	Channel        *amqp.Channel
	ChannelID      uint64
	ConnectionID   uint64
	ackable        bool
	ErrorMessages  chan *ErrorMessage
	ReturnMessages chan *ReturnMessage
	closeErrors    chan *amqp.Error
	returnMessages chan amqp.Return
}

// NewChannelHost creates a simple ConnectionHost wrapper for management by end-user developer.
func NewChannelHost(
	amqpConn *amqp.Connection,
	channelID uint64,
	connectionID uint64,
	ackable bool) (*ChannelHost, error) {

	if amqpConn.IsClosed() {
		return nil, errors.New("can't open a channel - connection is already closed")
	}

	amqpChan, err := amqpConn.Channel()
	if err != nil {
		return nil, err
	}

	channelHost := &ChannelHost{
		Channel:        amqpChan,
		ChannelID:      channelID,
		ConnectionID:   connectionID,
		ackable:        ackable,
		ErrorMessages:  make(chan *ErrorMessage, 1),
		ReturnMessages: make(chan *ReturnMessage, 1),
		closeErrors:    make(chan *amqp.Error, 1),
		returnMessages: make(chan amqp.Return, 1),
	}

	channelHost.Channel.NotifyClose(channelHost.closeErrors)
	channelHost.Channel.NotifyReturn(channelHost.returnMessages)

	return channelHost, nil
}

// CloseErrors allow you to listen for amqp.Error messages.
func (ch *ChannelHost) CloseErrors() <-chan *ErrorMessage {
	select {
	case amqpError := <-ch.closeErrors:
		if amqpError != nil { // received a nil during testing
			ch.ErrorMessages <- NewErrorMessage(amqpError)
		}
	default:
		break
	}

	return ch.ErrorMessages
}

// Returns allow you to listen for ReturnMessages.
func (ch *ChannelHost) Returns() <-chan *ReturnMessage {
	select {
	case amqpReturn := <-ch.returnMessages:
		ch.ReturnMessages <- NewReturnMessage(&amqpReturn)

	default:
		break
	}

	return ch.ReturnMessages
}

// IsAckable determines if this host contains an ackable channel.
func (ch *ChannelHost) IsAckable() bool {
	return ch.ackable
}
