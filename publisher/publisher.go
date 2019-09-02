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
	letters       chan *models.Letter
	autoStop      chan bool
	notifications chan *models.Notification
	autoStopped   bool
	smallSleep    time.Duration
	pubLock       *sync.Mutex
}

// NewPublisher creates and configures a new Publisher.
func NewPublisher(
	seasoning *models.RabbitSeasoning,
	chanPool *pools.ChannelPool,
	connPool *pools.ConnectionPool,
	sleepDuration uint32) (*Publisher, error) {

	// If nil, create your own isolated ChannelPool based on configuration settings.
	if chanPool == nil {
		var err error // If a connpool is nil, this will create an internal one here.
		chanPool, err = pools.NewChannelPool(seasoning, connPool, true)
		if err != nil {
			return nil, err
		}
	}

	return &Publisher{
		Config:        seasoning,
		ChannelPool:   chanPool,
		letters:       make(chan *models.Letter, 10),
		autoStop:      make(chan bool, 1),
		notifications: make(chan *models.Notification, 1),
		smallSleep:    time.Duration(sleepDuration) * time.Millisecond,
		pubLock:       &sync.Mutex{},
		autoStopped:   true,
	}, nil
}

// Shutdown cleanly shutsdown the publisher and resets it's internal state.
func (pub *Publisher) Shutdown(shutdownPools bool) {
	pub.StopAutoPublish(true)

	if shutdownPools { // in case the ChannelPool is shared between structs, you can prevent it from shuttingdown
		pub.ChannelPool.Shutdown()
	}
}

// Publish sends a single message to the address on the letter.
// Subscribe to Notifications to see success and errors.
func (pub *Publisher) Publish(letter *models.Letter) {

	chanHost, err := pub.ChannelPool.GetChannel()
	if err != nil {
		pub.sendToNotifications(letter.LetterID, err)
		return
	}

PublishWithRetry:
	for i := letter.RetryCount + 1; i > 0; i-- {
		pubErr := pub.simplePublish(chanHost.Channel, letter)
		if pubErr != nil {
		GetChannelRetry:
			for {
				if i > 0 {
					chanHost, err = pub.ChannelPool.GetChannel()
					if err != nil {
						pub.sendToNotifications(letter.LetterID, err)

						time.Sleep(pub.smallSleep)
						i-- // failure to acquire a new channel counts against the retry count
					} else {
						break GetChannelRetry
					}
				} else {
					break PublishWithRetry
				}
			}
		} else {
			pub.sendToNotifications(letter.LetterID, nil)
			break PublishWithRetry
		}
	}
}

// Notifications yields all the success and failures during all publish events.
func (pub *Publisher) Notifications() <-chan *models.Notification {
	return pub.notifications
}

// StartAutoPublish starts auto-publishing letters queued up.
func (pub *Publisher) StartAutoPublish() {
	pub.pubLock.Lock()
	defer pub.pubLock.Unlock()

FlushLoop:
	for {
		select {
		case <-pub.autoStop:
			// flush the autostops before start
		default:
			break FlushLoop
		}
	}

	go func() {
	PublishLoop:
		for {
			select {
			case stop := <-pub.autoStop:
				if stop {
					break PublishLoop
				}
			case letter := <-pub.letters:
				go pub.Publish(letter)
			default:
				time.Sleep(pub.smallSleep)
			}
		}
	}()

	pub.autoStopped = false
}

// StopAutoPublish stops publishing letters queued up.
// Immediate true option returns to caller after draining the queue.
// Immediate false option immediately returns to the caller but drains the letter queue in the background
// and you can check their statuses still in Notifications.
func (pub *Publisher) StopAutoPublish(immediate bool) {
	if immediate {
		pub.stopAutoPublish()
	} else {
		go pub.stopAutoPublish()
	}
}

func (pub *Publisher) stopAutoPublish() {
	pub.pubLock.Lock()
	defer pub.pubLock.Unlock()

	if pub.autoStopped { // Invalid/Multiple/Subsequent calls exit out
		return
	}

	pub.autoStopped = true               // prevent new letters being queued
	go func() { pub.autoStop <- true }() // signal auto publish to stop

	// allow it to finish publishing all remaining letters
FlushLoop:
	for {
		select {
		case letter := <-pub.letters:
			go pub.Publish(letter)
		default:
			break FlushLoop
		}
	}
}

// QueueLetter queues up a letter that will be consumed by AutoPublish.
// Error signals that the AutoPublish
func (pub *Publisher) QueueLetter(letter *models.Letter) error {
	if pub.autoPublishStopped() {
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

func (pub *Publisher) autoPublishStopped() bool {
	pub.pubLock.Lock()
	defer pub.pubLock.Unlock()

	return pub.autoStopped
}
