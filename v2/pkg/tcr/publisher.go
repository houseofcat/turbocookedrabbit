package tcr

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/streadway/amqp"
)

// Publisher contains everything you need to publish a message.
type Publisher struct {
	sleepOnErrorInterval   time.Duration
	publishTimeOutDuration time.Duration

	pool *ConnectionPool

	autoStarted     int32
	letters         chan *Letter
	publishReceipts chan *PublishReceipt
	wg              sync.WaitGroup
	shutdownSignal  chan struct{}
	once            sync.Once
}

// NewPublisherFromConfig creates and configures a new Publisher.
func NewPublisherFromConfig(config *RabbitSeasoning, cp *ConnectionPool) *Publisher {

	if config.PublisherConfig.MaxRetryCount == 0 {
		config.PublisherConfig.MaxRetryCount = 5
	}

	return &Publisher{
		pool: cp,

		letters:         make(chan *Letter, 1000),
		publishReceipts: make(chan *PublishReceipt, 1000),

		autoStarted:    0, // false
		shutdownSignal: make(chan struct{}),

		sleepOnErrorInterval:   time.Duration(config.PublisherConfig.SleepOnErrorInterval) * time.Millisecond,
		publishTimeOutDuration: time.Duration(config.PublisherConfig.PublishTimeOutInterval) * time.Millisecond,
	}
}

// NewPublisher creates and configures a new Publisher.
func NewPublisher(cp *ConnectionPool, sleepOnIdleInterval time.Duration, sleepOnErrorInterval time.Duration, publishTimeOutDuration time.Duration) *Publisher {

	return &Publisher{
		pool: cp,

		letters:         make(chan *Letter, 1000),
		publishReceipts: make(chan *PublishReceipt, 1000),

		autoStarted:    0, //false
		shutdownSignal: make(chan struct{}),

		sleepOnErrorInterval:   sleepOnErrorInterval,
		publishTimeOutDuration: publishTimeOutDuration,
	}
}

// Publish sends a single message to the address on the letter using a cached ChannelHost.
// Subscribe to PublishReceipts to see success and errors.
//
// For proper resilience (at least once delivery guarantee over shaky network) use PublishWithConfirmation
func (pub *Publisher) Publish(letter *Letter, skipReceipt bool) {

	chanHost, err := pub.pool.GetChannelFromPool()
	if err != nil {
		// potential problem of loosing the letter
		// upon shutdown of the connection pool
		pub.pool.ReturnChannel(chanHost, err)
		return
	}

	err = chanHost.Channel.Publish(
		letter.Envelope.Exchange,
		letter.Envelope.RoutingKey,
		letter.Envelope.Mandatory,
		letter.Envelope.Immediate,
		amqp.Publishing{
			ContentType:   letter.Envelope.ContentType,
			Body:          letter.Body,
			Headers:       letter.Envelope.Headers,
			DeliveryMode:  letter.Envelope.DeliveryMode,
			Priority:      letter.Envelope.Priority,
			MessageId:     letter.LetterID.String(),
			CorrelationId: letter.Envelope.CorrelationID,
			Type:          letter.Envelope.Type,
			Timestamp:     time.Now().UTC(),
			AppId:         pub.pool.Config.ApplicationName,
		},
	)

	if !skipReceipt {
		pub.publishReceipt(letter, err)
	}

	pub.pool.ReturnChannel(chanHost, err)
}

// PublishWithError sends a single message to the address on the letter using a cached ChannelHost.
//
// For proper resilience (at least once delivery guarantee over shaky network) use PublishWithConfirmation
func (pub *Publisher) PublishWithError(letter *Letter, skipReceipt bool) error {

	chanHost, err := pub.pool.GetChannelFromPool()
	if err != nil {
		return err
	}

	err = chanHost.Channel.Publish(
		letter.Envelope.Exchange,
		letter.Envelope.RoutingKey,
		letter.Envelope.Mandatory,
		letter.Envelope.Immediate,
		amqp.Publishing{
			ContentType:   letter.Envelope.ContentType,
			Body:          letter.Body,
			Headers:       letter.Envelope.Headers,
			DeliveryMode:  letter.Envelope.DeliveryMode,
			Priority:      letter.Envelope.Priority,
			MessageId:     letter.LetterID.String(),
			CorrelationId: letter.Envelope.CorrelationID,
			Type:          letter.Envelope.Type,
			Timestamp:     time.Now().UTC(),
			AppId:         pub.pool.Config.ApplicationName,
		},
	)

	if !skipReceipt {
		pub.publishReceipt(letter, err)
	}

	pub.pool.ReturnChannel(chanHost, err)
	return err
}

