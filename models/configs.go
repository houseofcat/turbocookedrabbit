package models

// RabbitSeasoning represents the configuration values.
type RabbitSeasoning struct {
	Pools     *Pools     `json:"ConnectionFactory"`
	TLSConfig *TLSConfig `json:"TLSConfig"`
}

// Pools represents settings for creating/configuring the ConnectionPool.
type Pools struct {
	URI             string `json:"URI"`
	ConnectionCount int    `json:"ConnectionCount"`
	ChannelCount    int    `json:"ChannelCount"`
}

// TLSConfig represents settings for configuring TLS.
type TLSConfig struct {
	EnableTLS         bool   `json:"EnableTLS"`
	PEMCertLocation   string `json:"PEMCertLocation"`
	LocalCertLocation string `json:"LocalCertLocation"`
	CertServerName    string `json:"CertServerName"`
}
