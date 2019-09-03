package models

import (
	"errors"

	"github.com/streadway/amqp"
)

// Message allow for you to acknowledge, after processing the payload, by its RabbitMQ tag and Channel pointer.
type Message struct {
	IsAckable  bool
	Body       []byte
	MessageTag uint64
	Channel    *amqp.Channel
}

// Acknowledge allows for you to acknowledge message on the original channel it was received.
// Will fail if channel is closed and this is by design per RabbitMQ server.
// Can't ack from a different channel.
func (msg *Message) Acknowledge() error {
	if !msg.IsAckable {
		return errors.New("can't acknowledge, not an ackable message")
	}

	if msg.Channel == nil {
		return errors.New("can't acknowledge, internal channel is nil")
	}

	return msg.Channel.Ack(msg.MessageTag, false)
}

// Nack allows for you to negative acknowledge message on the original channel it was received.
// Will fail if channel is closed and this is by design per RabbitMQ server.
func (msg *Message) Nack(requeue bool) error {
	if !msg.IsAckable {
		return errors.New("can't nack, not an ackable message")
	}

	if msg.Channel == nil {
		return errors.New("can't nack, internal channel is nil")
	}

	return msg.Channel.Nack(msg.MessageTag, false, requeue)
}

// Reject allows for you to reject on the original channel it was received.
// Will fail if channel is closed and this is by design per RabbitMQ server.
func (msg *Message) Reject(requeue bool) error {
	if !msg.IsAckable {
		return errors.New("can't reject, not an ackable message")
	}

	if msg.Channel == nil {
		return errors.New("can't reject, internal channel is nil")
	}

	return msg.Channel.Reject(msg.MessageTag, requeue)
}
