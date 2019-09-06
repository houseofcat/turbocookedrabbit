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
	ChannelID        uint64
	ConnectionClosed func() bool // super unreliable
	ErrorMessages    chan *ErrorMessage
	ReturnMessages   chan *ReturnMessage
	closeErrors      chan *amqp.Error
	returnMessages   chan amqp.Return
}

// NewChannelHost creates a simple ConnectionHost wrapper for management by end-user developer.
func (ch *ChannelHost) NewChannelHost(
	amqpConn *amqp.Connection,
	channelID uint64,
	retryCount uint32) (*ChannelHost, error) {

	if amqpConn.IsClosed() {
		return nil, errors.New("can't open a channel - connection is already closed")
	}

	var amqpChan *amqp.Channel
	var err error

	for i := retryCount + 1; i > 0; i-- {
		amqpChan, err = amqpConn.Channel()
		if err != nil {
			time.Sleep(50 * time.Millisecond)
		}
	}
	if amqpChan == nil {
		return nil, fmt.Errorf("opening channel retries exhausted [last err: %s]", err)
	}

	amqpChan.NotifyClose(ch.closeErrors)
	amqpChan.NotifyReturn(ch.returnMessages)

	channelHost := &ChannelHost{
		Channel:          amqpChan,
		ChannelID:        channelID,
		ConnectionClosed: amqpConn.IsClosed,
		ErrorMessages:    make(chan *ErrorMessage, 1),
		ReturnMessages:   make(chan *ReturnMessage, 1),
		closeErrors:      make(chan *amqp.Error, 1),
		returnMessages:   make(chan amqp.Return, 1),
	}

	return channelHost, nil
}

// CloseErrors allow you to listen for amqp.Error messages.
func (ch *ChannelHost) CloseErrors() <-chan *ErrorMessage {
	select {
	case amqpError := <-ch.closeErrors:
		ch.ErrorMessages <- NewErrorMessage(amqpError)

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
