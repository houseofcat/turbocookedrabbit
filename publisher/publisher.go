package publisher

import (
	"fmt"
	"sync"
	"time"

	"github.com/houseofcat/turbocookedrabbit/models"
	"github.com/houseofcat/turbocookedrabbit/pools"

	"github.com/streadway/amqp"
)

// Publisher contains everything you need to publish a message.
type Publisher struct {
	Config                   *models.RabbitSeasoning
	ChannelPool              *pools.ChannelPool
	letters                  chan *models.Letter
	autoStop                 chan bool
	publishReceipts          chan *models.PublishReceipt
	autoStarted              bool
	autoPublishGroup         *sync.WaitGroup
	sleepOnIdleInterval      time.Duration
	sleepOnQueueFullInterval time.Duration
	sleepOnErrorInterval     time.Duration
	pubLock                  *sync.Mutex
	pubRWLock                *sync.RWMutex
}

// NewPublisher creates and configures a new Publisher.
func NewPublisher(
	config *models.RabbitSeasoning,
	chanPool *pools.ChannelPool,
	connPool *pools.ConnectionPool) (*Publisher, error) {

	// If nil, create your own isolated ChannelPool based on configuration settings.
	if chanPool == nil {
		var err error
		chanPool, err = pools.NewChannelPool(config.PoolConfig, connPool, true)
		if err != nil {
			return nil, err
		}
	}

	return &Publisher{
		Config:                   config,
		ChannelPool:              chanPool,
		letters:                  make(chan *models.Letter),
		autoStop:                 make(chan bool, 1),
		autoPublishGroup:         &sync.WaitGroup{},
		publishReceipts:          make(chan *models.PublishReceipt),
		sleepOnIdleInterval:      time.Duration(config.PublisherConfig.SleepOnIdleInterval) * time.Millisecond,
		sleepOnQueueFullInterval: time.Duration(config.PublisherConfig.SleepOnQueueFullInterval) * time.Millisecond,
		sleepOnErrorInterval:     time.Duration(config.PublisherConfig.SleepOnErrorInterval) * time.Millisecond,
		pubLock:                  &sync.Mutex{},
		pubRWLock:                &sync.RWMutex{},
		autoStarted:              false,
	}, nil
}

// Publish sends a single message to the address on the letter.
// Subscribe to PublishReceipts to see success and errors.
// For proper resilience (at least once delivery guarantee over shaky network) use PublishWithConfirmation
func (pub *Publisher) Publish(letter *models.Letter) {

	var chanHost *pools.ChannelHost
	var err error

	// Loop till we can get a channel!
	for {
		if pub.Config.PublisherConfig.AutoAck {
			chanHost, err = pub.ChannelPool.GetChannel()
		} else {
			chanHost, err = pub.ChannelPool.GetAckableChannel()
		}

		if err != nil {
			// throttle on error
			time.Sleep(time.Millisecond * pub.sleepOnErrorInterval)
			continue
		}
		break
	}

	err = pub.simplePublish(chanHost.Channel, letter)
	pub.finalize(err, letter, chanHost)
}

// PublishWithConfirmation sends a single message to the address on the letter with confirmation capabilities.
// This is an expensive and slow call - use this when delivery confirmation on publish is your highest priority.
// Only uses transient channels due to the complex nature of subcribing to the return notification.
func (pub *Publisher) PublishWithConfirmation(letter *models.Letter) error {

GetChannelAndPublish:
	for {
		chanHost := pub.ChannelPool.GetTransientChannel(true)
		chanHost.Channel.NotifyPublish(chanHost.Confirmations)

		err := pub.simplePublish(chanHost.Channel, letter)
		if err != nil {
			chanHost.Channel.Close()
			goto GetChannelAndPublish
		}

		counter := 0 // exit condition
		for {
			select {
			case confirmation := <-chanHost.Confirmations:

				if !confirmation.Ack { // retry publish
					chanHost.Channel.Close()
					goto GetChannelAndPublish
				}

				pub.publishReceipt(letter, nil)
				chanHost.Channel.Close()
				break GetChannelAndPublish

			default:

				if counter == 100 {
					chanHost.Channel.Close()
					// TODO: Send to notifications. Needs to be re-PublishWithConfirmation which isn't supported yet.
					return fmt.Errorf("publish confirmation for LetterId: %d wasn't received in a timely manner (300ms) - recommend retry", letter.LetterID)
				}
				counter++
				time.Sleep(time.Duration(time.Millisecond * 3))
			}
		}
	}

	return nil
}

func (pub *Publisher) finalize(err error, letter *models.Letter, chanHost *pools.ChannelHost) {

	errorOccurred := err != nil

	pub.ChannelPool.ReturnChannel(chanHost, errorOccurred)
	pub.publishReceipt(letter, err)

	if errorOccurred {
		time.Sleep(pub.sleepOnErrorInterval * time.Millisecond)
	}
}

// PublishReceipts yields all the success and failures during all publish events. Highly recommend susbscribing to this.
func (pub *Publisher) PublishReceipts() <-chan *models.PublishReceipt {
	return pub.publishReceipts
}

// StartAutoPublish starts auto-publishing letters queued up - is locking.
func (pub *Publisher) StartAutoPublish() {
	pub.FlushStops()

	if !pub.autoStarted {

		go func() {
		PublishLoop:
			for {
				select {
				case stop := <-pub.autoStop:
					if stop {
						break PublishLoop
					}
				default:
					break
				}

				select {
				case letter := <-pub.letters:
					pub.autoPublishGroup.Add(1)

					go func() {
						defer pub.autoPublishGroup.Done()
						pub.Publish(letter)
					}()

				default:
					if pub.sleepOnIdleInterval > 0 {
						time.Sleep(pub.sleepOnIdleInterval)
					}
					break
				}
			}

			pub.autoPublishGroup.Wait() // let all remaining publishes finish.

			pub.pubLock.Lock()
			pub.autoStarted = false
			pub.pubLock.Unlock()
		}()

		pub.pubLock.Lock()
		pub.autoStarted = true
		pub.pubLock.Unlock()
	}
}

// StopAutoPublish stops publishing letters queued up - is locking.
func (pub *Publisher) StopAutoPublish() {
	pub.pubLock.Lock()
	defer pub.pubLock.Unlock()

	if !pub.autoStarted {
		return
	}

	go func() { pub.autoStop <- true }() // signal auto publish to stop
}

// QueueLetters allows you to bulk queue letters that will be consumed by AutoPublish.
// Blocks on the Letter Buffer being full.
func (pub *Publisher) QueueLetters(letters []*models.Letter) {

	for _, letter := range letters {

		pub.letters <- letter
	}
}

// QueueLetter queues up a letter that will be consumed by AutoPublish.
// Blocks on the Letter Buffer being full.
func (pub *Publisher) QueueLetter(letter *models.Letter) {

	pub.letters <- letter
}

// SimplePublish performs the actual amqp.Publish.
func (pub *Publisher) simplePublish(amqpChan *amqp.Channel, letter *models.Letter) error {

	return amqpChan.Publish(
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
}

// TODO ConfirmationPublish
func (pub *Publisher) complexPublish(amqpChan *amqp.Channel, letter *models.Letter) error {

	return amqpChan.Publish(
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
}

// publishReceipt sends the status to the receipt channel.
func (pub *Publisher) publishReceipt(letter *models.Letter, err error) {

	publishReceipt := &models.PublishReceipt{
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
		pub.ChannelPool.Shutdown()
	}
}
