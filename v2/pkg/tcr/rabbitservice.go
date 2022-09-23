package tcr

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/streadway/amqp"
)

// RabbitService is the struct for containing all you need for RabbitMQ access.
type RabbitService struct {
	config               *RabbitSeasoning
	monitorSleepInterval time.Duration

	*Topologer
	*Publisher
	pool *ConnectionPool

	shutdownSignal chan struct{}
	centralErr     chan error

	encryptionConfigured bool
	consumers            map[string]*Consumer
	serviceLock          *sync.Mutex
}

// NewRabbitService creates everything you need for a RabbitMQ communication service.
func NewRabbitService(
	config *RabbitSeasoning,
	passphrase string,
	salt string,
	processPublishReceipts func(*PublishReceipt),
	processError func(error)) (*RabbitService, error) {

	connectionPool, err := NewConnectionPool(config.PoolConfig)
	if err != nil {
		return nil, err
	}

	return NewRabbitServiceWithConnectionPool(connectionPool, config, passphrase, salt, processPublishReceipts, processError)
}

// NewRabbitServiceWithConnectionPool creates everything you need for a RabbitMQ communication service from a connection pool.
func NewRabbitServiceWithConnectionPool(
	connectionPool *ConnectionPool,
	config *RabbitSeasoning,
	passphrase string,
	salt string,
	processPublishReceipts func(*PublishReceipt),
	processError func(error)) (*RabbitService, error) {

	publisher := NewPublisherFromConfig(config, connectionPool)
	return NewRabbitServiceWithPublisher(publisher, config, passphrase, salt, processPublishReceipts, processError)
}

// NewRabbitServiceWithPublisher creates everything you need for a RabbitMQ communication service from a publisher.
func NewRabbitServiceWithPublisher(
	publisher *Publisher,
	config *RabbitSeasoning,
	passphrase string,
	salt string,
	processPublishReceipts func(*PublishReceipt),
	processError func(error)) (*RabbitService, error) {

	topologer := NewTopologer(publisher.pool)

	rs := &RabbitService{
		pool:                 publisher.pool,
		config:               config,
		Publisher:            publisher,
		Topologer:            topologer,
		centralErr:           make(chan error, 1000),
		shutdownSignal:       make(chan struct{}),
		consumers:            make(map[string]*Consumer),
		monitorSleepInterval: 200 * time.Millisecond,
		serviceLock:          &sync.Mutex{},
	}

	// Build a Map for Consumer retrieval.
	err := rs.createConsumers(config.ConsumerConfigs)
	if err != nil {
		return nil, err
	}

	// Create a HashKey for Encryption
	if config.EncryptionConfig.Enabled && len(passphrase) > 0 && len(salt) > 0 {
		rs.config.EncryptionConfig.Hashkey = GetHashWithArgon(
			passphrase,
			salt,
			rs.config.EncryptionConfig.TimeConsideration,
			rs.config.EncryptionConfig.MemoryMultiplier,
			rs.config.EncryptionConfig.Threads,
			32)

		rs.encryptionConfigured = true
	}

	// Start the background monitors and logging.
	go rs.collectConsumerErrors()

	// Monitors all publish events
	if processPublishReceipts != nil {
		go rs.invokeProcessPublishReceipts(processPublishReceipts)
	} else { // Default action is to retry publishing all failures.
		go rs.processPublishReceipts()
	}

	// Monitors all errors
	if processError != nil {
		go rs.invokeProcessError(processError)
	} else { // Default action is to retry publishing all failures.
		go rs.processErrors()
	}

	// Start the AutoPublisher
	rs.Publisher.StartAutoPublishing()

	return rs, nil
}

// CreateConsumers takes a config from the Config and builds all the consumers (errors if config is missing).
func (rs *RabbitService) createConsumers(consumerConfigs map[string]*ConsumerConfig) error {

	for consumerName, consumerConfig := range consumerConfigs {

		consumer := NewConsumerFromConfig(consumerConfig, rs.pool)
		hostName, err := os.Hostname()

		if err == nil {
			consumer.consumerName = hostName + "-" + consumer.consumerName
		}

		rs.consumers[consumerName] = consumer
	}

	return nil
}