// PublishWithTransient sends a single message to the address on the letter using a transient (new) RabbitMQ channel.
// Subscribe to PublishReceipts to see success and errors.
// For proper resilience (at least once delivery guarantee over shaky network) use PublishWithConfirmation
func (pub *Publisher) PublishWithTransient(letter *Letter) error {

	channel, err := pub.pool.GetTransientChannel(false)
	if err != nil {
		return fmt.Errorf("publish failed: %w", err)
	}
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
			ContentType:   letter.Envelope.ContentType,
			Body:          letter.Body,
			Headers:       letter.Envelope.Headers,
			DeliveryMode:  letter.Envelope.DeliveryMode,
			Priority:      letter.Envelope.Priority,
			MessageId:     letter.LetterID.String(),
			CorrelationId: letter.Envelope.CorrelationID,
			Type:          letter.Envelope.Type,
			Timestamp:     time.Now().UTC(),
			AppId:         pub.pool.Config.ApplicationName,
		},
	)
}

// PublishWithConfirmation sends a single message to the address on the letter with confirmation capabilities.
//
// This is an expensive and slow call - use this when delivery confirmation on publish is your highest priority.
// A timeout failure drops the letter back in the PublishReceipts.
// A confirmation failure keeps trying to publish (at least until timeout failure occurs.)
func (pub *Publisher) PublishWithConfirmation(letter *Letter, timeout time.Duration) {

	if timeout == 0 {
		timeout = pub.publishTimeOutDuration
	}

	for {
		// Has to use an Ackable channel for Publish Confirmations.
		chanHost, err := pub.pool.GetChannelFromPool()
		if err != nil {
			pub.publishReceipt(letter, fmt.Errorf("publish of LetterID: %s failed: %w", letter.LetterID.String(), err))
			return
		}
		chanHost.FlushConfirms() // Flush all previous publish confirmations

	Publish:
		timeoutAfter := time.After(timeout) // timeoutAfter resets everytime we try to publish.
		err = chanHost.Channel.Publish(
			letter.Envelope.Exchange,
			letter.Envelope.RoutingKey,
			letter.Envelope.Mandatory,
			letter.Envelope.Immediate,
			amqp.Publishing{
				ContentType:   letter.Envelope.ContentType,
				Body:          letter.Body,
				Headers:       letter.Envelope.Headers,
				DeliveryMode:  letter.Envelope.DeliveryMode,
				Priority:      letter.Envelope.Priority,
				MessageId:     letter.LetterID.String(),
				CorrelationId: letter.Envelope.CorrelationID,
				Type:          letter.Envelope.Type,
				Timestamp:     time.Now().UTC(),
				AppId:         pub.pool.Config.ApplicationName,
			},
		)
		if err != nil {
			pub.pool.ReturnChannel(chanHost, err)
			continue // Take it again! From the top!
		}

		// Wait for very next confirmation on this channel, which should be our confirmation.
		for {
			select {
			case <-timeoutAfter:
				pub.publishReceipt(letter, fmt.Errorf("publish confirmation for LetterID: %s wasn't received in a timely manner - recommend retry/requeue", letter.LetterID.String()))
				pub.pool.ReturnChannel(chanHost, nil) // not a channel error
				return

			case confirmation := <-chanHost.Confirmations:

				if !confirmation.Ack {
					goto Publish //nack has occurred, republish
				}

				// Happy Path, publish was received by server and we didn't timeout client side.
				pub.publishReceipt(letter, nil)
				pub.pool.ReturnChannel(chanHost, nil)
				return

			default:

				time.Sleep(time.Duration(time.Millisecond * 1)) // limits CPU spin up
			}
		}
	}
}

