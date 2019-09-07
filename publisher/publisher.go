package publisher

import (
	"errors"
	"sync"
	"time"

	"github.com/houseofcat/turbocookedrabbit/models"
	"github.com/houseofcat/turbocookedrabbit/pools"

	"github.com/streadway/amqp"
)

// Publisher contains everything you need to publish a message.
type Publisher struct {
	Config        *models.RabbitSeasoning
	ChannelPool   *pools.ChannelPool
	publishGroup  *sync.WaitGroup
	letters       chan *models.Letter
	autoStop      chan bool
	notifications chan *models.Notification
	autoStarted   bool
	smallSleep    time.Duration
	pubLock       *sync.Mutex
}

// NewPublisher creates and configures a new Publisher.
func NewPublisher(
	config *models.RabbitSeasoning,
	chanPool *pools.ChannelPool,
	connPool *pools.ConnectionPool,
	sleepDuration uint32) (*Publisher, error) {

	// If nil, create your own isolated ChannelPool based on configuration settings.
	if chanPool == nil {
		var err error
		chanPool, err = pools.NewChannelPool(config.PoolConfig, connPool, true)
		if err != nil {
			return nil, err
		}
	}

	return &Publisher{
		Config:        config,
		ChannelPool:   chanPool,
		publishGroup:  &sync.WaitGroup{},
		letters:       make(chan *models.Letter, 10),
		autoStop:      make(chan bool, 1),
		notifications: make(chan *models.Notification, 10),
		smallSleep:    time.Duration(sleepDuration) * time.Millisecond,
		pubLock:       &sync.Mutex{},
		autoStarted:   false,
	}, nil
}

// Shutdown cleanly shutsdown the publisher and resets it's internal state.
func (pub *Publisher) Shutdown(shutdownPools bool) {
	pub.StopAutoPublish()

	if shutdownPools { // in case the ChannelPool is shared between structs, you can prevent it from shuttingdown
		pub.ChannelPool.Shutdown()
	}
}

// Publish sends a single message to the address on the letter.
// Subscribe to Notifications to see success and errors.
func (pub *Publisher) Publish(letter *models.Letter) {
	pub.publishGroup.Add(1)
	defer pub.publishGroup.Done()

	chanHost, err := pub.ChannelPool.GetChannel()
	if err != nil {
		pub.sendToNotifications(letter.LetterID, err)
		return // exit out if you can't get a channel
	}

	pubErr := pub.simplePublish(chanHost.Channel, letter)
	pub.sendToNotifications(letter.LetterID, pubErr)
}

// PublishWithRetry sends a single message to the address on the letter with retry capabilities.
// Subscribe to Notifications to see success and errors.
// RetryCount is based on the letter property. Zero means it will try once.
func (pub *Publisher) PublishWithRetry(letter *models.Letter) {
	pub.publishGroup.Add(1)
	defer pub.publishGroup.Done()

	chanHost, err := pub.ChannelPool.GetChannel()
	if err != nil {
		pub.sendToNotifications(letter.LetterID, err)
		return // exit out if you can't get a channel
	}

	for i := letter.RetryCount + 1; i > 0; i-- {
		pubErr := pub.simplePublish(chanHost.Channel, letter)
		if pubErr != nil {
			chanHost, err = pub.ChannelPool.GetChannel()
			if err != nil {
				pub.sendToNotifications(letter.LetterID, err)
				break // break out if you can't get a channel
			}
			continue // try again
		}
		pub.sendToNotifications(letter.LetterID, pubErr)
		break // finished
	}
}

// Notifications yields all the success and failures during all publish events.
func (pub *Publisher) Notifications() <-chan *models.Notification {
	return pub.notifications
}

// StartAutoPublish starts auto-publishing letters queued up.
func (pub *Publisher) StartAutoPublish(allowRetry bool) {
	pub.FlushStops()

	go func() {
	PublishLoop:
		for {
			select {
			case stop := <-pub.autoStop:
				if stop {
					break PublishLoop
				}
			case letter := <-pub.letters:
				if allowRetry {
					go pub.PublishWithRetry(letter)
				} else {
					go pub.Publish(letter)
				}
			default:
				time.Sleep(pub.smallSleep)
			}
		}

		pub.publishGroup.Wait() // let all remaining publishes finish.

		pub.pubLock.Lock()
		defer pub.pubLock.Unlock()
		pub.autoStarted = false
	}()

	pub.pubLock.Lock()
	defer pub.pubLock.Unlock()
	pub.autoStarted = true
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

// QueueLetter queues up a letter that will be consumed by AutoPublish.
// Error signals that the AutoPublish
func (pub *Publisher) QueueLetter(letter *models.Letter) error {
	if !pub.autoPublishStarted() {
		return errors.New("can't add letters to the internal queue if AutoPublish has not been started")
	}

	go func() { pub.letters <- letter }()
	return nil
}

func (pub *Publisher) simplePublish(amqpChan *amqp.Channel, letter *models.Letter) error {
	return amqpChan.Publish(
		letter.Envelope.Exchange,
		letter.Envelope.RoutingKey,
		letter.Envelope.Mandatory,
		letter.Envelope.Immediate,
		amqp.Publishing{
			ContentType: letter.Envelope.ContentType,
			Body:        letter.Body,
		},
	)
}

// SendToNotifications sends the status to the notifications channel.
func (pub *Publisher) sendToNotifications(letterID uint64, err error) {

	notification := &models.Notification{
		LetterID: letterID,
		Error:    err,
	}

	if err == nil {
		notification.Success = true
	}

	go func() { pub.notifications <- notification }()
}

func (pub *Publisher) autoPublishStarted() bool {
	pub.pubLock.Lock()
	defer pub.pubLock.Unlock()

	return pub.autoStarted
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
