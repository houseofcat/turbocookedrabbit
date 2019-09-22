package models

// CompressionOptions allows you to configuration symmetric key encryption based on options
type CompressionOptions struct {
	Enabled bool   `json:"Enabled"`
	Type    string `json:"Type,omitempty"`
}