// PublishWithConfirmation tries to publish and wait for a confirmation.
func (rs *RabbitService) PublishWithConfirmation(
	input interface{},
	exchangeName, routingKey, metadata string,
	wrapPayload bool,
	headers amqp.Table) error {

	if rs.isShutdown() {
		return fmt.Errorf("unable to publish: %w", ErrServiceShutdown)
	}

	if input == nil || (exchangeName == "" && routingKey == "") {
		return errors.New("can't have a nil body or an empty exchangename with empty routing key")
	}

	var letterID = uuid.New()
	var data []byte
	var err error
	if wrapPayload {
		data, err = CreateWrappedPayload(input, letterID, metadata, rs.config.CompressionConfig, rs.config.EncryptionConfig)
		if err != nil {
			return err
		}
	} else {
		data, err = CreatePayload(input, rs.config.CompressionConfig, rs.config.EncryptionConfig)
		if err != nil {
			return err
		}
	}

	// Non-Transient Has A Bug For Now
	// https://github.com/streadway/amqp/issues/459
	rs.Publisher.PublishWithConfirmationTransient(
		&Letter{
			LetterID: letterID,
			Body:     data,
			Envelope: &Envelope{
				Exchange:     exchangeName,
				RoutingKey:   routingKey,
				ContentType:  "application/json",
				Mandatory:    false,
				Immediate:    false,
				DeliveryMode: 2,
				Headers:      headers,
			},
		},
		300*time.Millisecond)

	return nil
}

