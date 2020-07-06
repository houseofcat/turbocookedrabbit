package topology

import (
	"errors"

	"github.com/houseofcat/turbocookedrabbit/models"
	"github.com/houseofcat/turbocookedrabbit/pools"
	"github.com/streadway/amqp"
)

// Topologer allows you to build RabbitMQ topology backed by a ChannelPool.
type Topologer struct {
	channelPool *pools.ChannelPool
}

// NewTopologer builds you a new Topologer.
func NewTopologer(channelPool *pools.ChannelPool) (*Topologer, error) {

	if channelPool == nil {
		return nil, errors.New("channelpool can't be nil")
	}

	if !channelPool.Initialized {
		if err := channelPool.Initialize(); err != nil {
			return nil, err
		}
	}

	return &Topologer{
		channelPool: channelPool,
	}, nil
}

// BuildToplogy builds a topology based on a ToplogyConfig - stops on first error.
func (top *Topologer) BuildToplogy(config *models.TopologyConfig, ignoreErrors bool) error {
	err := top.BuildExchanges(config.Exchanges, ignoreErrors)
	if err != nil && !ignoreErrors {
		return err
	}

	err = top.BuildQueues(config.Queues, ignoreErrors)
	if err != nil && !ignoreErrors {
		return err
	}

	err = top.BindQueues(config.QueueBindings, ignoreErrors)
	if err != nil && !ignoreErrors {
		return err
	}

	err = top.BindExchanges(config.ExchangeBindings, ignoreErrors)
	if err != nil && !ignoreErrors {
		return err
	}

	return nil
}

// BuildExchanges loops through and builds Exchanges - stops on first error.
func (top *Topologer) BuildExchanges(exchanges []*models.Exchange, ignoreErrors bool) error {
	if len(exchanges) == 0 {
		return nil
	}

	for _, exchange := range exchanges {
		err := top.CreateExchangeFromConfig(exchange)
		if err != nil && !ignoreErrors {
			return err
		}
	}

	return nil
}

// BuildQueues loops through and builds Queues - stops on first error.
func (top *Topologer) BuildQueues(queues []*models.Queue, ignoreErrors bool) error {
	if len(queues) == 0 {
		return nil
	}

	for _, queue := range queues {
		err := top.CreateQueueFromConfig(queue)
		if err != nil && !ignoreErrors {
			return err
		}
	}

	return nil
}

// BindQueues loops through and binds Queues to Exchanges - stops on first error.
func (top *Topologer) BindQueues(bindings []*models.QueueBinding, ignoreErrors bool) error {
	if len(bindings) == 0 {
		return nil
	}

	for _, queueBinding := range bindings {
		err := top.QueueBind(queueBinding)
		if err != nil && !ignoreErrors {
			return err
		}
	}

	return nil
}

// BindExchanges loops thrrough and binds Exchanges to Exchanges - stops on first error.
func (top *Topologer) BindExchanges(bindings []*models.ExchangeBinding, ignoreErrors bool) error {
	if len(bindings) == 0 {
		return nil
	}

	for _, exchangeBinding := range bindings {
		err := top.ExchangeBind(exchangeBinding)
		if err != nil && !ignoreErrors {
			return err
		}
	}

	return nil
}

// CreateExchange builds an Exchange topology.
func (top *Topologer) CreateExchange(
	exchangeName string,
	exchangeType string,
	passiveDeclare, durable, autoDelete, internal, noWait bool,
	args map[string]interface{}) error {

	chanHost, err := top.channelPool.GetChannel()
	if err != nil {
		return err
	}

	defer top.channelPool.ReturnChannel(chanHost, false)

	if passiveDeclare {
		err = chanHost.Channel.ExchangeDeclarePassive(exchangeName, exchangeType, durable, autoDelete, internal, noWait, amqp.Table(args))
		if err != nil {
			top.channelPool.FlagChannel(chanHost.ChannelID)
			return err
		}

		return nil
	}

	err = chanHost.Channel.ExchangeDeclare(exchangeName, exchangeType, durable, autoDelete, internal, noWait, amqp.Table(args))
	if err != nil {
		top.channelPool.FlagChannel(chanHost.ChannelID)
		return err
	}

	return nil
}

