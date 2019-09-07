# TurboCookedRabbit
 A user friendly RabbitMQ written in Golang.  
 Based on my work found at [CookedRabbit](https://github.com/houseofcat/CookedRabbit).

### Developer's Notes
It was programmed against the following:

 * Golang 1.13.0
 * RabbitMQ Server v3.7.17 (simple localhost)
 * Erlang v22.0 (OTP v10.4)

If you see any issues with more advanced setups, I will need an intimate description of the setup. Without it, I more than likely won't be able to resolve it. I can accept PRs if you want to test out fixes that resolve things for yourself.

I also don't have the kind of free time I used to, I apologize in advance but hey that's life. So keep in mind that I am not paid to do this - this isn't my job, this isn't a corporate sponsorship.

### Work In Progress
 * Solidify Connections/Pools
 * Solidify Consumers
 * A solid Demo Client + Docs
 * Chaos Engineer + More Tests

## The Seasoning/Config

The config is just a **quality of life** feature. You don't have to use it. I just like how easy it is to change configurations on the fly.

    config, err := utils.ConvertJSONFileToConfig("testconsumerseasoning.json")

The full structure `RabbitSeasoning` is available under `models/configs.go`

<details><summary>Click to a sample config.json!</summary>
<p>

    {
        "PoolConfig": {
            "ChannelPoolConfig": {
                "ErrorBuffer": 10,
                "BreakOnInitializeError": false,
                "MaxInitializeErrorCount": 5,
                "SleepOnErrorInterval": 50,
                "CreateChannelRetryCount": 5,
                "ChannelCount": 25,
                "AckChannelCount": 25,
                "GlobalQosCount": 4
            },
            "ConnectionPoolConfig": {
                "URI": "amqp://guest:guest@localhost:5672/",
                "ErrorBuffer": 1,
                "BreakOnInitializeError": false,
                "MaxInitializeErrorCount": 5,
                "SleepOnErrorInterval": 50,
                "CreateConnectionRetryCount": 5,
                "ConnectionCount": 5,
                "TLSConfig": {
                    "EnableTLS": false,
                    "PEMCertLocation": "test/catest.pem",
                    "LocalCertLocation": "client/cert.ca",
                    "CertServerName": "hostname-in-cert"
                }
            }
        },
        "ConsumerConfigs": {
            "TurboCookedRabbitConsumer-Ackable": {
                "QueueName": "ConsumerTestQueue",
                "ConsumerName": "TurboCookedRabbitConsumer-Ackable",
                "AutoAck": false,
                "Exclusive": false,
                "NoWait": true,
                "QosCountOverride": 5,
                "QosSizeOverride": 65535,
                "MessageBuffer": 10,
                "ErrorBuffer": 1,
                "SleepOnErrorInterval": 1000
            },
            "TurboCookedRabbitConsumer-AutoAck": {
                "QueueName": "ConsumerTestQueue",
                "ConsumerName": "TurboCookedRabbitConsumer-AutoAck",
                "AutoAck": true,
                "Exclusive": false,
                "NoWait": true,
                "QosCountOverride": 5,
                "QosSizeOverride": 65535,
                "MessageBuffer": 10,
                "ErrorBuffer": 1,
                "SleepOnErrorInterval": 1000
            }
        },
        "PublisherConfig":{
            "SleepOnIdleInterval": 1000,
            "LetterBuffer": 10,
            "NotificationBuffer": 10
        }
    }

</p>
</details>

## The Publisher

<details><summary>Click for creating publisher examples!</summary>
<p>

Assuming you have a **ChannelPool** already setup. Creating a publisher can be achieved as so:

    publisher, err := publisher.NewPublisher(Seasoning, channelPool, nil)

Assuming you have a **ChannelPool** and **ConnectionPool** setup. Creating a publisher can be achieved as so:

    publisher, err := publisher.NewPublisher(Seasoning, channelPool, connectionPool)

The errors here indicate I was unable to create a Publisher - probably due to the ChannelPool/ConnectionPool you gave me.

</p>
</details>

---

<details><summary>Click for simple publish example!</summary>
<p>

Once you have a publisher, you can perform a relatively simple publish.

    letter := utils.CreateLetter("", "TestQueueName", nil)
	publisher.Publish(letter)

This creates a simple HelloWorld message letter with no ExchangeName and a QueueName/RoutingKey of TestQueueName. The body is nil, the helper function creates bytes for "h e l l o   w o r l d".

The concept of a Letter may seem clunky on a single publish. I don't disagree and you still always have `streadway/amqp` to rely on. The **letter** idea makes sense with **AutoPublish**.

</p>
</details>

---

<details><summary>Click for AutoPublish example!</summary>
<p>

Once you have a publisher, you can perform **StartAutoPublish**!

    allowInternalRetry := false
	publisher.StartAutoPublish(allowInternalRetryMechanism)

    ListeningForNotificationsLoop:
	for {
		select {
		case notification := <-publisher.Notifications():
			if !notification.Success {
                /* Handle Republish */
            }
		default:
			time.Sleep(1 * time.Millisecond)
		}
	}

This tells the Publisher to start reading an **internal queue**, a letter queue.

Once this has been started up, we allow letters to be placed in the mailbox/letter queue.

That could be simple like this...

    publisher.QueueLetter(letter) // How simple is that!

...or more complex like such

    for _, letter := range letters {
        err := publisher.QueueLetter(letter)
        if err != nil {
            /* Handle Retry To Add To Queue */
        }
    }

So you can see why we use these message containers called **letter**. The letter has the **body** and **envelope** inside of it. It has everything you need to publish it. Think of it a small, highly configurable, **unit of work**.

Notice that you don't have anything to do with channels and connections!

</p>
</details>

---

<details><summary>Click for a more convoluted AutoPublish example!</summary>
<p>

Let's say the above example was too simple for you... ...let's up it a notch on what you can do with AutoPublish.

    allowInternalRetry := true
	publisher.StartAutoPublish(allowInternalRetryMechanism) // this will retry based on the Letter.RetryCount passed in.

	timer := time.NewTimer(1 * time.Minute) // Stop Listening to notifications after 1 minute.

    messageCount = 1000
    channelFailureCount := 0
	successCount := 0
	failureCount := 0

    ListeningForNotificationsLoop:
	for {
		select {
		case <-timer.C:
			break ListeningForNotificationsLoop  
		case chanErr := <-channelPool.Errors():
			if chanErr != nil {
				channelFailureCount++ // Count ChannelPool failures.
			}
			break
		case notification := <-publisher.Notifications():
			if notification.Success {
				successCount++
			} else {
				failureCount++
			}

            // I am only expecting to publish 1000 messages
			if successCount+failureCount == messageCount { 
				break ListeningForNotificationsLoop
			}

			break
		default:
			time.Sleep(1 * time.Millisecond)
			break
		}
	}

We have finished our work, we **succeeded** or **failed** to publish **1000** messages. So now we want to shutdown everything!

	publisher.StopAutoPublish()
	// channelPool.Shutdown() // if you have a pointer to your channel pool nearby!

</p>
</details>

---

## The Consumer

<details><summary>Click for a Consumer example!</summary>
<p>

Again, the ConsumerConfig is just a **quality of life** feature. You don't have to use it.

```
    ...
	"ConsumerConfigs": {
		"TurboCookedRabbitConsumer-Ackable": {
			"QueueName": "ConsumerTestQueue",
			"ConsumerName": "TurboCookedRabbitConsumer-Ackable",
			"AutoAck": false,
			"Exclusive": false,
			"NoWait": true,
			"QosCountOverride": 5,
			"QosSizeOverride": 65535,
			"MessageBuffer": 10,
			"ErrorBuffer": 1,
			"SleepOnErrorInterval": 1000
		},
		"TurboCookedRabbitConsumer-AutoAck": {
			"QueueName": "ConsumerTestQueue",
			"ConsumerName": "TurboCookedRabbitConsumer-AutoAck",
			"AutoAck": true,
			"Exclusive": false,
			"NoWait": true,
			"QosCountOverride": 5,
			"QosSizeOverride": 65535,
			"MessageBuffer": 10,
			"ErrorBuffer": 1,
			"SleepOnErrorInterval": 1000
		}
	},
    ...
```

And finding this object after it was loaded from a JSON file.

	consumerConfig, ok := config.ConsumerConfigs["TurboCookedRabbitConsumer-AutoAck"]

Creating the Consumer from Config after creating a ChannelPool.

	consumer, err := consumer.NewConsumerFromConfig(consumerConfig, channelPool)

Then start Consumer?

    consumer.StartConsuming()

Thats it! Wait where our my messages?! MY QUEUE IS DRAINING!

Oh, right! That's over here, keeping with the *out of process design*.

    ConsumeMessages:
	for {
		select {
		case message := <-consumer.Messages():
            /* Do something with the message! */
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}

</p>
</details>

---

<details><summary>Wait! What the hell is coming out of <-Messages()</summary>
<p>

Great question. I toyed with the idea of retruning Letters (and I may still at some point) but for now you receive a `models.Message`.

Why? Because the payload/data/message body is here but more importantly contains the means of acking the message! It didn't feel right being merged with a `models.Letter`.

One of the complexities of RabbitMQ is that you need to Acknowledge off the same Channel that it was received on. That makes out of process designs like mine prone to two things: hackery and/or memory leaks (passing the channels around everywhere WITH messages).

There are two things I **hate** about RabbitMQ
 * Channels close on error.
 * Messages have to be acknowledge on the same channel.

What I have attempted to do is to make your life blissful by not forcing you to deal with it. The rules are still there, but hopefully, I give you the tools to not stress out about it and to simplify **out of process** acknowledgements.

That being said, there is only so much I can hide in my library, which is why I have exposed .Errors(), so that you can code accordingly.

	err := consumer.StartConsuming()
	// Handle failure to start.

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(1)*time.Minute) // Timeouts

    ConsumeMessages:
	for {
		select {
		case <-ctx.Done():
			fmt.Print("\r\nContextTimeout\r\n")
			break ConsumeMessages
		case message := <-consumer.Messages(): // View Messages
			fmt.Printf("Message Received: %s\r\n", string(message.Body))
		case err := <-consumer.Errors(): // View Consumer errors
			/* Handle */
		case err := <-channelPool.Errors(): // View ChannelPool errors
			/* Handle */
		default:
			time.Sleep(100 * time.Millisecond)
			break
		}
	}

Here you may trigger StopConsuming with this

	consumer.StopConsuming(false)

But be mindful there are Channel Buffers internally that may be full and goroutines waiting to add even more.

I have provided some tools that can be used to help with this. You will see them sprinkled periodically through my tests.

    consumer.FlushStop() // could have been called more than once.
    consumer.FlushErrors() // errors can quickly build up if you stop listening to them
    consumer.FlushMessages() // lets say the ackable messages you have can't be acked and you just need to flush them all out of memory

Becareful with FlushMessages(). If you are `autoAck = false` and receiving ackAble messages, this is safe. You will merely **wipe them from your memory** and ***they are still in the original queue***.

</p>
</details>

---

## The Pools

<details><summary>ChannelPools, how do they even work?!</summary>
<p>

CommingSoon™

</p>
</details>