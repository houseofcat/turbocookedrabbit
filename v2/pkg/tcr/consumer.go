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
	queueName    string
	consumerName string

	sleepOnErrorInterval time.Duration

	qosCountOverride int
	started          bool

	autoAck   bool
	exclusive bool
	noWait    bool
	conLock   *sync.Mutex

	pool *ConnectionPool

	errors           chan error
	receivedMessages chan *ReceivedMessage
	wg               sync.WaitGroup
	shutdownSignal   chan struct{}
	once             sync.Once
}

// NewConsumerFromConfig creates a new Consumer to receive messages from a specific queuename.
func NewConsumerFromConfig(config *ConsumerConfig, cp *ConnectionPool) *Consumer {

	return &Consumer{
		pool:                 cp,
		queueName:            config.QueueName,
		consumerName:         config.ConsumerName,
		errors:               make(chan error, 1000),
		sleepOnErrorInterval: time.Duration(config.SleepOnErrorInterval) * time.Millisecond,
		receivedMessages:     make(chan *ReceivedMessage, 1000),
		shutdownSignal:       make(chan struct{}),
		autoAck:              config.AutoAck,
		exclusive:            config.Exclusive,
		noWait:               config.NoWait,
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
	qosCountOverride int, // if zero ignored
) (*Consumer, error) {

	var (
		ok     bool
		config *ConsumerConfig
	)
	if config, ok = rconfig.ConsumerConfigs[consumerName]; !ok {
		return nil, fmt.Errorf("consumer %q was not found in config", consumerName)
	}

	return &Consumer{
		pool:                 cp,
		queueName:            queuename,
		consumerName:         consumerName,
		errors:               make(chan error, 1000),
		sleepOnErrorInterval: time.Duration(config.SleepOnErrorInterval) * time.Millisecond,
		receivedMessages:     make(chan *ReceivedMessage, 1000),
		shutdownSignal:       make(chan struct{}),
		started:              false,
		autoAck:              autoAck,
		exclusive:            exclusive,
		noWait:               noWait,
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
// returns partial batch upon error, check for len >0
func (con *Consumer) GetBatch(queueName string, batchSize int) ([]*amqp.Delivery, error) {

	if batchSize < 1 {
		return nil, errors.New("can't get a batch of messages whose size is less than 1")
	}
	messages := make([]*amqp.Delivery, 0, batchSize)

	// Get Channel
	channel, err := con.pool.GetTransientChannel(false)
	if err != nil {
		return nil, err
	}
	defer channel.Close()

	// Get A Batch of Messages
GetBatchLoop:
	for {
		amqpDelivery, ok, err := channel.Get(queueName, true)
		if err != nil {
			return messages, err
		}

		if !ok { // Break If empty
			break GetBatchLoop
		}

		messages = append(messages, &amqpDelivery)

		// Break if we have a full batch
		if len(messages) == batchSize {
			break GetBatchLoop
		}
	}

	return messages, nil
}

// Start starts the Consumer.
func (con *Consumer) Start() {
	con.conLock.Lock()
	defer con.conLock.Unlock()

	if !con.started {
		con.FlushErrors()

		con.wg.Add(1)
		go con.startConsumeLoop(nil, &con.wg)
		con.started = true
	}
}

// StartWithAction starts the Consumer invoking a method on every ReceivedMessage.
func (con *Consumer) StartWithAction(action func(*ReceivedMessage)) {
	con.conLock.Lock()
	defer con.conLock.Unlock()

	if !con.started {
		con.FlushErrors()

		con.wg.Add(1)
		go con.startConsumeLoop(action, &con.wg)
		con.started = true
	}
}

func (con *Consumer) startConsumeLoop(action func(*ReceivedMessage), wg *sync.WaitGroup) {
	defer wg.Done()

ConsumeLoop:
	for {
		// Detect if we should stop consuming.
		select {
		case <-con.catchShutdown():
			break ConsumeLoop
		default:
		}

		// Get ChannelHost
		chanHost, err := con.pool.GetChannelFromPool()
		if err != nil {
			con.sleepOnErr()
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
		deliveryChan, err := chanHost.Channel.Consume(
			con.queueName,
			con.consumerName,
			con.autoAck,
			con.exclusive,
			false,
			con.noWait,
			nil,
		)
		if err != nil {
			con.pool.ReturnChannel(chanHost, err)
			con.sleepOnErr()
			continue
		}

		// Process delivered messages by the consumer, returns true when we are to stop all consuming.
		err = con.processDeliveries(deliveryChan, chanHost, action)
		if err != nil {
			break ConsumeLoop
		}
	}
}

// ProcessDeliveries is the inner loop for processing the deliveries and returns true to break outer loop.
func (con *Consumer) processDeliveries(deliveryChan <-chan amqp.Delivery, chanHost *ChannelHost, action func(*ReceivedMessage)) error {

	for {
		select {
		// Listen for channel closure (close errors).
		case errorMessage := <-chanHost.Errors: // TODO: what happens if the channel is closed?
			if errorMessage != nil {
				con.pool.ReturnChannel(chanHost, errorMessage)
				con.sleepOnErr()
				return errorMessage
			}
		default:
		}

		select {
		case delivery, ok := <-deliveryChan:
			if !ok {
				break
			}
			// all buffered deliveries are wiped on a channel close error

			msg := NewReceivedMessage(!con.autoAck, delivery)

			if action != nil {
				action(msg)
			} else {
				con.receivedMessages <- msg
			}
		case <-con.catchShutdown():
			// Detect if we should stop consuming.
			con.pool.ReturnChannel(chanHost, nil)
			return ErrConnectionClosed
		}
	}
}

// Close shuts down the consumer and returns unprocessed messages
// It also closes the receivedMessages channel that is used to process the received messages.
// By default the internal connection pool is also shut down.
// Pass an optional false in order to prevent the pool shutdown.
func (con *Consumer) Close(shutdownConnPool ...bool) []*ReceivedMessage {
	flushed := make([]*ReceivedMessage, 0)

	con.once.Do(func() {
		shutdownPool := true
		if len(shutdownConnPool) > 0 {
			shutdownPool = shutdownConnPool[0]
		}
		con.conLock.Lock()
		defer con.conLock.Unlock()

		if !con.started {
			// already stopped, no error, as it is in the state we already want
			return
		}

		close(con.shutdownSignal)
		// wait for consumer routine to close
		con.wg.Wait()

		flushed = con.flushMessages()
		close(con.receivedMessages)

		if shutdownPool {
			con.pool.Close()
		}
	})

	return flushed
}

// ReceivedMessages yields all the internal messages ready for consuming.
// Use range over the returned channel.
// Upon shutdown this channel is closed and the Shutdown method returns all
// messages that have not been consumed, yet.
func (con *Consumer) ReceivedMessages() <-chan *ReceivedMessage {
	return con.receivedMessages
}

// Errors yields all the internal errs for consuming messages.
func (con *Consumer) Errors() <-chan error {
	return con.errors
}

// FlushErrors allows you to flush out all previous Errors.
func (con *Consumer) FlushErrors() {

FlushLoop:
	for {
		select {
		case _, ok := <-con.errors:
			if !ok {
				break FlushLoop
			}
		default:
			break FlushLoop
		}
	}
}

func (con *Consumer) flushMessages() []*ReceivedMessage {
	flushedMsgs := make([]*ReceivedMessage, 0, len(con.receivedMessages))

FlushLoop:
	for {
		select {
		case msg, ok := <-con.receivedMessages:
			if !ok {
				break FlushLoop
			}
			flushedMsgs = append(flushedMsgs, msg)
		default:
			break FlushLoop
		}
	}
	return flushedMsgs
}

// Started allows you to determine if a consumer has started.
func (con *Consumer) Started() bool {
	con.conLock.Lock()
	defer con.conLock.Unlock()
	return con.started
}

func (con *Consumer) catchShutdown() <-chan struct{} {
	return con.shutdownSignal
}

func (con *Consumer) sleepOnErr() {
	if con.sleepOnErrorInterval > 0 {
		time.Sleep(con.sleepOnErrorInterval)
	}
}
