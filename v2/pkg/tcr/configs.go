package tcr

// RabbitSeasoning represents the configuration values.
type RabbitSeasoning struct {
	EncryptionConfig  *EncryptionConfig          `json:"EncryptionConfig"`
	CompressionConfig *CompressionConfig         `json:"CompressionConfig"`
	PoolConfig        *PoolConfig                `json:"PoolConfig"`
	ConsumerConfigs   map[string]*ConsumerConfig `json:"ConsumerConfigs"`
	PublisherConfig   *PublisherConfig           `json:"PublisherConfig"`
}

// PoolConfig represents settings for creating/configuring pools.
type PoolConfig struct {
	ConnectionName       string     `json:"ConnectionName"`
	URI                  string     `json:"URI"`
	Heartbeat            uint32     `json:"Heartbeat"`
	ConnectionTimeout    uint32     `json:"ConnectionTimeout"`
	SleepOnErrorInterval uint32     `json:"SleepOnErrorInterval"` // sleep length on errors
	MaxConnectionCount   uint64     `json:"MaxConnectionCount"`   // number of connections to create in the pool
	MaxCacheChannelCount uint64     `json:"MaxCacheChannelCount"` // number of channels to be cached in the pool
	TLSConfig            *TLSConfig `json:"TLSConfig"`            // TLS settings for connection with AMQPS.
}

// TLSConfig represents settings for configuring TLS.
type TLSConfig struct {
	EnableTLS         bool   `json:"EnableTLS"` // Use TLSConfig to create connections with AMQPS uri.
	PEMCertLocation   string `json:"PEMCertLocation"`
	LocalCertLocation string `json:"LocalCertLocation"`
	CertServerName    string `json:"CertServerName"`
}

// ConsumerConfig represents settings for configuring a consumer with ease.
type ConsumerConfig struct {
	Enabled              bool                   `json:"Enabled"`
	QueueName            string                 `json:"QueueName"`
	ConsumerName         string                 `json:"ConsumerName"`
	AutoAck              bool                   `json:"AutoAck"`
	Exclusive            bool                   `json:"Exclusive"`
	NoWait               bool                   `json:"NoWait"`
	Args                 map[string]interface{} `json:"Args"`
	QosCountOverride     int                    `json:"QosCountOverride"`     // if zero ignored
	SleepOnErrorInterval uint32                 `json:"SleepOnErrorInterval"` // sleep on error
	SleepOnIdleInterval  uint32                 `json:"SleepOnIdleInterval"`  // sleep on idle
}

// PublisherConfig represents settings for configuring global settings for all Publishers with ease.
type PublisherConfig struct {
	AutoAck                bool   `json:"AutoAck"`
	SleepOnIdleInterval    uint32 `json:"SleepOnIdleInterval"`
	SleepOnErrorInterval   uint32 `json:"SleepOnErrorInterval"`
	PublishTimeOutInterval uint32 `json:"PublishTimeOutInterval"`
	MaxRetryCount          uint32 `json:"MaxRetryCount"`
}

// TopologyConfig allows you to build simple toplogies from a JSON file.
type TopologyConfig struct {
	Exchanges        []*Exchange        `json:"Exchanges"`
	Queues           []*Queue           `json:"Queues"`
	QueueBindings    []*QueueBinding    `json:"QueueBindings"`
	ExchangeBindings []*ExchangeBinding `json:"ExchangeBindings"`
}

// CompressionConfig allows you to configuration symmetric key encryption based on options
type CompressionConfig struct {
	Enabled bool   `json:"Enabled"`
	Type    string `json:"Type,omitempty"`
}

// EncryptionConfig allows you to configuration symmetric key encryption based on options
type EncryptionConfig struct {
	Enabled           bool   `json:"Enabled"`
	Type              string `json:"Type,omitempty"`
	Hashkey           []byte
	TimeConsideration uint32 `json:"TimeConsideration,omitempty"`
	MemoryMultiplier  uint32 `json:""`
	Threads           uint8  `json:"Threads,omitempty"`
}