// CreateExchangeFromConfig builds an Exchange toplogy from a config Exchange element.
func (top *Topologer) CreateExchangeFromConfig(exchange *models.Exchange) error {

	chanHost, err := top.channelPool.GetChannel()
	if err != nil {
		return err
	}

	defer top.channelPool.ReturnChannel(chanHost, false)

	if exchange.PassiveDeclare {
		err = chanHost.Channel.ExchangeDeclarePassive(
			exchange.Name,
			exchange.Type,
			exchange.Durable,
			exchange.AutoDelete,
			exchange.InternalOnly,
			exchange.NoWait,
			exchange.Args)

		if err != nil {
			top.channelPool.FlagChannel(chanHost.ChannelID)
			return err
		}

		return nil
	}

	err = chanHost.Channel.ExchangeDeclare(
		exchange.Name,
		exchange.Type,
		exchange.Durable,
		exchange.AutoDelete,
		exchange.InternalOnly,
		exchange.NoWait,
		exchange.Args)

	if err != nil {
		top.channelPool.FlagChannel(chanHost.ChannelID)
		return err
	}

	return nil
}

// ExchangeBind binds an exchange to an Exchange.
func (top *Topologer) ExchangeBind(exchangeBinding *models.ExchangeBinding) error {

	chanHost, err := top.channelPool.GetChannel()
	if err != nil {
		return err
	}

	defer top.channelPool.ReturnChannel(chanHost, false)

	err = chanHost.Channel.ExchangeBind(
		exchangeBinding.ExchangeName,
		exchangeBinding.RoutingKey,
		exchangeBinding.ParentExchangeName,
		exchangeBinding.NoWait,
		exchangeBinding.Args)

	if err != nil {
		top.channelPool.FlagChannel(chanHost.ChannelID)
		return err
	}

	return nil
}

// ExchangeDelete removes the exchange from the server.
func (top *Topologer) ExchangeDelete(
	exchangeName string,
	ifUnused, noWait bool) error {

	chanHost, err := top.channelPool.GetChannel()
	if err != nil {
		return err
	}

	defer top.channelPool.ReturnChannel(chanHost, false)

	err = chanHost.Channel.ExchangeDelete(exchangeName, ifUnused, noWait)
	if err != nil {
		top.channelPool.FlagChannel(chanHost.ChannelID)
		return err
	}

	return nil
}

// ExchangeUnbind removes the binding of an Exchange to an Exchange.
func (top *Topologer) ExchangeUnbind(exchangeName, routingKey, parentExchangeName string, noWait bool, args map[string]interface{}) error {

	chanHost, err := top.channelPool.GetChannel()
	if err != nil {
		return err
	}

	defer top.channelPool.ReturnChannel(chanHost, false)

	err = chanHost.Channel.ExchangeUnbind(
		exchangeName,
		routingKey,
		parentExchangeName,
		noWait,
		amqp.Table(args))

	if err != nil {
		top.channelPool.FlagChannel(chanHost.ChannelID)
		return err
	}

	return nil
}

// CreateQueue builds a Queue topology.
func (top *Topologer) CreateQueue(
	queueName string,
	passiveDeclare bool,
	durable bool,
	autoDelete bool,
	exclusive bool,
	noWait bool,
	args map[string]interface{}) error {

	chanHost, err := top.channelPool.GetChannel()
	if err != nil {
		return err
	}

	defer top.channelPool.ReturnChannel(chanHost, false)

	if passiveDeclare {
		_, err = chanHost.Channel.QueueDeclarePassive(queueName, durable, autoDelete, exclusive, noWait, amqp.Table(args))
		if err != nil {
			top.channelPool.FlagChannel(chanHost.ChannelID)
			return err
		}

		return nil
	}

	_, err = chanHost.Channel.QueueDeclare(queueName, durable, autoDelete, exclusive, noWait, amqp.Table(args))
	if err != nil {
		top.channelPool.FlagChannel(chanHost.ChannelID)
		return err
	}

	return nil
}

