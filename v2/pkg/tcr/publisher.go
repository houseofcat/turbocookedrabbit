package tcr

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/streadway/amqp"
)

// Publisher contains everything you need to publish a message.
type Publisher struct {
	Config                 *RabbitSeasoning
	ConnectionPool         *ConnectionPool
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

// NewPublisherFromConfig creates and configures a new Publisher.
func NewPublisherFromConfig(
	config *RabbitSeasoning,
	cp *ConnectionPool) *Publisher {

	if config.PublisherConfig.MaxRetryCount == 0 {
		config.PublisherConfig.MaxRetryCount = 5
	}

	return &Publisher{
		Config:                 config,
		ConnectionPool:         cp,
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
	}
}

// NewPublisher creates and configures a new Publisher.
func NewPublisher(
	cp *ConnectionPool,
	sleepOnIdleInterval time.Duration,
	sleepOnErrorInterval time.Duration,
	publishTimeOutDuration time.Duration) *Publisher {

	return &Publisher{
		ConnectionPool:         cp,
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
	}
}

// Publish sends a single message to the address on the letter using a cached ChannelHost.
// Subscribe to PublishReceipts to see success and errors.
// For proper resilience (at least once delivery guarantee over shaky network) use PublishWithConfirmation
func (pub *Publisher) Publish(letter *Letter, skipReceipt bool) {

	chanHost := pub.ConnectionPool.GetChannelFromPool()

	err := chanHost.Channel.Publish(
		letter.Envelope.Exchange,
		letter.Envelope.RoutingKey,
		letter.Envelope.Mandatory,
		letter.Envelope.Immediate,
		amqp.Publishing{
			ContentType:  letter.Envelope.ContentType,
			Body:         letter.Body,
			Headers:      letter.Envelope.Headers,
			DeliveryMode: letter.Envelope.DeliveryMode,
			Priority:     letter.Envelope.Priority,
			MessageId:    letter.LetterID.String(),
			Timestamp:    time.Now().UTC(),
			AppId:        pub.Config.PoolConfig.ApplicationName,
		},
	)

	if !skipReceipt {
		pub.publishReceipt(letter, err)
	}

	pub.ConnectionPool.ReturnChannel(chanHost, err != nil)
}

// PublishWithTransient sends a single message to the address on the letter using a transient (new) RabbitMQ channel.
// Subscribe to PublishReceipts to see success and errors.
// For proper resilience (at least once delivery guarantee over shaky network) use PublishWithConfirmation
func (pub *Publisher) PublishWithTransient(letter *Letter) error {

	channel := pub.ConnectionPool.GetTransientChannel(false)
	defer func() {
		defer func() {
			_ = recover()
		}()
		channel.Close()
	}()

	return channel.Publish(
		letter.Envelope.Exchange,
		letter.Envelope.RoutingKey,
		letter.Envelope.Mandatory,
		letter.Envelope.Immediate,
		amqp.Publishing{
			ContentType:  letter.Envelope.ContentType,
			Body:         letter.Body,
			Headers:      letter.Envelope.Headers,
			DeliveryMode: letter.Envelope.DeliveryMode,
			Priority:     letter.Envelope.Priority,
			MessageId:    letter.LetterID.String(),
			Timestamp:    time.Now().UTC(),
			AppId:        pub.Config.PoolConfig.ApplicationName,
		},
	)
}

// PublishWithConfirmation sends a single message to the address on the letter with confirmation capabilities.
// This is an expensive and slow call - use this when delivery confirmation on publish is your highest priority.
// A timeout failure drops the letter back in the PublishReceipts.
// A confirmation failure keeps trying to publish (at least until timeout failure occurs.)
func (pub *Publisher) PublishWithConfirmation(letter *Letter, timeout time.Duration) {

	if timeout == 0 {
		timeout = pub.publishTimeOutDuration
	}

	for {
		// Has to use an Ackable channel for Publish Confirmations.
		chanHost := pub.ConnectionPool.GetChannelFromPool()
		chanHost.FlushConfirms() // Flush all previous publish confirmations

	Publish:
		timeoutAfter := time.After(timeout * time.Millisecond) // timeoutAfter resets everytime we try to publish.
		err := chanHost.Channel.Publish(
			letter.Envelope.Exchange,
			letter.Envelope.RoutingKey,
			letter.Envelope.Mandatory,
			letter.Envelope.Immediate,
			amqp.Publishing{
				ContentType:  letter.Envelope.ContentType,
				Body:         letter.Body,
				Headers:      letter.Envelope.Headers,
				DeliveryMode: letter.Envelope.DeliveryMode,
				Priority:     letter.Envelope.Priority,
				MessageId:    letter.LetterID.String(),
				Timestamp:    time.Now().UTC(),
				AppId:        pub.Config.PoolConfig.ApplicationName,
			},
		)
		if err != nil {
			pub.ConnectionPool.ReturnChannel(chanHost, true)
			continue // Take it again! From the top!
		}

		// Wait for very next confirmation on this channel, which should be our confirmation.
		for {
			select {
			case <-timeoutAfter:
				pub.publishReceipt(letter, fmt.Errorf("publish confirmation for LetterID: %s wasn't received in a timely manner - recommend retry/requeue", letter.LetterID.String()))
				pub.ConnectionPool.ReturnChannel(chanHost, false) // not a channel error
				return

			case confirmation := <-chanHost.Confirmations:

				if !confirmation.Ack {
					goto Publish //nack has occurred, republish
				}

				// Happy Path, publish was received by server and we didn't timeout client side.
				pub.publishReceipt(letter, nil)
				pub.ConnectionPool.ReturnChannel(chanHost, false)
				return

			default:

				time.Sleep(time.Duration(time.Millisecond * 1)) // limits CPU spin up
			}
		}
	}
}

// PublishWithConfirmationContext sends a single message to the address on the letter with confirmation capabilities.
// This is an expensive and slow call - use this when delivery confirmation on publish is your highest priority.
// A timeout failure drops the letter back in the PublishReceipts.
// A confirmation failure keeps trying to publish (at least until timeout failure occurs.)
func (pub *Publisher) PublishWithConfirmationContext(ctx context.Context, letter *Letter) {

	for {
		// Has to use an Ackable channel for Publish Confirmations.
		chanHost := pub.ConnectionPool.GetChannelFromPool()
		chanHost.FlushConfirms() // Flush all previous publish confirmations

	Publish:
		err := chanHost.Channel.Publish(
			letter.Envelope.Exchange,
			letter.Envelope.RoutingKey,
			letter.Envelope.Mandatory,
			letter.Envelope.Immediate,
			amqp.Publishing{
				ContentType:  letter.Envelope.ContentType,
				Body:         letter.Body,
				Headers:      letter.Envelope.Headers,
				DeliveryMode: letter.Envelope.DeliveryMode,
				Priority:     letter.Envelope.Priority,
				MessageId:    letter.LetterID.String(),
				Timestamp:    time.Now().UTC(),
				AppId:        pub.Config.PoolConfig.ApplicationName,
			},
		)
		if err != nil {
			pub.ConnectionPool.ReturnChannel(chanHost, true)
			continue // Take it again! From the top!
		}

		// Wait for very next confirmation on this channel, which should be our confirmation.
		for {
			select {
			case <-ctx.Done():
				pub.publishReceipt(letter, fmt.Errorf("publish confirmation for LetterID: %s wasn't received before context expired - recommend retry/requeue", letter.LetterID.String()))
				pub.ConnectionPool.ReturnChannel(chanHost, false) // not a channel error
				return

			case confirmation := <-chanHost.Confirmations:

				if !confirmation.Ack {
					goto Publish //nack has occurred, republish
				}

				// Happy Path, publish was received by server and we didn't timeout client side.
				pub.publishReceipt(letter, nil)
				pub.ConnectionPool.ReturnChannel(chanHost, false)
				return

			default:

				time.Sleep(time.Duration(time.Millisecond * 1)) // limits CPU spin up
			}
		}
	}
}

// PublishWithConfirmationTransient sends a single message to the address on the letter with confirmation capabilities on transient Channels.
// This is an expensive and slow call - use this when delivery confirmation on publish is your highest priority.
// A timeout failure drops the letter back in the PublishReceipts. When combined with QueueLetter, it automatically
//   gets requeued for re-publish.
// A confirmation failure keeps trying to publish (at least until timeout failure occurs.)
func (pub *Publisher) PublishWithConfirmationTransient(letter *Letter, timeout time.Duration) {

	if timeout == 0 {
		timeout = pub.publishTimeOutDuration
	}

	for {
		// Has to use an Ackable channel for Publish Confirmations.
		channel := pub.ConnectionPool.GetTransientChannel(true)
		confirms := make(chan amqp.Confirmation, 1)
		channel.NotifyPublish(confirms)

	Publish:
		timeoutAfter := time.After(timeout  * time.Millisecond)
		err := channel.Publish(
			letter.Envelope.Exchange,
			letter.Envelope.RoutingKey,
			letter.Envelope.Mandatory,
			letter.Envelope.Immediate,
			amqp.Publishing{
				ContentType:  letter.Envelope.ContentType,
				Body:         letter.Body,
				Headers:      letter.Envelope.Headers,
				DeliveryMode: letter.Envelope.DeliveryMode,
				Priority:     letter.Envelope.Priority,
				MessageId:    letter.LetterID.String(),
				Timestamp:    time.Now().UTC(),
				AppId:        pub.Config.PoolConfig.ApplicationName,
			},
		)
		if err != nil {
			channel.Close()
			if pub.sleepOnErrorInterval < 0 {
				time.Sleep(pub.sleepOnErrorInterval)
			}
			continue // Take it again! From the top!
		}

		// Wait for very next confirmation on this channel, which should be our confirmation.
		for {
			select {
			case <-timeoutAfter:
				pub.publishReceipt(letter, fmt.Errorf("publish confirmation for LetterID: %s wasn't received in a timely manner (%dms) - recommend retry/requeue", letter.LetterID.String(), timeout))
				channel.Close()
				return

			case confirmation := <-confirms:

				if !confirmation.Ack {
					goto Publish //nack has occurred, republish
				}

				// Happy Path, publish was received by server and we didn't timeout client side.
				pub.publishReceipt(letter, nil)
				channel.Close()
				return

			default:

				time.Sleep(time.Duration(time.Millisecond * 4)) // limits CPU spin up
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

	// Allow parallel publishing with transient channels.
	parallelPublishSemaphore := make(chan struct{}, pub.ConnectionPool.Config.MaxCacheChannelCount/2+1)

	for {

		// Publish the letter.
	PublishLoop:
		for {
			select {
			case letter := <-pub.letters:

				parallelPublishSemaphore <- struct{}{}
				go func(letter *Letter) {
					pub.PublishWithConfirmation(letter, pub.publishTimeOutDuration)
					<-parallelPublishSemaphore
				}(letter)

			default:

				if pub.sleepOnIdleInterval > 0 {
					time.Sleep(pub.sleepOnIdleInterval)
				}
				break PublishLoop

			}
		}

		// Detect if we should stop publishing.
		select {
		case stop := <-pub.autoStop:
			if stop {
				close(pub.letters)
				return true
			}
		default:
			break
		}
	}
}

// stopAutoPublish stops publishing letters queued up.
func (pub *Publisher) stopAutoPublish() {
	pub.pubLock.Lock()
	defer pub.pubLock.Unlock()

	if !pub.autoStarted {
		return
	}

	go func() { pub.autoStop <- true }() // signal auto publish to stop
}

// QueueLetters allows you to bulk queue letters that will be consumed by AutoPublish. By default, AutoPublish uses PublishWithConfirmation as the mechanism for publishing.
func (pub *Publisher) QueueLetters(letters []*Letter) bool {

	for _, letter := range letters {

		if ok := pub.safeSend(letter); !ok {
			return false
		}
	}

	return true
}

// QueueLetter queues up a letter that will be consumed by AutoPublish. By default, AutoPublish uses PublishWithConfirmation as the mechanism for publishing.
func (pub *Publisher) QueueLetter(letter *Letter) bool {

	return pub.safeSend(letter)
}

// safeSend should handle a scenario on publishing to a closed channel.
func (pub *Publisher) safeSend(letter *Letter) (closed bool) {
	defer func() {
		if recover() != nil {
			closed = false
		}
	}()

	pub.letters <- letter
	return true // success
}

// publishReceipt sends the status to the receipt channel.
func (pub *Publisher) publishReceipt(letter *Letter, err error) {

	go func(*Letter, error) {
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
	}(letter, err)
}

// Shutdown cleanly shutdown the publisher and resets it's internal state.
func (pub *Publisher) Shutdown(shutdownPools bool) {

	pub.stopAutoPublish()

	if shutdownPools { // in case the ChannelPool is shared between structs, you can prevent it from shutting down
		pub.ConnectionPool.Shutdown()
	}
}
