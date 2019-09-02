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
func NewPublisher(seasoning *models.RabbitSeasoning, chanPool *pools.ChannelPool, connPool *pools.ConnectionPool) (*Publisher, error) {

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
		letters:       make(chan *models.Letter, 1),
		autoStop:      make(chan bool, 1),
		notifications: make(chan *models.Notification, 1),
		smallSleep:    time.Duration(50) * time.Millisecond,
		pubLock:       &sync.Mutex{},
		autoStopped:   true,
	}, nil
}

// Shutdown cleanly shutsdown the publisher and resets it's internal state.
func (pub *Publisher) Shutdown(skipPools bool) {
	if !skipPools { // in case the ChannelPool is shared between structs, you can prevent it from shuttingdown
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

// Notifications yields all the success and failures during all publish events.
func (pub *Publisher) Notifications() <-chan *models.Notification {
	return pub.notifications
}

// StartAutoPublish starts auto-publishing letters queued up.
func (pub *Publisher) StartAutoPublish() {
	go func() {
		select {
		case stop := <-pub.autoStop:
			if stop {
				break
			}
		case letter := <-pub.letters:
			{
				pub.Publish(letter)
			}
		default:
			time.Sleep(pub.smallSleep)
		}
	}()
}

// StopAutoPublish stops publishing letters queued up.
// Immediate true option immediately returns to caller without draining the queue.
// Immediate false option immediately returns to the caller but drains the letter queue in the background
// and you can check their statuses still in Notifications.
func (pub *Publisher) StopAutoPublish(immediate bool) {
	go func() {
		if immediate {
			pub.autoStop <- true
			pub.setAutoPublishStopped(true)
		} else {
			pub.autoStop <- true            // signal auto publish to stop
			pub.setAutoPublishStopped(true) // prevent new letters being queued

			// allow it to finish publishing all remaining letters
			select {
			case letter := <-pub.letters:
				{
					pub.Publish(letter)
				}
			default:
				break
			}
		}
	}()
}

// QueueLetter queues up a letter that will be consumed by AutoPublish.
// Error signals that the AutoPublish
func (pub *Publisher) QueueLetter(letter *models.Letter) error {
	if pub.getAutoPublishStopped() {
		return errors.New("can't add letters to the internal queue if AutoPublish has not been started")
	}

	go func() { pub.letters <- letter }()
	return nil
}

func (pub *Publisher) getAutoPublishStopped() bool {
	pub.pubLock.Lock()
	defer pub.pubLock.Unlock()

	return pub.autoStopped
}

func (pub *Publisher) setAutoPublishStopped(autoStopped bool) {
	pub.pubLock.Lock()
	defer pub.pubLock.Unlock()

	pub.autoStopped = autoStopped
}
