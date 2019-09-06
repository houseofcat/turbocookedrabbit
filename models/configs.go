package models

// RabbitSeasoning represents the configuration values.
type RabbitSeasoning struct {
	PoolConfig      *PoolConfig                `json:"PoolConfig"`
	TLSConfig       *TLSConfig                 `json:"TLSConfig"`
	ConsumerConfigs map[string]*ConsumerConfig `json:"ConsumerConfigs"`
}

// PoolConfig represents settings for creating/configuring the ConnectionPool.
type PoolConfig struct {
	URI                  string `json:"URI"`
	ConnectionRetryCount uint32 `json:"ConnectionRetryCount"`
	ConnectionCount      int64  `json:"ConnectionCount"`
	ChannelRetryCount    uint32 `json:"ChannelRetryCount"`
	ChannelCount         int64  `json:"ChannelCount"`
	AckChannelCount      int64  `json:"AckChannelCount"`
	BreakOnError         bool   `json:"BreakOnError"`
	GlobalQosCount       int    `json:"GlobalQosCount"` // Leave at 0 if you want to ignore them.
	GlobalQosSize        int    `json:"GlobalQosSize"`  // Leave at 0 if you want to ignore them.
}

// TLSConfig represents settings for configuring TLS.
type TLSConfig struct {
	EnableTLS         bool   `json:"EnableTLS"`
	PEMCertLocation   string `json:"PEMCertLocation"`
	LocalCertLocation string `json:"LocalCertLocation"`
	CertServerName    string `json:"CertServerName"`
}

// ConsumerConfig represents settings for configuring a consumer with ease.
type ConsumerConfig struct {
	QueueName        string                 `json:"QueueName"`
	ConsumerName     string                 `json:"ConsumerName"`
	AutoAck          bool                   `json:"AutoAck"`
	Exclusive        bool                   `json:"Exclusive"`
	NoWait           bool                   `json:"NoWait"`
	Args             map[string]interface{} `json:"Args"`
	QosCountOverride int                    `json:"QosCountOverride"` // if zero ignored
	QosSizeOverride  int                    `json:"QosSizeOverride"`  // if zero ignored
	MessageBuffer    uint32                 `json:"MessageBuffer"`
	ErrorBuffer      uint32                 `json:"ErrorBuffer"`
}

// TopologyConfig allows you to build a simple toplogy from Json.
type TopologyConfig struct {
}
