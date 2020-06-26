package tcr

import (
	"fmt"
	"sync"
	"time"

	"github.com/streadway/amqp"
)

// Publisher contains everything you need to publish a message.
type Publisher struct {
	Config                 *RabbitSeasoning
	ConnectionPool         *ConnectionPool
	errors                 chan error
	letters                chan *Letter
	autoStop               chan bool
	publishReceipts        chan *PublishReceipt
	autoStarted            bool
	autoPublishGroup       *sync.WaitGroup
	sleepOnIdleInterval    time.Duration
	sleepOnErrorInterval   time.Duration
	publishTimeOutDuration time.Duration
	pubLock                *sync.Mutex
	pubRWLock              *sync.RWMutex
}

// NewPublisherWithConfig creates and configures a new Publisher.
func NewPublisherWithConfig(
	config *RabbitSeasoning,
	cp *ConnectionPool) (*Publisher, error) {

	return &Publisher{
		Config:                 config,
		ConnectionPool:         cp,
		errors:                 make(chan error, 1000),
		letters:                make(chan *Letter, 1000),
		autoStop:               make(chan bool, 1),
		autoPublishGroup:       &sync.WaitGroup{},
		publishReceipts:        make(chan *PublishReceipt, 1000),
		sleepOnIdleInterval:    time.Duration(config.PublisherConfig.SleepOnIdleInterval) * time.Millisecond,
		sleepOnErrorInterval:   time.Duration(config.PublisherConfig.SleepOnErrorInterval) * time.Millisecond,
		publishTimeOutDuration: time.Duration(config.PublisherConfig.PublishTimeOutInterval) * time.Millisecond,
		pubLock:                &sync.Mutex{},
		pubRWLock:              &sync.RWMutex{},
		autoStarted:            false,
	}, nil
}

// NewPublisher creates and configures a new Publisher.
func NewPublisher(
	cp *ConnectionPool,
	sleepOnIdleInterval time.Duration,
	sleepOnErrorInterval time.Duration,
	publishTimeOutDuration time.Duration) (*Publisher, error) {

	return &Publisher{
		ConnectionPool:         cp,
		errors:                 make(chan error, 1000),
		letters:                make(chan *Letter, 1000),
		autoStop:               make(chan bool, 1),
		autoPublishGroup:       &sync.WaitGroup{},
		publishReceipts:        make(chan *PublishReceipt, 1000),
		sleepOnIdleInterval:    sleepOnIdleInterval,
		sleepOnErrorInterval:   sleepOnErrorInterval,
		publishTimeOutDuration: publishTimeOutDuration,
		pubLock:                &sync.Mutex{},
		pubRWLock:              &sync.RWMutex{},
		autoStarted:            false,
	}, nil
}

// Publish sends a single message to the address on the letter.
// Subscribe to PublishReceipts to see success and errors.
// For proper resilience (at least once delivery guarantee over shaky network) use PublishWithConfirmation
func (pub *Publisher) Publish(letter *Letter) {

	chanHost := pub.ConnectionPool.GetChannel(true)

	err := pub.simplePublish(chanHost, letter)

	pub.ConnectionPool.ReturnChannel(chanHost, err != nil)
}

func (pub *Publisher) simplePublish(chanHost *ChannelHost, letter *Letter) error {

	err := chanHost.Channel.Publish(
		letter.Envelope.Exchange,
		letter.Envelope.RoutingKey,
		letter.Envelope.Mandatory,
		letter.Envelope.Immediate,
		amqp.Publishing{
			ContentType:  letter.Envelope.ContentType,
			Body:         letter.Body,
			Headers:      amqp.Table(letter.Envelope.Headers),
			DeliveryMode: letter.Envelope.DeliveryMode,
		},
	)

	pub.publishReceipt(letter, err)
	return err
}

// PublishWithConfirmation sends a single message to the address on the letter with confirmation capabilities.
// This is an expensive and slow call - use this when delivery confirmation on publish is your highest priority.
// A timeout failure drops the letter back in the PublishReceipts.
// A confirmation failure keeps trying to publish (at least until timeout failure occurs.)
func (pub *Publisher) PublishWithConfirmation(letter *Letter, timeout time.Duration) {

	if timeout == 0 {
		timeout = pub.publishTimeOutDuration
	}

	timeoutAfter := time.After(timeout)

GetChannelAndPublish:
	for {
		// Has to use an Ackable channel for Publish Confirmations.
		chanHost := pub.ConnectionPool.GetChannel(true)

		// Subscribe to publish confirmations
		chanHost.Channel.NotifyPublish(chanHost.Confirmations)

	Publish:
		err := chanHost.Channel.Publish(
			letter.Envelope.Exchange,
			letter.Envelope.RoutingKey,
			letter.Envelope.Mandatory,
			letter.Envelope.Immediate,
			amqp.Publishing{
				ContentType:  letter.Envelope.ContentType,
				Body:         letter.Body,
				Headers:      amqp.Table(letter.Envelope.Headers),
				DeliveryMode: letter.Envelope.DeliveryMode,
			},
		)
		if err != nil {
			pub.ConnectionPool.ReturnChannel(chanHost, true)
			continue // Take it again! From the top!
		}

		// Wait for Publish Confirmations
		for {
			select {
			case <-timeoutAfter:
				pub.publishReceipt(letter, fmt.Errorf("publish confirmation for LetterId: %d wasn't received in a timely manner (%dms) - recommend retry/requeue", letter.LetterID, timeout))
				pub.ConnectionPool.ReturnChannel(chanHost, false)
				return

			case confirmation := <-chanHost.Confirmations:

				if !confirmation.Ack { // retry publishing
					goto Publish
				}

				pub.publishReceipt(letter, nil)
				pub.ConnectionPool.ReturnChannel(chanHost, false)
				break GetChannelAndPublish

			default:

				time.Sleep(time.Duration(time.Millisecond * 3))
			}
		}
	}
}

