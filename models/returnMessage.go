package models

import (
	"time"

	"github.com/streadway/amqp"
)

// ReturnMessage allow for you to replay a message that was returned.
type ReturnMessage struct {
	ReplyCode  uint16 // reason
	ReplyText  string // description
	Exchange   string // basic.publish exchange
	RoutingKey string // basic.publish routing key

	// Properties
	ContentType     string                 // MIME content type
	ContentEncoding string                 // MIME content encoding
	Headers         map[string]interface{} // Application or header exchange table
	DeliveryMode    uint8                  // queue implementation use - non-persistent (1) or persistent (2)
	Priority        uint8                  // queue implementation use - 0 to 9
	CorrelationID   string                 // application use - correlation identifier
	ReplyTo         string                 // application use - address to to reply to (ex: RPC)
	Expiration      string                 // implementation use - message expiration spec
	MessageID       string                 // application use - message identifier
	Timestamp       time.Time              // application use - message timestamp
	Type            string                 // application use - message type name
	UserID          string                 // application use - creating user id
	AppID           string                 // application use - creating application

	Body []byte
}

// NewReturnMessage creates a new ReturnMessage.
func NewReturnMessage(amqpReturn *amqp.Return) *ReturnMessage {

	return &ReturnMessage{
		ReplyCode:       amqpReturn.ReplyCode,
		ReplyText:       amqpReturn.ReplyText,
		Exchange:        amqpReturn.Exchange,
		RoutingKey:      amqpReturn.RoutingKey,
		ContentType:     amqpReturn.ContentType,
		ContentEncoding: amqpReturn.ContentEncoding,
		Headers:         amqpReturn.Headers,
		DeliveryMode:    amqpReturn.DeliveryMode,
		Priority:        amqpReturn.Priority,
		CorrelationID:   amqpReturn.CorrelationId,
		ReplyTo:         amqpReturn.ReplyTo,
		Expiration:      amqpReturn.Expiration,
		MessageID:       amqpReturn.MessageId,
		Timestamp:       amqpReturn.Timestamp,
		Type:            amqpReturn.Type,
		UserID:          amqpReturn.UserId,
		AppID:           amqpReturn.AppId,
	}
}
