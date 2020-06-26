package tcr

// Letter contains the message body and address of where things are going.
type Letter struct {
	LetterID   uint64
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
	Headers      map[string]interface{}
	DeliveryMode uint8
}

// ModdedLetter is a letter with a modified body and indicators of what was done to it.
type ModdedLetter struct {
	LetterID       uint64      `json:"LetterID"`
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
