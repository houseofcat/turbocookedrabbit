package tcr

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// RabbitService is the struct for containing all you need for RabbitMQ access.
type RabbitService struct {
	Config               *RabbitSeasoning
	ConnectionPool       *ConnectionPool
	Topologer            *Topologer
	Publisher            *Publisher
	encryptionConfigured bool
	centralErr           chan error
	consumers            map[string]*Consumer
	shutdownSignal       chan bool
	shutdown             bool
	letterCount          uint64
	monitorSleepInterval time.Duration
	serviceLock          *sync.Mutex
}

// NewRabbitService creates everything you need for a RabbitMQ communication service.
func NewRabbitService(config *RabbitSeasoning, passphrase, salt string) (*RabbitService, error) {

	connectionPool, err := NewConnectionPool(config.PoolConfig)
	if err != nil {
		return nil, err
	}

	publisher, err := NewPublisherWithConfig(config, connectionPool)
	if err != nil {
		return nil, err
	}

	topologer := NewTopologer(connectionPool)
	if err != nil {
		return nil, err
	}

	rs := &RabbitService{
		ConnectionPool:       connectionPool,
		Config:               config,
		Publisher:            publisher,
		Topologer:            topologer,
		centralErr:           make(chan error),
		shutdownSignal:       make(chan bool, 1),
		consumers:            make(map[string]*Consumer),
		monitorSleepInterval: time.Duration(200) * time.Millisecond,
		serviceLock:          &sync.Mutex{},
	}

	// Build a Map to Consumer retrieval.
	err = rs.createConsumers(config.ConsumerConfigs)
	if err != nil {
		return nil, err
	}

	// Create a HashKey for Encryption
	if config.EncryptionConfig.Enabled && len(passphrase) > 0 && len(salt) > 0 {
		rs.Config.EncryptionConfig.Hashkey = GetHashWithArgon(
			passphrase,
			salt,
			rs.Config.EncryptionConfig.TimeConsideration,
			rs.Config.EncryptionConfig.MemoryMultiplier,
			rs.Config.EncryptionConfig.Threads,
			32)

		rs.encryptionConfigured = true
	}

	// Start the background monitors and logging.
	go rs.collectConsumerErrors()
	go rs.collectAutoPublisherErrors()
	go rs.monitorForShutdown()

	// Start the AutoPublisher
	rs.Publisher.StartAutoPublishing()

	return rs, nil
}

// CreateConsumers takes a config from the Config and builds all the consumers (errors if config is missing).
func (rs *RabbitService) createConsumers(consumerConfigs map[string]*ConsumerConfig) error {

	for consumerName, consumerConfig := range consumerConfigs {

		consumer, err := NewConsumerFromConfig(consumerConfig, rs.ConnectionPool)
		if err != nil {
			return err
		}

		hostName, err := os.Hostname()
		if err == nil {
			consumer.ConsumerName = hostName + "-" + consumer.ConsumerName
		}

		rs.consumers[consumerName] = consumer
	}

	return nil
}

// PublishWithConfirmation tries to publish and wait for a confirmation.
func (rs *RabbitService) PublishWithConfirmation(input interface{}, exchangeName, routingKey string, wrapPayload bool, metadata string) error {

	if input == nil || (exchangeName == "" && routingKey == "") {
		return errors.New("can't have a nil body or an empty exchangename with empty routing key")
	}

	currentCount := atomic.LoadUint64(&rs.letterCount)
	atomic.AddUint64(&rs.letterCount, 1)

	var data []byte
	var err error
	if wrapPayload {
		data, err = CreateWrappedPayload(input, currentCount, metadata, rs.Config.CompressionConfig, rs.Config.EncryptionConfig)
		if err != nil {
			return err
		}
	} else {
		data, err = CreatePayload(input, rs.Config.CompressionConfig, rs.Config.EncryptionConfig)
		if err != nil {
			return err
		}
	}

	rs.Publisher.PublishWithConfirmation(
		&Letter{
			LetterID: currentCount,
			Body:     data,
			Envelope: &Envelope{
				Exchange:     exchangeName,
				RoutingKey:   routingKey,
				ContentType:  "application/json",
				Mandatory:    false,
				Immediate:    false,
				DeliveryMode: 2,
			},
		},
		time.Duration(time.Millisecond*300))

	return nil
}

