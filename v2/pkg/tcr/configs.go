package tcr

// RabbitSeasoning represents the configuration values.
type RabbitSeasoning struct {
	EncryptionConfig  *EncryptionConfig          `json:"EncryptionConfig yaml:"EncryptionConfig"`
	CompressionConfig *CompressionConfig         `json:"CompressionConfig yaml:"CompressionConfig"`
	PoolConfig        *PoolConfig                `json:"PoolConfig" yaml:"PoolConfig"`
	ConsumerConfigs   map[string]*ConsumerConfig `json:"ConsumerConfigs" yaml:"ConsumerConfigs"`
	PublisherConfig   *PublisherConfig           `json:"PublisherConfig" yaml:"PublisherConfig"`
}

// PoolConfig represents settings for creating/configuring pools.
type PoolConfig struct {
	ApplicationName      string     `json:"ApplicationName" yaml:"ApplicationName"`
	URI                  string     `json:"URI" yaml:"URI"`
	Heartbeat            uint32     `json:"Heartbeat" yaml:"Heartbeat"`
	ConnectionTimeout    uint32     `json:"ConnectionTimeout" yaml:"ConnectionTimeout"`
	SleepOnErrorInterval uint32     `json:"SleepOnErrorInterval" yaml:"SleepOnErrorInterval"` // sleep length on errors
	MaxConnectionCount   uint64     `json:"MaxConnectionCount" yaml:"MaxConnectionCount"`   // number of connections to create in the pool
	MaxCacheChannelCount uint64     `json:"MaxCacheChannelCount" yaml:"MaxCacheChannelCount"` // number of channels to be cached in the pool
	TLSConfig            *TLSConfig `json:"TLSConfig" yaml:"TLSConfig"`            // TLS settings for connection with AMQPS.
}

// TLSConfig represents settings for configuring TLS.
type TLSConfig struct {
	EnableTLS         bool   `json:"EnableTLS" yaml:"EnableTLS"` // Use TLSConfig to create connections with AMQPS uri.
	PEMCertLocation   string `json:"PEMCertLocation" yaml:"PEMCertLocation"`
	LocalCertLocation string `json:"LocalCertLocation" yaml:"LocalCertLocation"`
	CertServerName    string `json:"CertServerName" yaml:"CertServerName"`
}

// ConsumerConfig represents settings for configuring a consumer with ease.
type ConsumerConfig struct {
	Enabled              bool                   `json:"Enabled" yaml:"Enabled"`
	QueueName            string                 `json:"QueueName" yaml:"QueueName"`
	ConsumerName         string                 `json:"ConsumerName" yaml:"ConsumerName"`
	AutoAck              bool                   `json:"AutoAck" yaml:"AutoAck"`
	Exclusive            bool                   `json:"Exclusive" yaml:"Exclusive"`
	NoWait               bool                   `json:"NoWait" yaml:"NoWait"`
	Args                 map[string]interface{} `json:"Args" yaml:"Args"`
	QosCountOverride     int                    `json:"QosCountOverride" yaml:"QosCountOverride"`     // if zero ignored
	SleepOnErrorInterval uint32                 `json:"SleepOnErrorInterval" yaml:"SleepOnErrorInterval"` // sleep on error
	SleepOnIdleInterval  uint32                 `json:"SleepOnIdleInterval" yaml:"SleepOnIdleInterval"`  // sleep on idle
}

// PublisherConfig represents settings for configuring global settings for all Publishers with ease.
type PublisherConfig struct {
	AutoAck                bool   `json:"AutoAck" yaml:"AutoAck"`
	SleepOnIdleInterval    uint32 `json:"SleepOnIdleInterval" yaml:"SleepOnIdleInterval"`
	SleepOnErrorInterval   uint32 `json:"SleepOnErrorInterval" yaml:"SleepOnErrorInterval"`
	PublishTimeOutInterval uint32 `json:"PublishTimeOutInterval" yaml:"PublishTimeOutInterval"`
	MaxRetryCount          uint32 `json:"MaxRetryCount" yaml:"MaxRetryCount"`
}

// TopologyConfig allows you to build simple toplogies from a JSON file.
type TopologyConfig struct {
	Exchanges        []*Exchange        `json:"Exchanges" yaml:"Exchanges"`
	Queues           []*Queue           `json:"Queues" yaml:"Queues"`
	QueueBindings    []*QueueBinding    `json:"QueueBindings" yaml:"QueueBindings"`
	ExchangeBindings []*ExchangeBinding `json:"ExchangeBindings" yaml:"ExchangeBindings"`
}

// CompressionConfig allows you to configuration symmetric key encryption based on options
type CompressionConfig struct {
	Enabled bool   `json:"Enabled" yaml:"Enabled"`
	Type    string `json:"Type,omitempty" yaml:"Type,omitempty"`
}

// EncryptionConfig allows you to configuration symmetric key encryption based on options
type EncryptionConfig struct {
	Enabled           bool   `json:"Enabled" yaml:"Enabled"`
	Type              string `json:"Type,omitempty" yaml:"Type,omitempty"`
	Hashkey           []byte
	TimeConsideration uint32 `json:"TimeConsideration,omitempty" yaml:"TimeConsideration,omitempty"`
	MemoryMultiplier  uint32 `json:"" yaml:""`
	Threads           uint8  `json:"Threads,omitempty" yaml:"Threads,omitempty"`
}
