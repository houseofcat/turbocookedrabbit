package publisher

import (
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
	notifications chan *models.Notification
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
		notifications: make(chan *models.Notification, 1),
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
func (pub *Publisher) Publish(letter *models.Letter, retryCount uint32) {

	chanHost, err := pub.ChannelPool.GetChannel()
	if err != nil {
		pub.sendToNotifications(letter.LetterID, err)
		return
	}

PublishWithRetry:
	for i := retryCount + 1; i > 0; i-- {
		pubErr := pub.simplePublish(chanHost.Channel, letter)
		if pubErr != nil {
		GetChannelRetry:
			for {
				if i > 0 {
					chanHost, err = pub.ChannelPool.GetChannel()
					if err != nil {
						pub.sendToNotifications(letter.LetterID, err)

						time.Sleep(50 * time.Millisecond)
						i-- // decrement retry count
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

// Notifications yields all the success and failures during publish events.
func (pub *Publisher) Notifications() <-chan *models.Notification {
	return pub.notifications
}
