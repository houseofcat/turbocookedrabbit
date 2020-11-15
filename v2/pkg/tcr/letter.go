package tcr

import (
	"github.com/google/uuid"
	"github.com/streadway/amqp"
)

// Letter contains the message body and address of where things are going.
type Letter struct {
	LetterID   uuid.UUID
	RetryCount uint32
	Body       []byte
	Envelope   *Envelope
}

// Envelope contains all the address details of where a letter is going.
type Envelope struct {
	Exchange     string
	RoutingKey   string
	ContentType  string
	Mandatory    bool
	Immediate    bool
	Headers      amqp.Table
	DeliveryMode uint8
}

// WrappedBody is to go inside a Letter struct with indications of the body of data being modified (ex., compressed).
type WrappedBody struct {
	LetterID       uuid.UUID   `json:"LetterID"`
	Body           *ModdedBody `json:"Body"`
	LetterMetadata string      `json:"LetterMetadata"`
}

// ModdedBody is a payload with modifications and indicators of what was modified.
type ModdedBody struct {
	Encrypted   bool   `json:"Encrypted"`
	EType       string `json:"EncryptionType,omitempty"`
	Compressed  bool   `json:"Compressed"`
	CType       string `json:"CompressionType,omitempty"`
	UTCDateTime string `json:"UTCDateTime"`
	Data        []byte `json:"Data"`
}
