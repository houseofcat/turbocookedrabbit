package tcr

import (
	"errors"

	"github.com/houseofcat/turbocookedrabbit/pools"
	"github.com/streadway/amqp"
)

// Topologer allows you to build RabbitMQ topology backed by a ConnectionPool.
type Topologer struct {
	ConnectionPool *pools.ConnectionPool
}

// NewTopologer builds you a new Topologer.
func NewTopologer(cp *pools.ConnectionPool) *Topologer {

	return &Topologer{
		ConnectionPool: cp,
	}
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

	chanHost := top.ConnectionPool.GetChannel(false)
	defer chanHost.Close()

	if passiveDeclare {
		return chanHost.Channel.ExchangeDeclarePassive(exchangeName, exchangeType, durable, autoDelete, internal, noWait, amqp.Table(args))
	}

	return chanHost.Channel.ExchangeDeclare(exchangeName, exchangeType, durable, autoDelete, internal, noWait, amqp.Table(args))
}

// CreateExchangeFromConfig builds an Exchange toplogy from a config Exchange element.
func (top *Topologer) CreateExchangeFromConfig(exchange *models.Exchange) error {

	chanHost := top.ConnectionPool.GetChannel(false)
	defer chanHost.Close()

	if exchange.PassiveDeclare {
		return chanHost.Channel.ExchangeDeclarePassive(
			exchange.Name,
			exchange.Type,
			exchange.Durable,
			exchange.AutoDelete,
			exchange.InternalOnly,
			exchange.NoWait,
			exchange.Args)
	}

	return chanHost.Channel.ExchangeDeclare(
		exchange.Name,
		exchange.Type,
		exchange.Durable,
		exchange.AutoDelete,
		exchange.InternalOnly,
		exchange.NoWait,
		exchange.Args)
}

// ExchangeBind binds an exchange to an Exchange.
func (top *Topologer) ExchangeBind(exchangeBinding *models.ExchangeBinding) error {

	chanHost := top.ConnectionPool.GetChannel(false)
	defer chanHost.Close()

	return chanHost.Channel.ExchangeBind(
		exchangeBinding.ExchangeName,
		exchangeBinding.RoutingKey,
		exchangeBinding.ParentExchangeName,
		exchangeBinding.NoWait,
		exchangeBinding.Args)
}

// ExchangeDelete removes the exchange from the server.
func (top *Topologer) ExchangeDelete(
	exchangeName string,
	ifUnused, noWait bool) error {

	chanHost := top.ConnectionPool.GetChannel(false)
	defer chanHost.Close()

	return chanHost.Channel.ExchangeDelete(exchangeName, ifUnused, noWait)
}

// ExchangeUnbind removes the binding of an Exchange to an Exchange.
func (top *Topologer) ExchangeUnbind(exchangeName, routingKey, parentExchangeName string, noWait bool, args map[string]interface{}) error {

	chanHost := top.ConnectionPool.GetChannel(false)
	defer chanHost.Close()

	return chanHost.Channel.ExchangeUnbind(
		exchangeName,
		routingKey,
		parentExchangeName,
		noWait,
		amqp.Table(args))
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

	chanHost := top.ConnectionPool.GetChannel(false)
	defer chanHost.Close()

	if passiveDeclare {
		_, err := chanHost.Channel.QueueDeclarePassive(queueName, durable, autoDelete, exclusive, noWait, amqp.Table(args))
		return err
	}

	_, err := chanHost.Channel.QueueDeclare(queueName, durable, autoDelete, exclusive, noWait, amqp.Table(args))
	return err
}

// CreateQueueFromConfig builds a Queue topology from a config Exchange element.
func (top *Topologer) CreateQueueFromConfig(queue *models.Queue) error {

	chanHost := top.ConnectionPool.GetChannel(false)
	defer chanHost.Close()

	if queue.PassiveDeclare {
		_, err := chanHost.Channel.QueueDeclarePassive(queue.Name, queue.Durable, queue.AutoDelete, queue.Exclusive, queue.NoWait, queue.Args)
		return err
	}

	_, err := chanHost.Channel.QueueDeclare(queue.Name, queue.Durable, queue.AutoDelete, queue.Exclusive, queue.NoWait, queue.Args)
	return err
}

// QueueDelete removes the queue from the server (and all bindings) and returns messages purged (count).
func (top *Topologer) QueueDelete(name string, ifUnused, ifEmpty, noWait bool) (int, error) {

	chanHost := top.ConnectionPool.GetChannel(false)
	defer chanHost.Close()

	return chanHost.Channel.QueueDelete(name, ifUnused, ifEmpty, noWait)
}

// QueueBind binds an Exchange to a Queue.
func (top *Topologer) QueueBind(queueBinding *models.QueueBinding) error {

	chanHost := top.ConnectionPool.GetChannel(false)
	defer chanHost.Close()

	return chanHost.Channel.QueueBind(
		queueBinding.QueueName,
		queueBinding.RoutingKey,
		queueBinding.ExchangeName,
		queueBinding.NoWait,
		queueBinding.Args)
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

	chanHost := top.ConnectionPool.GetChannel(false)
	defer chanHost.Close()

	return chanHost.Channel.QueuePurge(
		queueName,
		noWait)
}

// UnbindQueue removes the binding of a Queue to an Exchange.
func (top *Topologer) UnbindQueue(queueName, routingKey, exchangeName string, args map[string]interface{}) error {

	chanHost := top.ConnectionPool.GetChannel(false)
	defer chanHost.Close()

	return chanHost.Channel.QueueUnbind(
		queueName,
		routingKey,
		exchangeName,
		amqp.Table(args))
}
