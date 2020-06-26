package tcr

import (
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/houseofcat/turbocookedrabbit/pkg/utils"
)

// RabbitService is the struct for containing RabbitMQ management.
type RabbitService struct {
	Config               *RabbitSeasoning
	ConnectionPool       *ConnectionPool
	Topologer            *Topologer
	Publisher            *Publisher
	encryptionConfigured bool
	centralErr           chan error
	consumers            map[string]*Consumer
	stopServiceSignal    chan bool
	stop                 bool
	retryCount           uint32
	letterCount          uint64
	monitorSleepInterval time.Duration
	serviceLock          *sync.Mutex
}

// NewRabbitService creates everything you need for a RabbitMQ communication service.
func NewRabbitService(config *RabbitSeasoning) (*RabbitService, error) {

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
		centralErr:           make(chan error, config.ServiceConfig.ErrorBuffer),
		stopServiceSignal:    make(chan bool, 1),
		consumers:            make(map[string]*Consumer),
		retryCount:           10,
		monitorSleepInterval: time.Duration(3) * time.Second,
		serviceLock:          &sync.Mutex{},
	}

	err = rs.CreateConsumers(config.ConsumerConfigs)
	if err != nil {
		return nil, err
	}

	return rs, nil
}

// CreateConsumers takes a config from the Config and builds all the consumers (errors if config is missing).
func (rs *RabbitService) CreateConsumers(consumerConfigs map[string]*ConsumerConfig) error {

	for consumerName, consumerConfig := range consumerConfigs {

		consumer, err := NewConsumerFromConfig(consumerConfig, rs.ConnectionPool)
		if err != nil {
			return err
		}

		hostName, err := os.Hostname()
		if err == nil {
			ConsumerName = hostName + "-" + ConsumerName
		}

		rs.consumers[consumerName] = consumer
	}

	return nil
}

// CreateConsumerFromConfig takes a config from the Config map and builds a consumer (errors if config is missing).
func (rs *RabbitService) CreateConsumerFromConfig(consumerName string) error {

	if consumerConfig, ok := rs.Config.ConsumerConfigs[consumerName]; ok {
		consumer, err := NewConsumerFromConfig(consumerConfig, rs.ConnectionPool)
		if err != nil {
			return err
		}
		rs.consumers[consumerName] = consumer
		return nil
	}
	return nil
}

// SetHashForEncryption generates a hash key for encrypting/decrypting.
func (rs *RabbitService) SetHashForEncryption(passphrase, salt string) {

	rs.serviceLock.Lock()
	defer rs.serviceLock.Unlock()

	if rs.Config.EncryptionConfig.Enabled {
		rs.Config.EncryptionConfig.Hashkey = utils.GetHashWithArgon(
			passphrase,
			salt,
			rs.Config.EncryptionConfig.TimeConsideration,
			rs.Config.EncryptionConfig.MemoryMultiplier,
			rs.Config.EncryptionConfig.Threads,
			32)

		rs.encryptionConfigured = true
	}
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
		data, err = utils.CreateWrappedPayload(input, currentCount, metadata, rs.Config.CompressionConfig, rs.Config.EncryptionConfig)
		if err != nil {
			return err
		}
	} else {
		data, err = utils.CreatePayload(input, rs.Config.CompressionConfig, rs.Config.EncryptionConfig)
		if err != nil {
			return err
		}
	}

	rs.Publisher.PublishWithConfirmation(
		&Letter{
			LetterID:   currentCount,
			RetryCount: rs.retryCount,
			Body:       data,
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
		data, err = utils.CreateWrappedPayload(input, currentCount, metadata, rs.Config.CompressionConfig, rs.Config.EncryptionConfig)
		if err != nil {
			return err
		}
	} else {
		data, err = utils.CreatePayload(input, rs.Config.CompressionConfig, rs.Config.EncryptionConfig)
		if err != nil {
			return err
		}
	}

	rs.Publisher.Publish(
		&Letter{
			LetterID:   currentCount,
			RetryCount: rs.retryCount,
			Body:       data,
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

// StartService gets all the background internals and logging/monitoring started.
func (rs *RabbitService) StartService() {

	// Start the background monitors and logging.
	go rs.collectConsumerErrors()
	go rs.collectAutoPublisherErrors()
	go rs.monitorStopService()

	// Start the AutoPublisher
	rs.Publisher.StartAutoPublishing()
}

func (rs *RabbitService) monitorStopService() {

MonitorLoop:
	for {
		select {
		case <-rs.stopServiceSignal:
			rs.stop = true
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
				if rs.stop {
					break MonitorLoop // Prevent leaking goroutine
				}

				select {
				case err := <-Errors():
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
		if rs.stop {
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

// GetConsumer allows you to get the individual
func (rs *RabbitService) GetConsumer(consumerName string) (*Consumer, error) {

	if consumer, ok := rs.consumers[consumerName]; ok {
		return consumer, nil
	}

	return nil, errors.New("consumer was not found")
}

// Shutdown stops the service and shuts down the ChannelPool.
func (rs *RabbitService) Shutdown(stopConsumers bool) {

	rs.Publisher.StopAutoPublish()

	time.Sleep(1 * time.Second)
	rs.stopServiceSignal <- true
	time.Sleep(1 * time.Second)

	if stopConsumers {
		for _, consumer := range rs.consumers {
			err := StopConsuming(true, true)
			if err != nil {
				rs.centralErr <- err
			}
		}
	}

	rs.ConnectionPool.Shutdown()
}

// CentralErr yields all the internal errs for sub-processes.
func (rs *RabbitService) CentralErr() <-chan error {
	return rs.centralErr
}

// PublishReceipts yields all the receipts generated by the internal
func (rs *RabbitService) PublishReceipts() <-chan *PublishReceipt {
	return rs.Publisher.PublishReceipts()
}
