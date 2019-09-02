package models

// RabbitSeasoning represents the configuration values.
type RabbitSeasoning struct {
	Pools     *Pools     `json:"Pools"`
	TLSConfig *TLSConfig `json:"TLSConfig"`
}

// Pools represents settings for creating/configuring the ConnectionPool.
type Pools struct {
	URI                  string `json:"URI"`
	ConnectionRetryCount uint32 `json:"ConnectionRetryCount"`
	ConnectionCount      int64  `json:"ConnectionCount"`
	ChannelRetryCount    uint32 `json:"ChannelRetryCount"`
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

// TopologyConfig allows you to build a simple toplogy from Json.
type TopologyConfig struct {
	Exchanges []*Exchange `json:"Exchanges"`
	Queues    []*Queue    `json:"Queues"`
}
