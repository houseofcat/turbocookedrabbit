package tcr

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/streadway/amqp"
)

// Consumer receives messages from a RabbitMQ location.
type Consumer struct {
	Config               *RabbitSeasoning
	ConnectionPool       *ConnectionPool
	Enabled              bool
	QueueName            string
	ConsumerName         string
	errors               chan error
	sleepOnErrorInterval time.Duration
	sleepOnIdleInterval  time.Duration
	messageGroup         *sync.WaitGroup
	receivedMessages     chan *ReceivedMessage
	consumeStop          chan bool
	stopImmediate        bool
	started              bool
	autoAck              bool
	exclusive            bool
	noWait               bool
	args                 amqp.Table
	qosCountOverride     int
	conLock              *sync.Mutex
}

// NewConsumerFromConfig creates a new Consumer to receive messages from a specific queuename.
func NewConsumerFromConfig(config *ConsumerConfig, cp *ConnectionPool) (*Consumer, error) {

	if config == nil || cp == nil {
		return nil, fmt.Errorf("config or connection pool was nil")
	}

	return &Consumer{
		Config:               nil,
		ConnectionPool:       cp,
		Enabled:              config.Enabled,
		QueueName:            config.QueueName,
		ConsumerName:         config.ConsumerName,
		errors:               make(chan error, 1000),
		sleepOnErrorInterval: time.Duration(config.SleepOnErrorInterval) * time.Millisecond,
		sleepOnIdleInterval:  time.Duration(config.SleepOnIdleInterval) * time.Millisecond,
		messageGroup:         &sync.WaitGroup{},
		receivedMessages:     make(chan *ReceivedMessage, 1000),
		consumeStop:          make(chan bool, 1),
		autoAck:              config.AutoAck,
		exclusive:            config.Exclusive,
		noWait:               config.NoWait,
		args:                 amqp.Table(config.Args),
		qosCountOverride:     config.QosCountOverride,
		conLock:              &sync.Mutex{},
	}, nil
}

// NewConsumer creates a new Consumer to receive messages from a specific queuename.
func NewConsumer(
	config *RabbitSeasoning,
	cp *ConnectionPool,
	queuename string,
	consumerName string,
	autoAck bool,
	exclusive bool,
	noWait bool,
	args map[string]interface{},
	qosCountOverride int, // if zero ignored
	sleepOnErrorInterval uint32,
	sleepOnIdleInterval uint32) (*Consumer, error) {

	return &Consumer{
		Config:               config,
		ConnectionPool:       cp,
		Enabled:              true,
		QueueName:            queuename,
		ConsumerName:         consumerName,
		errors:               make(chan error, 1000),
		sleepOnErrorInterval: time.Duration(sleepOnErrorInterval) * time.Millisecond,
		sleepOnIdleInterval:  time.Duration(sleepOnIdleInterval) * time.Millisecond,
		messageGroup:         &sync.WaitGroup{},
		receivedMessages:     make(chan *ReceivedMessage, 1000),
		consumeStop:          make(chan bool, 1),
		stopImmediate:        false,
		started:              false,
		autoAck:              autoAck,
		exclusive:            exclusive,
		noWait:               noWait,
		args:                 amqp.Table(args),
		qosCountOverride:     qosCountOverride,
		conLock:              &sync.Mutex{},
	}, nil
}

// Get gets a single message from any queue. Auto-Acknowledges.
func (con *Consumer) Get(queueName string) (*amqp.Delivery, error) {

	// Get Channel
	chanHost := con.ConnectionPool.GetChannel(false)
	defer chanHost.Close()

	// Get Single Message
	amqpDelivery, ok, getErr := chanHost.Channel.Get(queueName, true)
	if getErr != nil {
		return nil, getErr
	}

	if ok {
		return &amqpDelivery, nil
	}

	return nil, nil
}

// GetBatch gets a group of messages from any queue. Auto-Acknowledges.
func (con *Consumer) GetBatch(queueName string, batchSize int) ([]*amqp.Delivery, error) {

	if batchSize < 1 {
		return nil, errors.New("can't get a batch of messages whose size is less than 1")
	}

	// Get Channel
	chanHost := con.ConnectionPool.GetChannel(false)
	defer chanHost.Close()

	messages := make([]*amqp.Delivery, 0)

	// Get A Batch of Messages
GetBatchLoop:
	for {
		// Break if we have a full batch
		if len(messages) == batchSize {
			break GetBatchLoop
		}

		amqpDelivery, ok, err := chanHost.Channel.Get(queueName, true)
		if err != nil {
			return nil, err
		}

		if !ok { // Break If empty
			break GetBatchLoop
		}

		messages = append(messages, &amqpDelivery)
	}

	return messages, nil
}