// Publish tries to publish directly without retry and data optionally wrapped in a ModdedLetter.
func (rs *RabbitService) Publish(
	input interface{},
	exchangeName, routingKey, metadata string,
	wrapPayload bool,
	headers amqp.Table) error {

	if rs.isShutdown() {
		return fmt.Errorf("unable to publish: %w", ErrServiceShutdown)
	}

	if input == nil || (exchangeName == "" && routingKey == "") {
		return errors.New("can't have a nil input or an empty exchangename with empty routing key")
	}

	var letterID = uuid.New()
	var data []byte
	var err error
	if wrapPayload {
		data, err = CreateWrappedPayload(input, letterID, metadata, rs.config.CompressionConfig, rs.config.EncryptionConfig)
		if err != nil {
			return err
		}
	} else {
		data, err = CreatePayload(input, rs.config.CompressionConfig, rs.config.EncryptionConfig)
		if err != nil {
			return err
		}
	}

	rs.Publisher.Publish(
		&Letter{
			LetterID: letterID,
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
		false)

	return nil
}

// PublishData tries to publish.
func (rs *RabbitService) PublishData(
	data []byte,
	exchangeName, routingKey string,
	headers amqp.Table) error {

	if rs.isShutdown() {
		return fmt.Errorf("unable to publish: %w", ErrServiceShutdown)
	}

	if data == nil || (exchangeName == "" && routingKey == "") {
		return errors.New("can't have a nil input or an empty exchangename with empty routing key")
	}

	rs.Publisher.Publish(
		&Letter{
			LetterID: uuid.New(),
			Body:     data,
			Envelope: &Envelope{
				Exchange:     exchangeName,
				RoutingKey:   routingKey,
				ContentType:  "application/json",
				Mandatory:    false,
				Immediate:    false,
				DeliveryMode: 2,
				Headers:      headers,
			},
		},
		false)

	return nil
}

// PublishLetter wraps around Publisher to simply Publish.
func (rs *RabbitService) PublishLetter(letter *Letter) error {

	if rs.isShutdown() {
		return fmt.Errorf("unable to publish: %w", ErrServiceShutdown)
	}

	if letter.LetterID.String() == "" {
		letter.LetterID = uuid.New()
	}

	rs.Publisher.Publish(letter, false)

	return nil
}

// QueueLetter wraps around AutoPublisher to simply QueueLetter.
// Error indicates message was not queued.
func (rs *RabbitService) QueueLetter(letter *Letter) error {

	if rs.isShutdown() {
		return fmt.Errorf("unable to queue letter : %w", ErrServiceShutdown)
	}

	if letter.LetterID.String() == "" {
		letter.LetterID = uuid.New()
	}

	if ok := rs.Publisher.QueueLetter(letter); !ok {
		return errors.New("unable to queue letter... most likely cause is autopublisher chan was shut")
	}

	return nil
}

// GetConsumer allows you to get the individual consumers stored in memory.
func (rs *RabbitService) GetConsumer(consumerName string) (*Consumer, error) {

	if consumer, ok := rs.consumers[consumerName]; ok {
		return consumer, nil
	}

	return nil, fmt.Errorf("consumer %q was not found", consumerName)
}

// GetConsumerConfig allows you to get the individual consumers' config stored in memory.
func (rs *RabbitService) GetConsumerConfig(consumerName string) (*ConsumerConfig, error) {

	if consumer, ok := rs.consumers[consumerName]; ok {
		return consumer.config, nil
	}

	return nil, fmt.Errorf("consumer %q was not found", consumerName)
}

// CentralErr yields all the internal errs for sub-processes.
func (rs *RabbitService) CentralErr() <-chan error {
	return rs.centralErr
}

// Shutdown stops the service and shuts down the ChannelPool.
func (rs *RabbitService) Shutdown() {

	rs.Publisher.Shutdown(false)

	time.Sleep(time.Second)
	close(rs.shutdownSignal)
	time.Sleep(time.Second)

	for _, consumer := range rs.consumers {
		err := consumer.StopConsuming(true, true)
		if err != nil {
			rs.centralErr <- err
		}
	}

	rs.pool.Shutdown()
}

func (rs *RabbitService) collectConsumerErrors() {

	for {
		if rs.isShutdown() {
			return // Prevent leaking goroutine
		}

		// TODO: rs.consumers might be written to asynchronously, potential race condition
		for _, consumer := range rs.consumers {
		IndividualConsumerLoop:
			for {
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

func (rs *RabbitService) invokeProcessPublishReceipts(processReceipts func(*PublishReceipt)) {

	for {
		select {
		case <-rs.catchShutdown():
			return // Prevent leaking goroutine
		case receipt := <-rs.Publisher.PublishReceipts():
			processReceipts(receipt)
		}
	}
}

func (rs *RabbitService) processPublishReceipts() {

	for {
		select {
		case <-rs.catchShutdown():
			return // Prevent leaking goroutine
		case receipt := <-rs.Publisher.PublishReceipts():
			if !receipt.Success {
				if receipt.FailedLetter != nil {
					if receipt.FailedLetter.RetryCount < rs.config.PublisherConfig.MaxRetryCount {
						receipt.FailedLetter.RetryCount++
						rs.centralErr <- fmt.Errorf("failed to publish LetterID %s... retrying (count: %d)", receipt.LetterID.String(), receipt.FailedLetter.RetryCount)
						if ok := rs.Publisher.QueueLetter(receipt.FailedLetter); !ok {
							rs.centralErr <- fmt.Errorf("failed to publish a LetterID %s and autopublisher has been shutdown", receipt.LetterID.String())
						}
					} else {
						rs.centralErr <- fmt.Errorf("failed to retry publish a LetterID %s, it has exhausted all of it's retries", receipt.LetterID.String())
					}
				} else {
					rs.centralErr <- fmt.Errorf("failed to publish a LetterID %s and unable to retry as a copy of the letter was not received", receipt.LetterID.String())
				}
			}
		}
	}
}

func (rs *RabbitService) invokeProcessError(processError func(error)) {

	for {
		select {
		case <-rs.catchShutdown():
			return // prevent goroutine leak
		case err := <-rs.centralErr:
			processError(err)
		}
	}
}

func (rs *RabbitService) processErrors() {

	for {
		select {
		case <-rs.catchShutdown():
			return // Prevent leaking goroutine
		case err := <-rs.centralErr:
			fmt.Printf("TCR Central Err: %s\r\n", err)
		}
	}
}

func (rs *RabbitService) isShutdown() bool {

	select {
	case <-rs.shutdownSignal:
		return true
	default:
		return false
	}
}

func (rs *RabbitService) catchShutdown() <-chan struct{} {
	return rs.shutdownSignal
}
