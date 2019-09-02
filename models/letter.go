package models

// Letter contains the message body and address of where things are going.
type Letter struct {
	LetterID uint64
	Body     []byte
	Envelope Envelope
}

// Envelope contains all the address details of where a letter is going.
type Envelope struct {
	Exchange    string
	RoutingKey  string
	ContentType string
	Mandatory   bool
	Immediate   bool
}
