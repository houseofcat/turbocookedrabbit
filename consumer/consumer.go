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
	Config           *models.RabbitSeasoning
	ChannelPool      *pools.ChannelPool
	QueueName        string
	ConsumerName     string
	QOS              uint32
	errors           chan error
	messageGroup     *sync.WaitGroup
	messages         chan *models.Message
	consumeStop      chan bool
	stopImmediate    bool
	started          bool
	autoAck          bool
	exclusive        bool
	noLocal          bool
	noWait           bool
	args             map[string]interface{}
	qosCountOverride int
	qosSizeOverride  int
	conLock          *sync.Mutex
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
	args map[string]interface{},
	qosCountOverride int, // if zero ignored
	qosSizeOverride int) (*Consumer, error) {

	var err error
	if channelPool == nil {
		channelPool, err = pools.NewChannelPool(config, nil, true)
		if err != nil {
			return nil, err
		}
	}

	return &Consumer{
		Config:           config,
		ChannelPool:      channelPool,
		QueueName:        queuename,
		ConsumerName:     consumerName,
		errors:           make(chan error, 10),
		messageGroup:     &sync.WaitGroup{},
		messages:         make(chan *models.Message, 10),
		consumeStop:      make(chan bool, 1),
		stopImmediate:    false,
		started:          false,
		autoAck:          autoAck,
		exclusive:        exclusive,
		noWait:           noWait,
		args:             args,
		qosCountOverride: qosCountOverride,
		qosSizeOverride:  qosSizeOverride,
		conLock:          &sync.Mutex{},
	}, nil
}

// StartConsuming starts the Consumer.
func (con *Consumer) StartConsuming() error {
	con.conLock.Lock()
	defer con.conLock.Unlock()

	if con.started {
		return errors.New("can't start an already started consumer")
	}

	con.FlushErrors()
	con.FlushStop()

	go con.startConsuming()
	con.started = true
	return nil
}

func (con *Consumer) startConsuming() {

GetChannelLoop:
	for {
		// Detect if we should stop.
		select {
		case stop := <-con.consumeStop:
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

		// Quality of Service channel overrides
		if con.qosCountOverride != 0 && con.qosSizeOverride != 0 {
			chanHost.Channel.Qos(con.qosCountOverride, con.qosSizeOverride, false)
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
				con.messageGroup.Add(1)
				con.convertDelivery(chanHost.Channel, &delivery, !con.autoAck)
			default:
				break
			}

			// Detect if we should stop.
			select {
			case stop := <-con.consumeStop:
				if stop {
					chanHost.Channel.Flow(false)
					break GetChannelLoop
				}
			default:
				break
			}
		}

		// Quality of Service channel overrides reset
		if con.Config.Pools.GlobalQosCount != 0 && con.Config.Pools.GlobalQosSize != 0 {
			chanHost.Channel.Qos(con.qosCountOverride, con.qosSizeOverride, false)
		}
	}

	con.conLock.Lock()
	defer con.conLock.Unlock()

	immediateStop := con.stopImmediate

	if !immediateStop {
		con.messageGroup.Wait() // wait for every message to be received to internal queue
	}

	con.started = false
	con.stopImmediate = false
}

// StopConsuming allows you to signal stop to the consumer.
// Will stop on the consumer channelclose or responding to signal after getting all remaining deviveries.
func (con *Consumer) StopConsuming(immediate bool) error {
	con.conLock.Lock()
	defer con.conLock.Unlock()

	con.stopImmediate = true
	if !con.started {
		return errors.New("can't stop a stopped consumer")
	}

	con.signalStop()
	return nil
}

// SignalStop sends stop signal.
func (con *Consumer) signalStop() {
	go func() { con.consumeStop <- true }()
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

	go func() {
		defer con.messageGroup.Done()
		con.messages <- msg
	}()
}

// FlushStop allows you to flush out all previous Stop signals.
func (con *Consumer) FlushStop() {

FlushLoop:
	for {
		select {
		case <-con.consumeStop:
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
