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
	Config               *models.RabbitSeasoning
	channelPool          *pools.ChannelPool
	QueueName            string
	ConsumerName         string
	errors               chan error
	sleepOnErrorInterval time.Duration
	messageGroup         *sync.WaitGroup
	messages             chan *models.Message
	consumeStop          chan bool
	stopImmediate        bool
	started              bool
	autoAck              bool
	exclusive            bool
	noLocal              bool
	noWait               bool
	args                 amqp.Table
	qosCountOverride     int
	conLock              *sync.Mutex
}

// NewConsumerFromConfig creates a new Consumer to receive messages from a specific queuename.
func NewConsumerFromConfig(
	config *models.ConsumerConfig,
	channelPool *pools.ChannelPool) (*Consumer, error) {

	if channelPool == nil {
		return nil, errors.New("can't start a consumer without a channel pool")
	} else if !channelPool.Initialized {
		channelPool.Initialize()
	}

	if config.MessageBuffer == 0 || config.ErrorBuffer == 0 {
		return nil, errors.New("message and/or error buffer in config can't be 0")
	}

	return &Consumer{
		Config:               nil,
		channelPool:          channelPool,
		QueueName:            config.QueueName,
		ConsumerName:         config.ConsumerName,
		errors:               make(chan error, config.ErrorBuffer),
		sleepOnErrorInterval: time.Duration(config.SleepOnErrorInterval) * time.Millisecond,
		messageGroup:         &sync.WaitGroup{},
		messages:             make(chan *models.Message, config.MessageBuffer),
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
	config *models.RabbitSeasoning,
	channelPool *pools.ChannelPool,
	queuename string,
	consumerName string,
	autoAck bool,
	exclusive bool,
	noWait bool,
	args map[string]interface{},
	qosCountOverride int, // if zero ignored
	messageBuffer uint32,
	errorBuffer uint32,
	sleepOnErrorInterval uint32) (*Consumer, error) {

	var err error
	if channelPool == nil {
		channelPool, err = pools.NewChannelPool(config.PoolConfig, nil, true)
		if err != nil {
			return nil, err
		}
	}

	if messageBuffer == 0 || errorBuffer == 0 {
		return nil, errors.New("message and/or error buffer can't be 0")
	}

	return &Consumer{
		Config:               config,
		channelPool:          channelPool,
		QueueName:            queuename,
		ConsumerName:         consumerName,
		errors:               make(chan error, errorBuffer),
		sleepOnErrorInterval: time.Duration(sleepOnErrorInterval) * time.Millisecond,
		messageGroup:         &sync.WaitGroup{},
		messages:             make(chan *models.Message, messageBuffer),
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

// Get gets a single message from any queue.
func (con *Consumer) Get(queueName string, autoAck bool) (*models.Message, error) {

	// Get Channel
	var chanHost *models.ChannelHost
	var err error

	if autoAck {
		chanHost, err = con.channelPool.GetChannel()
	} else {
		chanHost, err = con.channelPool.GetAckableChannel(con.noWait)
	}

	if err != nil {
		return nil, err
	}

	// Get Single Message
	amqpDelivery, ok, getErr := chanHost.Channel.Get(queueName, autoAck)
	if getErr != nil {
		con.channelPool.FlagChannel(chanHost.ChannelID)
		return nil, getErr
	}

	if ok {
		return models.NewMessage(
			!autoAck,
			amqpDelivery.Body,
			amqpDelivery.DeliveryTag,
			chanHost.Channel), nil
	}

	return nil, nil
}

// GetBatch gets a group of messages from any queue.
func (con *Consumer) GetBatch(queueName string, batchSize int, autoAck bool) ([]*models.Message, error) {

	if batchSize < 1 {
		return nil, errors.New("can't get a batch of messages whose size is less than 1")
	}

	// Get Channel
	var chanHost *models.ChannelHost
	var err error

	if autoAck {
		chanHost, err = con.channelPool.GetChannel()
	} else {
		chanHost, err = con.channelPool.GetAckableChannel(con.noWait)
	}

	if err != nil {
		return nil, err
	}

	messages := make([]*models.Message, 0)

	// Get A Batch of Messages
GetBatchLoop:
	for {
		if len(messages) == batchSize {
			break GetBatchLoop
		}

		amqpDelivery, ok, getErr := chanHost.Channel.Get(queueName, autoAck)
		if getErr != nil {
			con.channelPool.FlagChannel(chanHost.ChannelID)
			return nil, getErr
		}

		if !ok {
			break GetBatchLoop
		}

		messages = append(messages, models.NewMessage(
			!autoAck,
			amqpDelivery.Body,
			amqpDelivery.DeliveryTag,
			chanHost.Channel))
	}

	return messages, nil
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

ConsumerOuterLoop:
	for {
		// Detect if we should stop.
		select {
		case stop := <-con.consumeStop:
			if stop {
				break ConsumerOuterLoop
			}
		default:
			break
		}

		deliveryChan, chanHost, err := con.getDeliveryChannel()
		if err != nil {
			continue // retry
		}

		//ProcessDeliveries InnerLoop - Returns true when consumer stop is called.
		if con.processDeliveries(deliveryChan, chanHost) {
			break ConsumerOuterLoop
		}

		// Quality of Service channel overrides reset
		if con.Config.PoolConfig.ChannelPoolConfig.GlobalQosCount > 0 {
			chanHost.Channel.Qos(
				con.Config.PoolConfig.ChannelPoolConfig.GlobalQosCount,
				0,
				false)
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

// GetDeliveryChannel attempts to get the amqp.Delivery chan and a viable ChannelHost from the ChannelPool.
func (con *Consumer) getDeliveryChannel() (<-chan amqp.Delivery, *models.ChannelHost, error) {

	// Get Channel
	var chanHost *models.ChannelHost
	var err error

	if con.autoAck {
		chanHost, err = con.channelPool.GetChannel()
	} else {
		chanHost, err = con.channelPool.GetAckableChannel(con.noWait)
	}

	if err != nil {
		con.handleError(err)
		return nil, nil, err
	}

	// Quality of Service channel overrides
	if con.qosCountOverride > 0 {
		chanHost.Channel.Qos(con.qosCountOverride, 0, false)
	}

	// Start Consuming
	deliveryChan, err := chanHost.Channel.Consume(con.QueueName, con.ConsumerName, con.autoAck, con.exclusive, false, con.noWait, nil)
	if err != nil {
		con.handleErrorAndFlagChannel(err, chanHost.ChannelID)
		return nil, nil, err // Retry
	}

	return deliveryChan, chanHost, nil
}

// ProcessDeliveries is the inner loop for processing the deliveries and returns true to break outer loop.
func (con *Consumer) processDeliveries(deliveryChan <-chan amqp.Delivery, chanHost *models.ChannelHost) bool {

ProcessDeliveriesInnerLoop:
	for {
		// Listen for channel closure (close errors).
		// Highest priority so separated to it's own select.
		select {
		case errorMessage := <-chanHost.CloseErrors():
			if errorMessage != nil {
				con.handleErrorAndFlagChannel(fmt.Errorf("consumer's current channel closed\r\n[reason: %s]\r\n[code: %d]", errorMessage.Reason, errorMessage.Code), chanHost.ChannelID)

				break ProcessDeliveriesInnerLoop
			}
		default:
			break
		}

		// Convert amqp.Delivery into our internal struct for later use.
		select {
		case delivery := <-deliveryChan: // all buffered deliveries are wipe on a channel close error
			con.messageGroup.Add(1)
			con.convertDelivery(chanHost.Channel, &delivery, !con.autoAck)
		default:
			break
		}

		// Detect if we should stop.
		select {
		case stop := <-con.consumeStop:
			if stop {
				return true
			}
		default:
			break
		}
	}

	return false
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

// Messages yields all the internal messages ready for consuming.
func (con *Consumer) Messages() <-chan *models.Message {
	return con.messages
}

func (con *Consumer) handleErrorAndFlagChannel(err error, channelID uint64) {
	con.channelPool.FlagChannel(channelID)
	con.handleError(err)
}

func (con *Consumer) handleError(err error) {
	go func() { con.errors <- err }()

	if con.sleepOnErrorInterval > 0 {
		time.Sleep(con.sleepOnErrorInterval * time.Millisecond)
	}
}

// Errors yields all the internal errs for consuming messages.
func (con *Consumer) Errors() <-chan error {
	return con.errors
}

func (con *Consumer) convertDelivery(amqpChan *amqp.Channel, delivery *amqp.Delivery, isAckable bool) {
	msg := models.NewMessage(
		isAckable,
		delivery.Body,
		delivery.DeliveryTag,
		amqpChan)

	go func() {
		defer con.messageGroup.Done() // finished after getting the message in the channel

		con.messages <- msg
	}()
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
		case <-con.messages:
		default:
			break FlushLoop
		}
	}
}