// StartConsuming starts the Consumer.
func (con *Consumer) StartConsuming() {
	con.conLock.Lock()
	defer con.conLock.Unlock()

	if con.Enabled {

		con.FlushErrors()
		con.FlushStop()

		go con.startConsumeLoop()
		con.started = true
	}
}

func (con *Consumer) startConsumeLoop() {

ConsumeLoop:
	for {
		// Detect if we should stop consuming.
		select {
		case stop := <-con.consumeStop:
			if stop {
				break ConsumeLoop
			}
		default:
			break
		}

		// Get ChannelHost
		chanHost := con.ConnectionPool.GetChannel(true)

		// Configure RabbitMQ channel QoS for Consumer
		if con.qosCountOverride > 0 {
			chanHost.Channel.Qos(con.qosCountOverride, 0, false)
		}

		// Initiate consuming process.
		deliveryChan, err := chanHost.Channel.Consume(con.QueueName, con.ConsumerName, con.autoAck, con.exclusive, false, con.noWait, nil)
		if err != nil {
			con.ConnectionPool.ReturnChannel(chanHost, true)
			continue
		}

		// Process delivered messages by the consumer, returns true when we are to stop all consuming.
		if con.processDeliveries(deliveryChan, chanHost) {
			break ConsumeLoop
		}
	}

	con.conLock.Lock()
	immediateStop := con.stopImmediate
	con.conLock.Unlock()

	if !immediateStop {
		con.messageGroup.Wait() // wait for every message to be received to the internal queue
	}

	con.conLock.Lock()
	con.started = false
	con.stopImmediate = false
	con.conLock.Unlock()
}

// ProcessDeliveries is the inner loop for processing the deliveries and returns true to break outer loop.
func (con *Consumer) processDeliveries(deliveryChan <-chan amqp.Delivery, chanHost *ChannelHost) bool {

	for {
		// Listen for channel closure (close errors).
		// Highest priority so separated to it's own select.
		select {
		case errorMessage := <-chanHost.Errors:
			if errorMessage != nil {
				con.ConnectionPool.ReturnChannel(chanHost, true)
				con.errors <- fmt.Errorf("consumer's current channel closed\r\n[reason: %s]\r\n[code: %d]", errorMessage.Reason, errorMessage.Code)
				return false
			}
		default:
			break
		}

		// Convert amqp.Delivery into our internal struct for later use.
		select {
		case delivery := <-deliveryChan: // all buffered deliveries are wiped on a channel close error
			con.messageGroup.Add(1)
			con.convertDelivery(chanHost.Channel, &delivery, !con.autoAck)
		default:
			if con.sleepOnIdleInterval > 0 {
				time.Sleep(con.sleepOnIdleInterval)
			}
			break
		}

		// Detect if we should stop consuming.
		select {
		case stop := <-con.consumeStop:
			if stop {
				con.ConnectionPool.ReturnChannel(chanHost, false)
				return true
			}
		default:
			break
		}
	}
}

// StopConsuming allows you to signal stop to the consumer.
// Will stop on the consumer channelclose or responding to signal after getting all remaining deviveries.
// FlushMessages empties the internal buffer of messages received by queue. Ackable messages are still in
// RabbitMQ queue, while noAck messages will unfortunately be lost. Use wisely.
func (con *Consumer) StopConsuming(immediate bool, flushMessages bool) error {
	con.conLock.Lock()
	defer con.conLock.Unlock()

	if !con.started {
		return errors.New("can't stop a stopped consumer")
	}

	con.stopImmediate = immediate
	con.consumeStop <- true

	// This helps terminate all goroutines trying to add messages too.
	if flushMessages {
		con.FlushMessages()
	}

	return nil
}

// ReceivedMessages yields all the internal messages ready for consuming.
func (con *Consumer) ReceivedMessages() <-chan *ReceivedMessage {
	return con.receivedMessages
}

// Errors yields all the internal errs for consuming messages.
func (con *Consumer) Errors() <-chan error {
	return con.errors
}

func (con *Consumer) convertDelivery(amqpChan *amqp.Channel, delivery *amqp.Delivery, isAckable bool) {
	msg := NewMessage(
		isAckable,
		delivery.Body,
		delivery.DeliveryTag,
		amqpChan)

	con.receivedMessages <- msg
	con.messageGroup.Done() // finished after getting the message in the channel
}

// FlushStop allows you to flush out all previous Stop signals.
func (con *Consumer) FlushStop() {

FlushLoop:
	for {
		select {
		case <-con.consumeStop:
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
		case <-con.receivedMessages:
		default:
			break FlushLoop
		}
	}
}
