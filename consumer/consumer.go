package consumer

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/houseofcat/turbocookedrabbit/models"
	"github.com/houseofcat/turbocookedrabbit/pools"
	"github.com/streadway/amqp"
)

// Consumer receives messages from a RabbitMQ location.
type Consumer struct {
	Config       *models.RabbitSeasoning
	ChannelPool  *pools.ChannelPool
	QueueName    string
	ConsumerName string
	errors       chan error
	messages     chan *models.Message
	stop         chan bool
	started      bool
	autoAck      bool
	exclusive    bool
	noLocal      bool
	noWait       bool
	args         map[string]interface{}
	conLock      *sync.Mutex
}

// NewConsumer creates a new Consumer to receive messages from a specific queuename.
func NewConsumer(
	config *models.RabbitSeasoning,
	channelPool *pools.ChannelPool,
	queuename string,
	consumerName string,
	autoAck bool,
	exclusive bool,
	noWait bool,
	args map[string]interface{}) (*Consumer, error) {

	var err error
	if channelPool == nil {
		channelPool, err = pools.NewChannelPool(config, nil, true)
		if err != nil {
			return nil, err
		}
	}

	return &Consumer{
		Config:       config,
		ChannelPool:  channelPool,
		QueueName:    queuename,
		ConsumerName: consumerName,
		errors:       make(chan error, 1),
		messages:     make(chan *models.Message, 1),
		started:      false,
		autoAck:      autoAck,
		exclusive:    exclusive,
		noWait:       noWait,
		args:         args,
		conLock:      &sync.Mutex{},
	}, nil
}

// StartConsumer starts the Consumer.
func (con *Consumer) StartConsumer() error {
	con.conLock.Lock()
	defer con.conLock.Unlock()

	if con.started {
		return errors.New("can't start an already started consumer")
	}

	con.FlushErrors()
	con.FlushStop()

	go con.startConsumer()
	con.started = true
	return nil
}

func (con *Consumer) startConsumer() {

GetChannelLoop:
	for {
		// Detect if we should stop.
		select {
		case stop := <-con.stop:
			if stop {
				break GetChannelLoop
			}
		default:
			break
		}

		// Get Channel
		chanHost, err := con.ChannelPool.GetChannel()
		if err != nil {
			go func() { con.errors <- err }()
			time.Sleep(1 * time.Second)
			continue // Retry
		}

		// Start Consuming
		deliveryChan, err := chanHost.Channel.Consume(con.QueueName, con.ConsumerName, con.autoAck, con.exclusive, false, con.noWait, con.args)
		if err != nil {
			go func() { con.errors <- err }()
			time.Sleep(1 * time.Second)
			continue // Retry
		}

	GetDeliveriesLoop:
		for {
			// Listen for channel closure (close errors).
			// Highest priority so separated to it's own select.
			select {
			case amqpError := <-chanHost.CloseErrors():
				if amqpError != nil {
					go func() {
						con.errors <- fmt.Errorf("consumer's current channel closed\r\n[Reason: %s]\r\n[Code: %d]", amqpError.Reason, amqpError.Code)
					}()

					break GetDeliveriesLoop
				}
			default:
				break
			}

			// Convert amqp.Delivery into our internal struct for later use.
			select {
			case delivery := <-deliveryChan:
				go con.convertDelivery(chanHost.Channel, &delivery, !con.autoAck)
			default:
				break
			}

			// Detect if we should stop.
			select {
			case stop := <-con.stop:
				if stop {
					break GetChannelLoop
				}
			default:
				break
			}
		}
	}

	con.started = false
}

// SignalStop allows you to signal stop to the consumer.
// Will stop on the consumer channelclose or responding to signal after getting all remaining deviveries.
func (con *Consumer) SignalStop() error {
	con.conLock.Lock()
	defer con.conLock.Unlock()

	if !con.started {
		return errors.New("can't stop a stopped consumer")
	}

	con.signalStop()
	return nil
}

// SignalStop sends stop signal.
func (con *Consumer) signalStop() {
	go func() { con.stop <- true }()
}

// Messages yields all the internal messages ready for consuming.
func (con *Consumer) Messages() <-chan *models.Message {
	return con.messages
}

// Errors yields all the internal errs for consuming messages.
func (con *Consumer) Errors() <-chan error {
	return con.errors
}

func (con *Consumer) convertDelivery(amqpChan *amqp.Channel, delivery *amqp.Delivery, isAckable bool) {
	msg := &models.Message{
		IsAckable:  isAckable,
		Channel:    amqpChan,
		Body:       delivery.Body,
		MessageTag: delivery.DeliveryTag,
	}

	go func() { con.messages <- msg }()
}

// FlushStop allows you to flush out all previous Stop signals.
func (con *Consumer) FlushStop() {

FlushLoop:
	for {
		select {
		case <-con.stop:
			break
		default:
			break FlushLoop
		}
	}
}

// FlushErrors allows you to flush out all previous Errors.
func (con *Consumer) FlushErrors() {

FlushLoop:
	for {
		select {
		case <-con.errors:
			break
		default:
			break FlushLoop
		}
	}
}

// FlushMessages allows you to flush out all previous Messages.
// WARNING: THIS WILL RESULT IN LOST MESSAGES.
func (con *Consumer) FlushMessages() {

FlushLoop:
	for {
		select {
		case <-con.messages:
			break
		default:
			break FlushLoop
		}
	}
}