// PublishWithConfirmationError sends a single message to the address on the letter with confirmation capabilities.
//
// This is an expensive and slow call - use this when delivery confirmation on publish is your highest priority.
// A timeout failure drops the letter back in the PublishReceipts.
// A confirmation failure keeps trying to publish (at least until timeout failure occurs.)
func (pub *Publisher) PublishWithConfirmationError(letter *Letter, timeout time.Duration) error {

	if timeout == 0 {
		timeout = pub.publishTimeOutDuration
	}

	for {
		// Has to use an Ackable channel for Publish Confirmations.
		chanHost, err := pub.pool.GetChannelFromPool()
		if err != nil {
			return fmt.Errorf("publish of LetterID: %s failed: %w", letter.LetterID.String(), err)
		}
		chanHost.FlushConfirms() // Flush all previous publish confirmations

	Publish:
		timeoutAfter := time.After(timeout) // timeoutAfter resets everytime we try to publish.
		err = chanHost.Channel.Publish(
			letter.Envelope.Exchange,
			letter.Envelope.RoutingKey,
			letter.Envelope.Mandatory,
			letter.Envelope.Immediate,
			amqp.Publishing{
				ContentType:   letter.Envelope.ContentType,
				Body:          letter.Body,
				Headers:       letter.Envelope.Headers,
				DeliveryMode:  letter.Envelope.DeliveryMode,
				Priority:      letter.Envelope.Priority,
				MessageId:     letter.LetterID.String(),
				CorrelationId: letter.Envelope.CorrelationID,
				Type:          letter.Envelope.Type,
				Timestamp:     time.Now().UTC(),
				AppId:         pub.pool.Config.ApplicationName,
			},
		)
		if err != nil {
			pub.pool.ReturnChannel(chanHost, err)
			continue // Take it again! From the top!
		}

		// Wait for very next confirmation on this channel, which should be our confirmation.
		for {
			select {
			case <-timeoutAfter:
				pub.pool.ReturnChannel(chanHost, nil) // not a channel error
				return fmt.Errorf("publish confirmation for LetterID: %s wasn't received in a timely manner - recommend retry/requeue", letter.LetterID.String())

			case confirmation := <-chanHost.Confirmations:

				if !confirmation.Ack {
					goto Publish //nack has occurred, republish
				}

				// Happy Path, publish was received by server and we didn't timeout client side.
				pub.pool.ReturnChannel(chanHost, nil)
				return nil

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
		chanHost, err := pub.pool.GetChannelFromPool()
		if err != nil {
			pub.publishReceipt(letter, fmt.Errorf("publish of LetterID: %s failed: %w", letter.LetterID.String(), err))
			return
		}
		chanHost.FlushConfirms() // Flush all previous publish confirmations

	Publish:
		err = chanHost.Channel.Publish(
			letter.Envelope.Exchange,
			letter.Envelope.RoutingKey,
			letter.Envelope.Mandatory,
			letter.Envelope.Immediate,
			amqp.Publishing{
				ContentType:   letter.Envelope.ContentType,
				Body:          letter.Body,
				Headers:       letter.Envelope.Headers,
				DeliveryMode:  letter.Envelope.DeliveryMode,
				Priority:      letter.Envelope.Priority,
				MessageId:     letter.LetterID.String(),
				CorrelationId: letter.Envelope.CorrelationID,
				Type:          letter.Envelope.Type,
				Timestamp:     time.Now().UTC(),
				AppId:         pub.pool.Config.ApplicationName,
			},
		)
		if err != nil {
			pub.pool.ReturnChannel(chanHost, err)
			continue // Take it again! From the top!
		}

		// Wait for very next confirmation on this channel, which should be our confirmation.
		for {
			select {
			case <-ctx.Done():
				pub.publishReceipt(letter, fmt.Errorf("publish confirmation for LetterID: %s wasn't received before context expired - recommend retry/requeue", letter.LetterID.String()))
				pub.pool.ReturnChannel(chanHost, nil) // not a channel error
				return

			case confirmation := <-chanHost.Confirmations:

				if !confirmation.Ack {
					goto Publish //nack has occurred, republish
				}

				// Happy Path, publish was received by server and we didn't timeout client side.
				pub.publishReceipt(letter, nil)
				pub.pool.ReturnChannel(chanHost, nil)
				return

			default:

				time.Sleep(time.Duration(time.Millisecond * 1)) // limits CPU spin up
			}
		}
	}
}

// PublishWithConfirmationContextError sends a single message to the address on the letter with confirmation capabilities.
// This is an expensive and slow call - use this when delivery confirmation on publish is your highest priority.
// A timeout failure drops the letter back in the PublishReceipts.
// A confirmation failure keeps trying to publish (at least until timeout failure occurs.)
func (pub *Publisher) PublishWithConfirmationContextError(ctx context.Context, letter *Letter) error {

	for {
		// Has to use an Ackable channel for Publish Confirmations.
		chanHost, err := pub.pool.GetChannelFromPool()
		if err != nil {
			return fmt.Errorf("publish of LetterID: %s failed: %w", letter.LetterID.String(), err)
		}
		chanHost.FlushConfirms() // Flush all previous publish confirmations

	Publish:
		err = chanHost.Channel.Publish(
			letter.Envelope.Exchange,
			letter.Envelope.RoutingKey,
			letter.Envelope.Mandatory,
			letter.Envelope.Immediate,
			amqp.Publishing{
				ContentType:   letter.Envelope.ContentType,
				Body:          letter.Body,
				Headers:       letter.Envelope.Headers,
				DeliveryMode:  letter.Envelope.DeliveryMode,
				Priority:      letter.Envelope.Priority,
				MessageId:     letter.LetterID.String(),
				CorrelationId: letter.Envelope.CorrelationID,
				Type:          letter.Envelope.Type,
				Timestamp:     time.Now().UTC(),
				AppId:         pub.pool.Config.ApplicationName,
			},
		)
		if err != nil {
			pub.pool.ReturnChannel(chanHost, err)
			continue // Take it again! From the top!
		}

		// Wait for very next confirmation on this channel, which should be our confirmation.
		for {
			select {
			case <-ctx.Done():
				pub.pool.ReturnChannel(chanHost, nil) // not a channel error
				return fmt.Errorf("publish confirmation for LetterID: %s wasn't received before context expired - recommend retry/requeue", letter.LetterID.String())

			case confirmation := <-chanHost.Confirmations:

				if !confirmation.Ack {
					goto Publish //nack has occurred, republish
				}

				pub.pool.ReturnChannel(chanHost, nil)
				return nil

			default:

				time.Sleep(time.Duration(time.Millisecond * 1)) // limits CPU spin up
			}
		}
	}
}

// PublishWithConfirmationTransient sends a single message to the address on the letter with confirmation capabilities on transient Channels.
// This is an expensive and slow call - use this when delivery confirmation on publish is your highest priority.
// A timeout failure drops the letter back in the PublishReceipts. When combined with QueueLetter, it automatically
// gets requeued for re-publish.
// A confirmation failure keeps trying to publish (at least until timeout failure occurs.)
func (pub *Publisher) PublishWithConfirmationTransient(letter *Letter, timeout time.Duration) {

	if timeout == 0 {
		timeout = pub.publishTimeOutDuration
	}

	for {
		// Has to use an Ackable channel for Publish Confirmations.
		channel, err := pub.pool.GetTransientChannel(true)
		if err != nil {
			pub.publishReceipt(letter, fmt.Errorf("publish of LetterID: %s failed: %w", letter.LetterID.String(), err))
			return
		}
		confirms := make(chan amqp.Confirmation, 1)
		channel.NotifyPublish(confirms)

	Publish:
		timeoutAfter := time.After(timeout)
		err = channel.Publish(
			letter.Envelope.Exchange,
			letter.Envelope.RoutingKey,
			letter.Envelope.Mandatory,
			letter.Envelope.Immediate,
			amqp.Publishing{
				ContentType:   letter.Envelope.ContentType,
				Body:          letter.Body,
				Headers:       letter.Envelope.Headers,
				DeliveryMode:  letter.Envelope.DeliveryMode,
				Priority:      letter.Envelope.Priority,
				MessageId:     letter.LetterID.String(),
				CorrelationId: letter.Envelope.CorrelationID,
				Type:          letter.Envelope.Type,
				Timestamp:     time.Now().UTC(),
				AppId:         pub.pool.Config.ApplicationName,
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

	if !pub.isAutoStarted() {
		pub.setAutoStarted(true)
		pub.wg.Add(1)
		go pub.startAutoPublishingLoop()
	}
}

// StartAutoPublish starts auto-publishing letters queued up - is locking.
func (pub *Publisher) startAutoPublishingLoop() {
	defer pub.wg.Done()

	// Deliver letters queued in the publisher, returns true when we are to stop publishing.
	pub.deliverLetters()
	pub.setAutoStarted(false)
}

func (pub *Publisher) deliverLetters() {

	// Allow parallel publishing with transient channels.
	parallelPublishSemaphore := make(chan struct{}, pub.pool.Config.MaxCacheChannelCount/2+1)

	for {
		select {
		case <-pub.catchShutdown():
			return
		case letter, ok := <-pub.letters:
			if !ok {
				return
			}
			// Publish the letter.
			parallelPublishSemaphore <- struct{}{} // throttling
			pub.wg.Add(1)
			go func(letter *Letter) {
				defer pub.wg.Done()

				pub.PublishWithConfirmation(letter, pub.publishTimeOutDuration)
				<-parallelPublishSemaphore
			}(letter)
		}
	}
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
func (pub *Publisher) safeSend(letter *Letter) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()

	select {
	case <-pub.catchShutdown():
		return false
	case pub.letters <- letter:
		return true // success
	}
}

// publishReceipt sends the status to the receipt channel.
func (pub *Publisher) publishReceipt(l *Letter, e error) {
	pub.wg.Add(1)
	go func(letter *Letter, err error) {
		defer pub.wg.Done()

		publishReceipt := &PublishReceipt{
			LetterID: letter.LetterID,
			Error:    err,
		}

		if err == nil {
			publishReceipt.Success = true
		} else {
			publishReceipt.FailedLetter = letter
		}

		select {
		case <-pub.catchShutdown():
			// TODO: loosing receipt, potentially loosing
			// failed letter delivery
			fmt.Println("lost publishing receipt")
			return
		case pub.publishReceipts <- publishReceipt:
			// it's possible that in case the receipts are never received
			// that this will block forever, so we want to catch shutdowns
			return
		}

	}(l, e)
}

// Close cleanly shutdown the publisher.
// By default the internal connection pool is also closed.
func (pub *Publisher) Close(shutdownPools ...bool) {
	pub.once.Do(func() {
		closePool := true
		if len(shutdownPools) > 0 {
			closePool = shutdownPools[0]
		}

		close(pub.shutdownSignal)

		if closePool { // in case the ChannelPool is shared between structs, you can prevent it from shutting down
			pub.pool.Close()
		}

		// wait for all spawned goroutines to finish execution
		pub.wg.Wait()
		// all routines are down, now close the publisher
		// receipt channel
		close(pub.publishReceipts)
	})
}

func (pub *Publisher) isAutoStarted() bool {
	autoStarted := atomic.LoadInt32(&pub.autoStarted)
	return autoStarted != 0
}

func (pub *Publisher) setAutoStarted(autoStarted bool) {
	var i int32 = 0
	if autoStarted {
		i = 1
	}

	atomic.StoreInt32(&pub.autoStarted, i)
}

func (pub *Publisher) catchShutdown() <-chan struct{} {
	return pub.shutdownSignal
}
