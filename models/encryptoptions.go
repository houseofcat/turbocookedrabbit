package models

// EncryptOptions allows you to configuration symmetric key encryption based on options
type EncryptOptions struct {
	Enabled           bool   `json:"Enabled"`
	Type              string `json:"Type,omitempty"`
	Hashkey           []byte
	TimeConsideration uint32 `json:"TimeConsideration,omitempty"`
	Threads           uint8  `json:"Threads,omitempty"`
}
