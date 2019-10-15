package publisher

import (
	"sync"
	"sync/atomic"
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
	letterCount              uint64
	letterBuffer             uint64
	maxOverBuffer            uint64
	autoStop                 chan bool
	notifications            chan *models.Notification
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
		letters:                  make(chan *models.Letter, config.PublisherConfig.LetterBuffer),
		letterBuffer:             config.PublisherConfig.LetterBuffer,
		maxOverBuffer:            config.PublisherConfig.MaxOverBuffer,
		autoStop:                 make(chan bool, 1),
		autoPublishGroup:         &sync.WaitGroup{},
		notifications:            make(chan *models.Notification, config.PublisherConfig.NotificationBuffer),
		sleepOnIdleInterval:      time.Duration(config.PublisherConfig.SleepOnIdleInterval) * time.Millisecond,
		sleepOnQueueFullInterval: time.Duration(config.PublisherConfig.SleepOnQueueFullInterval) * time.Millisecond,
		sleepOnErrorInterval:     time.Duration(config.PublisherConfig.SleepOnErrorInterval) * time.Millisecond,
		pubLock:                  &sync.Mutex{},
		pubRWLock:                &sync.RWMutex{},
		autoStarted:              false,
	}, nil
}

// Publish sends a single message to the address on the letter.
// Subscribe to Notifications to see success and errors.
func (pub *Publisher) Publish(letter *models.Letter) {

	chanHost, err := pub.ChannelPool.GetChannel()
	if err != nil {
		pub.sendToNotifications(letter.LetterID, err)
		return // exit out if you can't get a channel
	}

	err = pub.simplePublish(chanHost.Channel, letter)
	if err != nil {
		pub.handleErrorAndChannel(err, letter.LetterID, chanHost)
	} else {
		pub.sendToNotifications(letter.LetterID, err)
		pub.ChannelPool.ReturnChannel(chanHost, false)
	}
}

// PublishWithRetry sends a single message to the address on the letter with retry capabilities.
// Subscribe to Notifications to see success and errors.
// RetryCount is based on the letter property. Zero means it will try once.
func (pub *Publisher) PublishWithRetry(letter *models.Letter) {

	for i := letter.RetryCount + 1; i > 0; i-- {
		chanHost, err := pub.ChannelPool.GetChannel()
		if err != nil {
			time.Sleep(pub.sleepOnErrorInterval * time.Millisecond)
			continue // can't get a channel
		}

		err = pub.simplePublish(chanHost.Channel, letter)
		if err != nil {
			pub.handleErrorAndChannel(err, letter.LetterID, chanHost)
			continue // flag channel and try again
		}

		pub.sendToNotifications(letter.LetterID, err)
		pub.ChannelPool.ReturnChannel(chanHost, false)
		break // finished
	}
}

func (pub *Publisher) handleErrorAndChannel(err error, letterID uint64, chanHost *pools.ChannelHost) {
	pub.ChannelPool.ReturnChannel(chanHost, true)
	pub.sendToNotifications(letterID, err)
	time.Sleep(pub.sleepOnErrorInterval * time.Millisecond)
}

// Notifications yields all the success and failures during all publish events. Highly recommend susbscribing to this.
// Buffer will block if not consumed and leave goroutines stuck.
func (pub *Publisher) Notifications() <-chan *models.Notification {
	return pub.notifications
}

// StartAutoPublish starts auto-publishing letters queued up - is locking.
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
			default:
				break
			}

			select {
			case letter := <-pub.letters:
				pub.autoPublishGroup.Add(1)

				go func() {
					defer pub.autoPublishGroup.Done()
					if allowRetry {
						pub.PublishWithRetry(letter)
					} else {
						pub.Publish(letter)
					}

					pub.reduceLetterCount()
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
		defer pub.pubLock.Unlock()
		pub.autoStarted = false
	}()

	pub.pubLock.Lock()
	defer pub.pubLock.Unlock()
	pub.autoStarted = true
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

	for i := 0; i < len(letters); i++ {
		// Loop here until (buffer + maxOverBuffer) has room for you.
		for atomic.LoadUint64(&pub.letterCount) >= (pub.letterBuffer + pub.maxOverBuffer) {
			time.Sleep(pub.sleepOnQueueFullInterval)
			continue
		}

		pub.queueLetter(letters[i])
	}
}

// QueueLetter queues up a letter that will be consumed by AutoPublish.
// Blocks on the Letter Buffer being full.
func (pub *Publisher) QueueLetter(letter *models.Letter) {

	// Loop here until (buffer + maxOverBuffer) has room for you.
	for atomic.LoadUint64(&pub.letterCount) >= (pub.letterBuffer + pub.maxOverBuffer) {
		time.Sleep(pub.sleepOnQueueFullInterval)
		continue
	}

	pub.queueLetter(letter)
}

func (pub *Publisher) queueLetter(letter *models.Letter) {
	pub.increaseLetterCount()
	pub.letters <- letter
}

// IncreaseLetterCount decreases internal letter count - used to minimize outage CPU/Mem spin up on a blocked channel.
func (pub *Publisher) increaseLetterCount() {
	pub.pubRWLock.Lock()
	pub.letterCount++
	pub.pubRWLock.Unlock()
}

// ReduceLetterCount decreases internal letter count - used to minimize outage CPU/Mem spin up on a blocked channel.
func (pub *Publisher) reduceLetterCount() {
	pub.pubRWLock.Lock()
	pub.letterCount--
	pub.pubRWLock.Unlock()
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

// AutoPublishStarted allows you to see if the AutoPublish feature has started - is locking.
func (pub *Publisher) AutoPublishStarted() bool {
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

// Shutdown cleanly shutsdown the publisher and resets it's internal state.
func (pub *Publisher) Shutdown(shutdownPools bool) {
	pub.StopAutoPublish()

	if shutdownPools { // in case the ChannelPool is shared between structs, you can prevent it from shuttingdown
		pub.ChannelPool.Shutdown()
	}
}
