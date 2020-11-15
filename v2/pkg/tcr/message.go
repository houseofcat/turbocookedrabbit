package tcr

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/streadway/amqp"
)

// PublishReceipt is a way to monitor publishing success and to initiate a retry when using async publishing.
type PublishReceipt struct {
	LetterID     uuid.UUID
	FailedLetter *Letter
	Success      bool
	Error        error
}

// ToString allows you to quickly log the PublishReceipt struct as a string.
func (not *PublishReceipt) ToString() string {
	if not.Success {
		return fmt.Sprintf("[LetterID: %s] - Publish successful.\r\n", not.LetterID.String())
	}

	return fmt.Sprintf("[LetterID: %s] - Publish failed.\r\nError: %s\r\n", not.LetterID.String(), not.Error.Error())
}

// ReceivedMessage allow for you to acknowledge, after processing the received payload, by its RabbitMQ tag and Channel pointer.
type ReceivedMessage struct {
	IsAckable     bool
	Body          []byte
	MessageID     string // LetterID
	ApplicationID string
	PublishDate   string
	Delivery      amqp.Delivery // Access everything.
}

// NewReceivedMessage creates a new ReceivedMessage.
func NewReceivedMessage(
	isAckable bool,
	delivery amqp.Delivery) *ReceivedMessage {

	return &ReceivedMessage{
		IsAckable:     isAckable,
		Body:          delivery.Body,
		MessageID:     delivery.MessageId,
		ApplicationID: delivery.AppId,
		PublishDate:   JSONUtcTimestampFromTime(delivery.Timestamp),
		Delivery:      delivery,
	}
}

// Acknowledge allows for you to acknowledge message on the original channel it was received.
// Will fail if channel is closed and this is by design per RabbitMQ server.
// Can't ack from a different channel.
func (msg *ReceivedMessage) Acknowledge() error {
	if !msg.IsAckable {
		return errors.New("can't acknowledge, not an ackable message")
	}

	if msg.Delivery.Acknowledger == nil {
		return errors.New("can't acknowledge, internal channel is nil")
	}

	return msg.Delivery.Acknowledger.Ack(msg.Delivery.DeliveryTag, false)
}

// Nack allows for you to negative acknowledge message on the original channel it was received.
// Will fail if channel is closed and this is by design per RabbitMQ server.
func (msg *ReceivedMessage) Nack(requeue bool) error {
	if !msg.IsAckable {
		return errors.New("can't nack, not an ackable message")
	}

	if msg.Delivery.Acknowledger == nil {
		return errors.New("can't nack, internal channel is nil")
	}

	return msg.Delivery.Acknowledger.Nack(msg.Delivery.DeliveryTag, false, requeue)
}

// Reject allows for you to reject on the original channel it was received.
// Will fail if channel is closed and this is by design per RabbitMQ server.
func (msg *ReceivedMessage) Reject(requeue bool) error {
	if !msg.IsAckable {
		return errors.New("can't reject, not an ackable message")
	}

	if msg.Delivery.Acknowledger == nil {
		return errors.New("can't reject, internal channel is nil")
	}

	return msg.Delivery.Acknowledger.Reject(msg.Delivery.DeliveryTag, requeue)
}

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

// Error allows you to quickly log the ErrorMessage struct as a string.
func (em *ErrorMessage) Error() string {
	return fmt.Sprintf("[ErrorCode: %d] Reason: %s \r\n[Server Initiated: %v]\r\n[Recoverable: %v]\r\n", em.Code, em.Reason, em.Server, em.Recover)
}

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

// PublishConfirmation aids in guaranteed Deliverability.
type PublishConfirmation struct {
	DeliveryTag uint64 // Delivery Tag Id
	Acked       bool   // Acked Serverside
}

// NewPublishConfirmation creates a new PublishConfirmation.
func NewPublishConfirmation(confirmation *amqp.Confirmation) *PublishConfirmation {

	return &PublishConfirmation{
		DeliveryTag: confirmation.DeliveryTag,
		Acked:       confirmation.Ack,
	}
}
