package models

import "github.com/streadway/amqp"

// Exchange allows for you to create Exchange topology.
type Exchange struct {
	Name           string     `json:"Name"`
	Type           string     `json:"Type"` // "direct", "fanout", "topic", "headers"
	PassiveDeclare bool       `json:"PassiveDeclare"`
	Durable        bool       `json:"Durable"`
	AutoDelete     bool       `json:"AutoDelete"`
	InternalOnly   bool       `json:"InternalOnly"`
	NoWait         bool       `json:"NoWait"`
	Args           amqp.Table `json:"Args,omitempty"` // map[string]interface()
}

// Queue allows for you to create Queue topology.
type Queue struct {
	Name           string     `json:"Name"`
	PassiveDeclare bool       `json:"PassiveDeclare"`
	Durable        bool       `json:"Durable"`
	AutoDelete     bool       `json:"AutoDelete"`
	Exclusive      bool       `json:"Exclusive"`
	NoWait         bool       `json:"NoWait"`
	Args           amqp.Table `json:"Args,omitempty"` // map[string]interface()
}

// QueueBinding allows for you to create Bindings between a Queue and Exchange.
type QueueBinding struct {
	QueueName    string     `json:"QueueName"`
	ExchangeName string     `json:"ExchangeName"`
	RoutingKey   string     `json:"RoutingKey"`
	NoWait       bool       `json:"NoWait"`
	Args         amqp.Table `json:"Args,omitempty"` // map[string]interface()
}

// ExchangeBinding allows for you to create Bindings between an Exchange and Exchange.
type ExchangeBinding struct {
	ExchangeName       string     `json:"ExchangeName"`
	ParentExchangeName string     `json:"ParentExchangeName"`
	RoutingKey         string     `json:"RoutingKey"`
	NoWait             bool       `json:"NoWait"`
	Args               amqp.Table `json:"Args,omitempty"` // map[string]interface()
}
