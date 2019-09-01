package models

// RabbitSeasoning represents the configuration values.
type RabbitSeasoning struct {
	Pools     *Pools     `json:"Pools"`
	TLSConfig *TLSConfig `json:"TLSConfig"`
}

// Pools represents settings for creating/configuring the ConnectionPool.
type Pools struct {
	URI                  string `json:"URI"`
	ConnectionRetryCount int32  `json:"ConnectionRetryCount"`
	ConnectionCount      int64  `json:"ConnectionCount"`
	ChannelRetryCount    int32  `json:"ChannelRetryCount"`
	ChannelCount         int64  `json:"ChannelCount"`
	BreakOnError         bool   `json:"BreakOnError"`
}

// TLSConfig represents settings for configuring TLS.
type TLSConfig struct {
	EnableTLS         bool   `json:"EnableTLS"`
	PEMCertLocation   string `json:"PEMCertLocation"`
	LocalCertLocation string `json:"LocalCertLocation"`
	CertServerName    string `json:"CertServerName"`
}