// PublishReceipts yields all the success and failures during all publish events. Highly recommend susbscribing to this.
func (pub *Publisher) PublishReceipts() <-chan *PublishReceipt {
	return pub.publishReceipts
}

// StartAutoPublishing starts the Publisher's auto-publishing capabilities.
func (pub *Publisher) StartAutoPublishing() {
	pub.pubLock.Lock()
	defer pub.pubLock.Unlock()

	if !pub.autoStarted {
		pub.FlushStops()

		pub.autoStarted = true
		go pub.startAutoPublishingLoop()
	}
}

// StartAutoPublish starts auto-publishing letters queued up - is locking.
func (pub *Publisher) startAutoPublishingLoop() {

AutoPublishLoop:
	for {
		// Detect if we should stop publishing.
		select {
		case stop := <-pub.autoStop:
			if stop {
				break AutoPublishLoop
			}
		default:
			break
		}

		// Deliver letters queued in the publisher, returns true when we are to stop publishing.
		if pub.deliverLetters() {
			break AutoPublishLoop
		}
	}

	pub.pubLock.Lock()
	pub.autoStarted = false
	pub.pubLock.Unlock()
}

func (pub *Publisher) deliverLetters() bool {

	parallelPublishSemaphore := make(chan struct{}, pub.Config.PoolConfig.ConnectionPoolConfig.MaxCacheChannelCount/2+1)

	for {

		// Publish the letter.
		select {
		case letter := <-pub.letters:

			parallelPublishSemaphore <- struct{}{}
			go func() {
				pub.PublishWithConfirmation(letter, pub.publishTimeOutDuration)
				<-parallelPublishSemaphore
			}()

		default:

			if pub.sleepOnIdleInterval > 0 {
				time.Sleep(pub.sleepOnIdleInterval)
			}
			break

		}

		// Detect if we should stop publishing.
		select {
		case stop := <-pub.autoStop:
			if stop {
				return true
			}
		default:
			break
		}
	}
}

// StopAutoPublish stops publishing letters queued up.
func (pub *Publisher) StopAutoPublish() {
	pub.pubLock.Lock()
	defer pub.pubLock.Unlock()

	if !pub.autoStarted {
		return
	}

	go func() { pub.autoStop <- true }() // signal auto publish to stop
}

// QueueLetters allows you to bulk queue letters that will be consumed by AutoPublish. By default, AutoPublish uses PublishWithConfirmation as the mechanism for publishing.
func (pub *Publisher) QueueLetters(letters []*Letter) {

	for _, letter := range letters {

		pub.letters <- letter
	}
}

// QueueLetter queues up a letter that will be consumed by AutoPublish. By default, AutoPublish uses PublishWithConfirmation as the mechanism for publishing.
func (pub *Publisher) QueueLetter(letter *Letter) {
	pub.letters <- letter
}

// publishReceipt sends the status to the receipt channel.
func (pub *Publisher) publishReceipt(letter *Letter, err error) {

	publishReceipt := &PublishReceipt{
		LetterID: letter.LetterID,
		Error:    err,
	}

	if err == nil {
		publishReceipt.Success = true
	} else {
		publishReceipt.FailedLetter = letter
	}

	pub.publishReceipts <- publishReceipt
}

// Errors yields all the internal errs for delivering letters.
func (pub *Publisher) Errors() <-chan error {
	return pub.errors
}

// FlushStops flushes out all the AutoStop messages.
func (pub *Publisher) FlushStops() {

FlushLoop:
	for {
		select {
		case <-pub.autoStop:
		default:
			break FlushLoop
		}
	}
}

// Shutdown cleanly shutsdown the publisher and resets it's internal state.
func (pub *Publisher) Shutdown(shutdownPools bool) {
	pub.StopAutoPublish()

	if shutdownPools { // in case the ChannelPool is shared between structs, you can prevent it from shutting down
		pub.ConnectionPool.Shutdown()
	}
}