// Publish tries to publish directly without retry and data optionally wrapped in a ModdedLetter.
func (rs *RabbitService) Publish(input interface{}, exchangeName, routingKey string, wrapPayload bool, metadata string) error {

	if input == nil || (exchangeName == "" && routingKey == "") {
		return errors.New("can't have a nil input or an empty exchangename with empty routing key")
	}

	currentCount := atomic.LoadUint64(&rs.letterCount)
	atomic.AddUint64(&rs.letterCount, 1)

	var data []byte
	var err error
	if wrapPayload {
		data, err = CreateWrappedPayload(input, currentCount, metadata, rs.Config.CompressionConfig, rs.Config.EncryptionConfig)
		if err != nil {
			return err
		}
	} else {
		data, err = CreatePayload(input, rs.Config.CompressionConfig, rs.Config.EncryptionConfig)
		if err != nil {
			return err
		}
	}

	rs.Publisher.Publish(
		&Letter{
			LetterID: currentCount,
			Body:     data,
			Envelope: &Envelope{
				Exchange:     exchangeName,
				RoutingKey:   routingKey,
				ContentType:  "application/json",
				Mandatory:    false,
				Immediate:    false,
				DeliveryMode: 2,
			},
		})

	return nil
}

// GetConsumer allows you to get the individual
func (rs *RabbitService) GetConsumer(consumerName string) (*Consumer, error) {

	if consumer, ok := rs.consumers[consumerName]; ok {
		return consumer, nil
	}

	return nil, fmt.Errorf("consumer %q was not found", consumerName)
}

// CentralErr yields all the internal errs for sub-processes.
func (rs *RabbitService) CentralErr() <-chan error {
	return rs.centralErr
}

// PublishReceipts yields all the receipts generated by the internal publisher.
func (rs *RabbitService) PublishReceipts() <-chan *PublishReceipt {
	return rs.Publisher.PublishReceipts()
}

// Shutdown stops the service and shuts down the ChannelPool.
func (rs *RabbitService) Shutdown(stopConsumers bool) {

	rs.Publisher.StopAutoPublish()

	time.Sleep(1 * time.Second)
	rs.shutdownSignal <- true
	time.Sleep(1 * time.Second)

	if stopConsumers {
		for _, consumer := range rs.consumers {
			err := consumer.StopConsuming(true, true)
			if err != nil {
				rs.centralErr <- err
			}
		}
	}

	rs.ConnectionPool.Shutdown()
}

func (rs *RabbitService) monitorForShutdown() {

MonitorLoop:
	for {
		select {
		case <-rs.shutdownSignal:
			rs.shutdown = true
			break MonitorLoop // Prevent leaking goroutine
		default:
			time.Sleep(rs.monitorSleepInterval)
			break
		}
	}
}

func (rs *RabbitService) collectConsumerErrors() {

MonitorLoop:
	for {

		for _, consumer := range rs.consumers {
		IndividualConsumerLoop:
			for {
				if rs.shutdown {
					break MonitorLoop // Prevent leaking goroutine
				}

				select {
				case err := <-consumer.Errors():
					rs.centralErr <- err
				default:
					break IndividualConsumerLoop
				}
			}
		}

		time.Sleep(rs.monitorSleepInterval)
	}
}

func (rs *RabbitService) collectAutoPublisherErrors() {

MonitorLoop:
	for {
		if rs.shutdown {
			break MonitorLoop // Prevent leaking goroutine
		}

		select {
		case err := <-rs.Publisher.Errors():
			rs.centralErr <- err
		default:
			time.Sleep(rs.monitorSleepInterval)
			break
		}
	}
}
