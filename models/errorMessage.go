package models

import "github.com/streadway/amqp"

// ErrorMessage allow for you to replay a message that was returned.
type ErrorMessage struct {
	Code    int
	Reason  string
	Server  bool
	Recover bool
}

// NewErrorMessage creates a new ErrorMessage.
func NewErrorMessage(amqpError *amqp.Error) *ErrorMessage {

	return &ErrorMessage{
		Code:    amqpError.Code,
		Reason:  amqpError.Reason,
		Server:  amqpError.Server,
		Recover: amqpError.Recover,
	}
}
