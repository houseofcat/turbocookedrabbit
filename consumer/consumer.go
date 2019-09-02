package consumer

// Consumer receives messages from a RabbitMQ location.
type Consumer struct {
}

// NewConsumer creates a new Consumer to receive messages from.
func NewConsumer() *Consumer {
	return &Consumer{}

}
