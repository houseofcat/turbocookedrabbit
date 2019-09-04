package models

import (
	"errors"

	"github.com/streadway/amqp"
)

// Message allow for you to acknowledge, after processing the payload, by its RabbitMQ tag and Channel pointer.
type Message struct {
	IsAckable   bool
	Body        []byte
	deliveryTag uint64
	amqpChan    *amqp.Channel
}

// NewMessage creates a new Message.
func NewMessage(
	isAckable bool,
	body []byte,
	deliveryTag uint64,
	amqpChan *amqp.Channel) *Message {

	return &Message{
		IsAckable:   isAckable,
		Body:        body,
		deliveryTag: deliveryTag,
		amqpChan:    amqpChan,
	}
}

// Acknowledge allows for you to acknowledge message on the original channel it was received.
// Will fail if channel is closed and this is by design per RabbitMQ server.
// Can't ack from a different channel.
func (msg *Message) Acknowledge() error {
	if !msg.IsAckable {
		return errors.New("can't acknowledge, not an ackable message")
	}

	if msg.amqpChan == nil {
		return errors.New("can't acknowledge, internal channel is nil")
	}

	return msg.amqpChan.Ack(msg.deliveryTag, false)
}

// Nack allows for you to negative acknowledge message on the original channel it was received.
// Will fail if channel is closed and this is by design per RabbitMQ server.
func (msg *Message) Nack(requeue bool) error {
	if !msg.IsAckable {
		return errors.New("can't nack, not an ackable message")
	}

	if msg.amqpChan == nil {
		return errors.New("can't nack, internal channel is nil")
	}

	return msg.amqpChan.Nack(msg.deliveryTag, false, requeue)
}

// Reject allows for you to reject on the original channel it was received.
// Will fail if channel is closed and this is by design per RabbitMQ server.
func (msg *Message) Reject(requeue bool) error {
	if !msg.IsAckable {
		return errors.New("can't reject, not an ackable message")
	}

	if msg.amqpChan == nil {
		return errors.New("can't reject, internal channel is nil")
	}

	return msg.amqpChan.Reject(msg.deliveryTag, requeue)
}
