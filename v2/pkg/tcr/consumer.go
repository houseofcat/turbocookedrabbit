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
	config       *ConsumerConfig
	queueName    string
	consumerName string

	sleepOnIdleInterval  time.Duration
	sleepOnErrorInterval time.Duration

	args             amqp.Table
	qosCountOverride int
	stopImmediate    bool
	started          bool
	autoAck          bool
	exclusive        bool
	noWait           bool
	enabled          bool
	conLock          *sync.Mutex

	pool *ConnectionPool

	messageGroup *sync.WaitGroup

	errors           chan error
	receivedMessages chan *ReceivedMessage
	consumeStop      chan bool
}

// NewConsumerFromConfig creates a new Consumer to receive messages from a specific queuename.
func NewConsumerFromConfig(config *ConsumerConfig, cp *ConnectionPool) *Consumer {

	return &Consumer{
		config:               config,
		pool:                 cp,
		enabled:              config.Enabled,
		queueName:            config.QueueName,
		consumerName:         config.ConsumerName,
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
	}
}

// NewConsumer creates a new Consumer to receive messages from a specific queuename.
func NewConsumer(
	rconfig *RabbitSeasoning,
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

	var ok bool
	var config *ConsumerConfig
	if config, ok = rconfig.ConsumerConfigs[consumerName]; !ok {
		return nil, fmt.Errorf("consumer %q was not found in config", consumerName)
	}

	return &Consumer{
		config:               config,
		pool:                 cp,
		enabled:              true,
		queueName:            queuename,
		consumerName:         consumerName,
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
		args:                 args,
		qosCountOverride:     qosCountOverride,
		conLock:              &sync.Mutex{},
	}, nil
}

// Get gets a single message from any queue. Auto-Acknowledges.
func (con *Consumer) Get(queueName string) (*amqp.Delivery, error) {

	// Get Channel
	channel, err := con.pool.GetTransientChannel(false)
	if err != nil {
		return nil, err
	}
	defer channel.Close()

	// Get Single Message
	amqpDelivery, ok, getErr := channel.Get(queueName, true)
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
	channel, err := con.pool.GetTransientChannel(false)
	if err != nil {
		return nil, err
	}
	defer channel.Close()

	messages := make([]*amqp.Delivery, 0)

	// Get A Batch of Messages
GetBatchLoop:
	for {
		// Break if we have a full batch
		if len(messages) == batchSize {
			break GetBatchLoop
		}

		amqpDelivery, ok, err := channel.Get(queueName, true)
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

	if con.enabled {

		con.FlushErrors()
		con.FlushStop()

		go con.startConsumeLoop(nil)
		con.started = true
	}
}

// StartConsumingWithAction starts the Consumer invoking a method on every ReceivedMessage.
func (con *Consumer) StartConsumingWithAction(action func(*ReceivedMessage)) {
	con.conLock.Lock()
	defer con.conLock.Unlock()

	if con.enabled {

		con.FlushErrors()
		con.FlushStop()

		go con.startConsumeLoop(action)
		con.started = true
	}
}

func (con *Consumer) startConsumeLoop(action func(*ReceivedMessage)) {

ConsumeLoop:
	for {
		// Detect if we should stop consuming.
		select {
		case stop := <-con.consumeStop:
			if stop {
				break ConsumeLoop
			}
		default:
		}

		// Get ChannelHost
		chanHost, err := con.pool.GetChannelFromPool()
		if err != nil {
			if con.sleepOnErrorInterval > 0 {
				time.Sleep(con.sleepOnErrorInterval)
			}
			// in the next loop iteration
			// the consumer is supposed to be stopped
			// due to consumer
			continue
		}

		// Configure RabbitMQ channel QoS for Consumer
		if con.qosCountOverride > 0 {
			_ = chanHost.Channel.Qos(con.qosCountOverride, 0, false)
		}

		// Initiate consuming process.
		deliveryChan, err := chanHost.Channel.Consume(con.queueName, con.consumerName, con.autoAck, con.exclusive, false, con.noWait, nil)
		if err != nil {
			con.pool.ReturnChannel(chanHost, true)
			if con.sleepOnErrorInterval > 0 {
				time.Sleep(con.sleepOnErrorInterval)
			}
			continue
		}

		// Process delivered messages by the consumer, returns true when we are to stop all consuming.
		if con.processDeliveries(deliveryChan, chanHost, action) {
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
func (con *Consumer) processDeliveries(deliveryChan <-chan amqp.Delivery, chanHost *ChannelHost, action func(*ReceivedMessage)) bool {

	for {
		// Listen for channel closure (close errors).
		// Highest priority so separated to it's own select.
		select {
		case errorMessage := <-chanHost.Errors:
			if errorMessage != nil {
				con.pool.ReturnChannel(chanHost, true)
				con.errors <- fmt.Errorf("consumer's current channel closed\r\n[reason: %s]\r\n[code: %d]", errorMessage.Reason, errorMessage.Code)
				if con.sleepOnErrorInterval > 0 {
					time.Sleep(con.sleepOnErrorInterval)
				}
				return false
			}
		default:
		}

		// Convert amqp.Delivery into our internal struct for later use.
		select {
		case delivery := <-deliveryChan: // all buffered deliveries are wiped on a channel close error

			msg := NewReceivedMessage(
				!con.autoAck,
				delivery)

			if action != nil {
				action(msg)
			} else {
				con.receivedMessages <- msg
			}

		default:
			if con.sleepOnIdleInterval > 0 {
				time.Sleep(con.sleepOnIdleInterval)
			}
		}

		// Detect if we should stop consuming.
		select {
		case stop := <-con.consumeStop:
			if stop {
				con.pool.ReturnChannel(chanHost, false)
				return true
			}
		default:
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

// Started allows you to determine if a consumer has started.
func (con *Consumer) Started() bool {
	con.conLock.Lock()
	defer con.conLock.Unlock()
	return con.started
}