// CreateQueueFromConfig builds a Queue topology from a config Exchange element.
func (top *Topologer) CreateQueueFromConfig(queue *models.Queue) error {

	chanHost, err := top.channelPool.GetChannel()
	if err != nil {
		return err
	}

	defer top.channelPool.ReturnChannel(chanHost, false)

	if queue.PassiveDeclare {
		_, err = chanHost.Channel.QueueDeclarePassive(queue.Name, queue.Durable, queue.AutoDelete, queue.Exclusive, queue.NoWait, queue.Args)
		if err != nil {
			top.channelPool.FlagChannel(chanHost.ChannelID)
			return err
		}

		return nil
	}

	_, err = chanHost.Channel.QueueDeclare(queue.Name, queue.Durable, queue.AutoDelete, queue.Exclusive, queue.NoWait, queue.Args)
	if err != nil {
		top.channelPool.FlagChannel(chanHost.ChannelID)
		return err
	}

	return nil
}

// QueueDelete removes the queue from the server (and all bindings) and returns messages purged (count).
func (top *Topologer) QueueDelete(name string, ifUnused, ifEmpty, noWait bool) (int, error) {

	chanHost, err := top.channelPool.GetChannel()
	if err != nil {
		return 0, err
	}

	defer top.channelPool.ReturnChannel(chanHost, false)

	count, err := chanHost.Channel.QueueDelete(name, ifUnused, ifEmpty, noWait)
	if err != nil {
		top.channelPool.FlagChannel(chanHost.ChannelID)
		return 0, err
	}

	return count, nil
}

// QueueBind binds an Exchange to a Queue.
func (top *Topologer) QueueBind(queueBinding *models.QueueBinding) error {

	chanHost, err := top.channelPool.GetChannel()
	if err != nil {
		return err
	}

	defer top.channelPool.ReturnChannel(chanHost, false)

	err = chanHost.Channel.QueueBind(
		queueBinding.QueueName,
		queueBinding.RoutingKey,
		queueBinding.ExchangeName,
		queueBinding.NoWait,
		queueBinding.Args)

	if err != nil {
		top.channelPool.FlagChannel(chanHost.ChannelID)
		return err
	}

	return nil
}

// PurgeQueues purges each Queue provided.
func (top *Topologer) PurgeQueues(queueNames []string, noWait bool) (int, error) {

	if len(queueNames) == 0 {
		return 0, errors.New("can't purge an empty array of queues")
	}

	total := 0
	for i := 0; i < len(queueNames); i++ {
		count, err := top.PurgeQueue(queueNames[i], noWait)
		if err != nil {
			return total, err
		}

		total += count
	}

	return total, nil
}

// PurgeQueue removes all messages from the Queue that are not waiting to be Acknowledged and returns the count.
func (top *Topologer) PurgeQueue(queueName string, noWait bool) (int, error) {

	chanHost, err := top.channelPool.GetChannel()
	if err != nil {
		return 0, err
	}

	defer top.channelPool.ReturnChannel(chanHost, false)

	count, err := chanHost.Channel.QueuePurge(
		queueName,
		noWait)

	if err != nil {
		top.channelPool.FlagChannel(chanHost.ChannelID)
		return 0, err
	}

	return count, nil
}

// UnbindQueue removes the binding of a Queue to an Exchange.
func (top *Topologer) UnbindQueue(queueName, routingKey, exchangeName string, args map[string]interface{}) error {

	chanHost, err := top.channelPool.GetChannel()
	if err != nil {
		return err
	}

	defer top.channelPool.ReturnChannel(chanHost, false)

	err = chanHost.Channel.QueueUnbind(
		queueName,
		routingKey,
		exchangeName,
		amqp.Table(args))

	if err != nil {
		top.channelPool.FlagChannel(chanHost.ChannelID)
		return err
	}

	return nil
}
