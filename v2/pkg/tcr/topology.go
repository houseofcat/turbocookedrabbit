package tcr

import "github.com/streadway/amqp"

// Exchange allows for you to create Exchange topology.
type Exchange struct {
	Name           string     `json:"Name" yaml:"Name"`
	Type           string     `json:"Type" yaml:"Type"` // "direct", "fanout", "topic", "headers"
	PassiveDeclare bool       `json:"PassiveDeclare" yaml:"PassiveDeclare"`
	Durable        bool       `json:"Durable" yaml:"Durable"`
	AutoDelete     bool       `json:"AutoDelete" yaml:"AutoDelete"`
	InternalOnly   bool       `json:"InternalOnly" yaml:"InternalOnly"`
	NoWait         bool       `json:"NoWait" yaml:"NoWait"`
	Args           amqp.Table `json:"Args,omitempty" yaml:"Args,omitempty"` // map[string]interface()
}

// Queue allows for you to create Queue topology.
type Queue struct {
	Name           string     `json:"Name" yaml:"Name"`
	PassiveDeclare bool       `json:"PassiveDeclare" yaml:"PassiveDeclare"`
	Durable        bool       `json:"Durable" yaml:"Durable"`
	AutoDelete     bool       `json:"AutoDelete" yaml:"AutoDelete"`
	Exclusive      bool       `json:"Exclusive" yaml:"Exclusive"`
	NoWait         bool       `json:"NoWait" yaml:"NoWait"`
	Type           string     `json:"Type" yaml:"Type"`           // classic or quorum, type of quorum disregards exclusive and enables durable properties when building from config
	Args           amqp.Table `json:"Args,omitempty" yaml:"Args,omitempty"` // map[string]interface()
}

// QueueBinding allows for you to create Bindings between a Queue and Exchange.
type QueueBinding struct {
	QueueName    string     `json:"QueueName" yaml:"QueueName"`
	ExchangeName string     `json:"ExchangeName" yaml:"ExchangeName"`
	RoutingKey   string     `json:"RoutingKey" yaml:"RoutingKey"`
	NoWait       bool       `json:"NoWait" yaml:"NoWait"`
	Args         amqp.Table `json:"Args,omitempty" yaml:"Args,omitempty"` // map[string]interface()
}

// ExchangeBinding allows for you to create Bindings between an Exchange and Exchange.
type ExchangeBinding struct {
	ExchangeName       string     `json:"ExchangeName" yaml:"ExchangeName"`
	ParentExchangeName string     `json:"ParentExchangeName" yaml:"ParentExchangeName"`
	RoutingKey         string     `json:"RoutingKey" yaml:"RoutingKey"`
	NoWait             bool       `json:"NoWait" yaml:"NoWait"`
	Args               amqp.Table `json:"Args,omitempty" yaml:"Args,omitempty"` // map[string]interface()
}
