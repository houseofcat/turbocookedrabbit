# TurboCookedRabbit
 A user friendly RabbitMQ written in Golang.  
 Based on my work found at [CookedRabbit](https://github.com/houseofcat/CookedRabbit).

[![Go Report Card](https://goreportcard.com/badge/github.com/houseofcat/turbocookedrabbit)](https://goreportcard.com/report/github.com/houseofcat/turbocookedrabbit)

### Developer's Notes
It was programmed against the following:

 * Golang 1.13.0
 * RabbitMQ Server v3.7.17 (simple localhost)
 * Erlang v22.0 (OTP v10.4)

If you see any issues with more advanced setups, I will need an intimate description of the setup. Without it, I more than likely won't be able to resolve it. I can accept PRs if you want to test out fixes that resolve things for yourself.

I also don't have the kind of free time I used to. I apologize in advance but, hey, that's life. So keep in mind that I am not paid to do this - this isn't my job, this isn't a corporate sponsorship.

Also if you see something syntactically wrong, speak up! I am, relatively speaking, an idiot. Also, I am still new to the golang ecosystem. My background is in infrastructure development, C#, and the .NET/NetCore ecosystem, so if any `golang wizards` want to provide advice, please do.

### Basic Performance

JSON Config Used: `testseason.json`
Benchmark Ran: `BenchmarkPublishConsumeAckForDuration` in `main_bench_test.go`

    1x Publisher AutoPublish ~ 500-650 msg/s - Single Queue - Small Message Sizes
    1x Consumer Consumption ~ 2000-3000 msg/s - Single Queue - Small Message Sizes

### Work Recently Finished
 * Solidify Connections/Pools
 * Solidify Consumers
 * Properly handle total outages server side.
 * Refactor the reconnection logic.
   * Now everything stops/pauses until connectivity is restored.
 * RabbitMQ Topology Creation/Destruction Support
 * Started Profiling and including Benchmark/Profile .svgs.

### Current Known Issues
 * ~CPU/MEM spin up on a total server outage even after adjusting config.~
   * ~Channels in ChannelPool aren't redistributing evenly over Connections.~
 * ~Consumer stops working after server outage restore.~
   * ~Publisher is still working though.~
 * Publisher is a tad on the slow side. Might be an underlying Queue datastructure.
 * README needs small comments/updates related to new work (9/13/2019 - 7:10 PM EST)

### Work In Progress
 * Streamline error handling.
 * More documentation.
 * A solid Demo Client
 * More Chaos Engineering / More Tests

## The Seasoning/Config

The config.json is just a **quality of life** feature. You don't have to use it. I just like how easy it is to change configurations on the fly.

```golang
config, err := utils.ConvertJSONFileToConfig("testconsumerseasoning.json")
```

The full structure `RabbitSeasoning` is available under `models/configs.go`

<details><summary>Click here to see a sample config.json!</summary>
<p>

```javascript
{
	"PoolConfig": {
		"ChannelPoolConfig": {
			"ErrorBuffer": 10,
			"SleepOnErrorInterval": 1000,
			"MaxChannelCount": 50,
			"MaxAckChannelCount": 50,
			"AckNoWait": false,
			"GlobalQosCount": 5
		},
		"ConnectionPoolConfig": {
			"URI": "amqp://guest:guest@localhost:5672/",
			"ErrorBuffer": 10,
			"SleepOnErrorInterval": 5000,
			"MaxConnectionCount": 10,
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
			"NoWait": false,
			"QosCountOverride": 5,
			"MessageBuffer": 100,
			"ErrorBuffer": 10,
			"SleepOnErrorInterval": 100,
			"SleepOnIdleInterval": 0
		},
		"TurboCookedRabbitConsumer-AutoAck": {
			"QueueName": "ConsumerTestQueue",
			"ConsumerName": "TurboCookedRabbitConsumer-AutoAck",
			"AutoAck": true,
			"Exclusive": false,
			"NoWait": true,
			"QosCountOverride": 5,
			"MessageBuffer": 100,
			"ErrorBuffer": 10,
			"SleepOnErrorInterval": 100,
			"SleepOnIdleInterval": 0
		}
	},
	"PublisherConfig":{
		"SleepOnIdleInterval": 0,
		"SleepOnErrorInterval": 1000,
		"LetterBuffer": 1000,
		"MaxOverBuffer": 1000,
		"NotificationBuffer": 1000
	}
}
```

</p>
</details>

## The Publisher

<details><summary>Click for creating publisher examples!</summary>
<p>

Assuming you have a **ChannelPool** already setup. Creating a publisher can be achieved like so:

```golang
publisher, err := publisher.NewPublisher(Seasoning, channelPool, nil)
```

Assuming you have a **ChannelPool** and **ConnectionPool** setup. Creating a publisher can be achieved like so:

```golang
publisher, err := publisher.NewPublisher(Seasoning, channelPool, connectionPool)
```

The errors here indicate I was unable to create a Publisher - probably due to the ChannelPool/ConnectionPool given.

</p>
</details>

---

<details><summary>Click for simple publish example!</summary>
<p>

Once you have a publisher, you can perform a relatively simple publish.

```golang
letter := utils.CreateLetter("", "TestQueueName", nil)
publisher.Publish(letter)
```

This **CreateLetter** method creates a simple HelloWorld message letter with no ExchangeName and a QueueName/RoutingKey of TestQueueName. The body is nil, the helper function creates bytes for "h e l l o   w o r l d".

The concept of a Letter may seem clunky on a single publish. I don't disagree and you still always have `streadway/amqp` to rely on. The **letter** idea makes sense with **AutoPublish**.

</p>
</details>

---

<details><summary>Click for AutoPublish example!</summary>
<p>

Once you have a publisher, you can perform **StartAutoPublish**!

```golang
allowInternalRetry := false
publisher.StartAutoPublish(allowInternalRetry)

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
```

This tells the Publisher to start reading an **internal queue**, a letter queue.

Once this has been started up, we allow letters to be placed in the mailbox/letter queue.

That could be simple like this...

```golang
publisher.QueueLetter(letter) // How simple is that!
```

...or more complex such as...

```golang
for _, letter := range letters {
    // will queue up to the letter buffer
    // will allow blocking calls upto max over buffer
    // after reaching full LetterBuffer+MaxOverBuffer, it spins a
    //    sleep loop based on the SleepOnErrorInterval for Publishers
    publisher.QueueLetter(letter)
}
```

So you can see why we use these message containers called **letter**. The letter has the **body** and **envelope** inside of it. It has everything you need to publish it. Think of it a small, highly configurable, **unit of work**.

Notice that you don't have anything to do with channels and connections!

</p>
</details>

---

<details><summary>Click for a more convoluted AutoPublish example!</summary>
<p>

Let's say the above example was too simple for you... ...let's up it a notch on what you can do with AutoPublish.

```golang

allowInternalRetry := true
publisher.StartAutoPublish(allowInternalRetry) // this will retry based on the Letter.RetryCount passed in.

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
```

We have finished our work, we **succeeded** or **failed** to publish **1000** messages. So now we want to shutdown everything!

```golang
publisher.StopAutoPublish()
// channelPool.Shutdown() // if you have a pointer to your channel pool nearby!
```

</p>
</details>

---

## The Consumer

<details><summary>Click for simple Consumer usage example!</summary>
<p>

Consumer provides a simple Get and GetBatch much like the Publisher has a simple Publish.

```golang
autoAck := true
message, err = consumer.Get("ConsumerTestQueue", autoAck)
```

Exit Conditions:

 * On Error: Error Return, Nil Message Return
 * On Not Ok: Nil Error Return, Nil Message Return
 * On OK: Nil Error Return, Message Returned

We also provide a simple Batch version of this call.


```golang
autoAck := false
messages, err = consumer.GetBatch("ConsumerTestQueue", 10, autoAck)
```

Exit Conditions:

 * On Error: Error Return, Nil Messages Return
 * On Not Ok: Nil Error Return, Available Messages Return (0 upto (nth - 1) message)
 * When BatchSize is Reached: Nil Error Return, All Messages Return (n messages)

Since `autoAck=false` is an option so you will want to have some post processing **ack/nack/rejects**.

Here is what that may look like:

```golang
requeueError := true
for _, message := range messages {
    /* Do some processing with message */

    if err != nil {
        message.Nack(requeueError)
    }

    message.Acknowledge()
}
```

</p>
</details>

---

<details><summary>Click for an actual Consumer consuming example!</summary>
<p>

Let's start with the ConsumerConfig, and again, the config is just a **quality of life** feature. You don't have to use it.

Here is a **JSON map/dictionary** wrapped in a **ConsumerConfigs**.

```javascript
"ConsumerConfigs": {
	"TurboCookedRabbitConsumer-Ackable": {
		"QueueName": "ConsumerTestQueue",
		"ConsumerName": "TurboCookedRabbitConsumer-Ackable",
		"AutoAck": false,
		"Exclusive": false,
		"NoWait": false,
		"QosCountOverride": 5,
		"MessageBuffer": 100,
		"ErrorBuffer": 10,
		"SleepOnErrorInterval": 100,
		"SleepOnIdleInterval": 0
	},
	"TurboCookedRabbitConsumer-AutoAck": {
		"QueueName": "ConsumerTestQueue",
		"ConsumerName": "TurboCookedRabbitConsumer-AutoAck",
		"AutoAck": true,
		"Exclusive": false,
		"NoWait": true,
		"QosCountOverride": 5,
		"MessageBuffer": 100,
		"ErrorBuffer": 10,
		"SleepOnErrorInterval": 100,
		"SleepOnIdleInterval": 0
	}
},
```

And finding this object after it was loaded from a JSON file.

```golang
consumerConfig, ok := config.ConsumerConfigs["TurboCookedRabbitConsumer-AutoAck"]
```

Creating the Consumer from Config after creating a ChannelPool.

```golang
consumer, err := consumer.NewConsumerFromConfig(consumerConfig, channelPool)
```

Then start Consumer?

```golang
consumer.StartConsuming()
```

Thats it! Wait where our my messages?! MY QUEUE IS DRAINING!

Oh, right! That's over here, keeping with the ***out of process design***...

```golang
ConsumeMessages:
    for {
        select {
        case message := <-consumer.Messages():

            requeueError := false
            var err error
            /* Do something with the message! */
            if message.IsAckable { // Message might be Ackable - be sure to check!
                if err != nil {
                    message.Nack(requeueError)
                }

                message.Acknowledge()
            }

        default:
            time.Sleep(100 * time.Millisecond) // No messages == optional nap time.
        }
    }
```

</p>
</details>

---

<details><summary>Wait! What the hell is coming out of <-Messages()</summary>
<p>

Great question. I toyed with the idea of returning Letters like Publisher uses (and I may still at some point) but for now you receive a `models.Message`.

***But... why***? Because the payload/data/message body is in there but, more importantly, it contains the means of quickly acking the message! It didn't feel right being merged with a `models.Letter`. I may revert and use the base `amqp.Delivery` which does all this and more... I just didn't want users to have to also pull in `streadway/amqp` to simplify their imports. If you were already using it wouldn't be an issue. This design is still being code reviewed in my head.

One of the complexities of RabbitMQ is that you need to Acknowledge off the same Channel that it was received on. That makes out of process designs like mine prone to two things: hackery and/or memory leaks (passing the channels around everywhere WITH messages).

There are two things I **hate** about RabbitMQ
 * Channels close on error.
 * Messages have to be acknowledge on the same channel.

What I have attempted to do is to make your life blissful by not forcing you to deal with it. The rules are still there, but hopefully, I give you the tools to not stress out about it and to simplify **out of process** acknowledgements.

That being said, there is only so much I can hide in my library, which is why I have exposed .Errors(), so that you can code and log accordingly.

```golang
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
```

Here you may trigger StopConsuming with this

```golang
consumer.StopConsuming(false)
```

But be mindful there are Channel Buffers internally that may be full and goroutines waiting to add even more.

I have provided some tools that can be used to help with this. You will see them sprinkled periodically through my tests.

```golang
consumer.FlushStop() // could have been called more than once.
consumer.FlushErrors() // errors can quickly build up if you stop listening to them
consumer.FlushMessages() // lets say the ackable messages you have can't be acked and you just need to flush them all out of memory
```

Becareful with FlushMessages(). If you are `autoAck = false` and receiving ackAble messages, this is safe. You will merely **wipe them from your memory** and ***they are still in the original queue***.

Here I demonstrate a very busy ***ConsumerLoop***. Just replace all the counter variables with logging and then an action performed with the message and this could be a production microservice loop.

```golang
ConsumeLoop:
	for {
		select {
		case <-timeOut:
			break ConsumeLoop
		case notice := <-publisher.Notifications():
			if notice.Success {
				fmt.Printf("%s: Published Success - LetterID: %d\r\n", time.Now(), notice.LetterID)
				messagesPublished++
			} else {
				fmt.Printf("%s: Published Failed Error - LetterID: %d\r\n", time.Now(), notice.LetterID)
				messagesFailedToPublish++
			}
		case err := <-ChannelPool.Errors():
			fmt.Printf("%s: ChannelPool Error - %s\r\n", time.Now(), err)
			channelPoolErrors++
		case err := <-ConnectionPool.Errors():
			fmt.Printf("%s: ConnectionPool Error - %s\r\n", time.Now(), err)
			connectionPoolErrors++
		case err := <-consumer.Errors():
			fmt.Printf("%s: Consumer Error - %s\r\n", time.Now(), err)
			consumerErrors++
		case message := <-consumer.Messages():
			messagesReceived++
			fmt.Printf("%s: ConsumedMessage\r\n", time.Now())
			go func(msg *models.Message) {
				err := msg.Acknowledge()
				if err != nil {
					fmt.Printf("%s: AckMessage Error - %s\r\n", time.Now(), err)
					messagesFailedToAck++
				} else {
					fmt.Printf("%s: AckMessaged\r\n", time.Now())
					messagesAcked++
				}
			}(message)
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
```


</p>
</details>

---

## The Pools

<details><summary>Rabbit Pools, how do they even work?!</summary>
<p>

ChannelPools are built on top of ConnectionPools and unfortunately, there is a bit of complexity here. Suffice to say I recommend (when creating both pools) to think 1:5 ratio. If you have one Connection, I recommend around 5 Channels to be built on top of it.

Ex.) ConnectionCount: 5 => ChannelPool: 25

I allow most of this to be configured now inside the ChannelPoolConfig and ConnectionPoolConfig. I had previously been hard coding some base variables but that's wrong.

```javascript
"PoolConfig": {
    "ChannelPoolConfig": {
        "ErrorBuffer": 10,
        "SleepOnErrorInterval": 1000,
        "MaxChannelCount": 50,
        "MaxAckChannelCount": 50,
        "AckNoWait": false,
        "GlobalQosCount": 5
    },
    "ConnectionPoolConfig": {
        "URI": "amqp://guest:guest@localhost:5672/",
        "ErrorBuffer": 10,
        "SleepOnErrorInterval": 5000,
        "MaxConnectionCount": 10,
        "TLSConfig": {
            "EnableTLS": false,
            "PEMCertLocation": "test/catest.pem",
            "LocalCertLocation": "client/cert.ca",
            "CertServerName": "hostname-in-cert"
        }
    }
},
```

Feel free to test out what works for yourself. Suffice to say though, there is a chance for a pause/delay/lag when there are no Channels available. High performance on your system may require fine tuning and benchmarking. The thing is though, you can't just add Connections and Channels evenly. First off Connections, server side are not infinite. You can't keep just adding those.

Every sequential Channel you get from the ChannelPool, was made with a different Connection. They are both backed by a Queue data structure, so this means you can't get the same Connection twice in sequence* (*with the exception of probability and concurrency/parallelism). There is a significant chance for greater throughput/performance by essentially load balancing Connections (which boils down to basically TCP sockets). All this means, layman's terms is that each ChannelPool is built off a Round Robin ConnectionPool (TCP Sockets). The ChannelPool itself adds another distribution of load balancing by ensuring every ChannelPool.GetChannel() is also non-sequential (Queue-structure). It's a double layer of Round Robin.

Why am I sharing any of this? Because the ChannelPool / ConnectionPool can be used 100% independently of everything else. You can implement your own fancy RabbitService using just my ConnectionPool and it won't hurt my feelings. Also - it looks complicated. There is a lot going on under the covers that can be confusing without explaining what I was trying to do. Hell you may even see my mistakes! (Submit PR!)

The following code demonstrates one super important part with ChannelPools: **flag erred Channels**. RabbitMQ server closes Channels on error, meaning this guy is dead. You normally won't know it's dead until the next time you use it - and that can mean messages lost. By flagging the channel as dead properly, on the next GetChannel() call - if we get the channel that was just flagged - we discard it and in place make a new fresh Channel for caller to receive.

```golang
chanHost, err := pub.ChannelPool.GetChannel()
if err != nil {
    pub.sendToNotifications(letter.LetterID, err)
    pub.ChannelPool.ReturnChannel(chanHost)
    continue // can't get a channel
}

pubErr := pub.simplePublish(chanHost.Channel, letter)
if pubErr != nil {
    pub.handleErrorAndFlagChannel(err, chanHost.ChannelID, letter.LetterID)
    pub.ChannelPool.ReturnChannel(chanHost)
    continue // flag channel and try again
}
```

Unfortunately, there are still times when GetChannel() will fail, which is why we still produce errors and I do return those to you.

</p>
</details>

---

<details><summary>Click here to see how to build a Connection and Channel Pool!</summary>
<p>

Um... this is the easy way to do is with the Configs.

```golang
connectionPool, err := pools.NewConnectionPool(Seasoning.PoolConfig, false)
channelPool, err := pools.NewChannelPool(Seasoning.PoolConfig, connectionPool, false)
```

Then you want to Initiate the Pools (this builds your Connections and Channels)

```golang
connectionPool, err := pools.NewConnectionPool(Seasoning.PoolConfig, false)
channelPool, err := pools.NewChannelPool(Seasoning.PoolConfig, connectionPool, false)
connectionPool.Initialize()
channelPool.Initialize()
```

I saw this as rather cumbersome... so I provided some short-cuts. The following instantiates a ConnectionPool internally to the ChannelPool. The only thing you lose here is the ability to share or use the ConnectionPool independently of the ChannelPool.

```golang
connectionPool, err := pools.NewConnectionPool(Seasoning.PoolConfig, false)
channelPool, err := pools.NewChannelPool(Seasoning.PoolConfig, connectionPool, false)
channelPool.Initialize() // auto-initializes the ConnectionPool...
```
But I am still pretty lazy.

```golang
channelPool, err := pools.NewChannelPool(Seasoning.PoolConfig, nil, false)
channelPool.Initialize()
```

</p>
</details>

---

<details><summary>Click here to see how to get and use a Channel!</summary>
<p>

So now you will more than likely want to use your ChannelPool.

```golang
channelHost, err := channelPool.GetChannel()

channelPool.ReturnChannel(chanHost)
```

This ChannelHost is like a wrapper around the AmqpChannel that adds a few features like Errors and ReturnMessages. You also don't have to use my Publisher, Consumer, and Topologer. You can use the ChannelPools yourself if you just like the idea of backing your already existing code behind a ChannelPool/ConnectionPool.

The Publisher/Consumer/Topologer all use code similar to this!

```golang
channelHost, err := channelPool.GetChannel()
channelHost.Channel.Publish(
		exchangeName,
		routingKey,
		mandatory,
		immediate,
		amqp.Publishing{
			ContentType: contentType,
			Body:        body,
		},
    )
channelPool.ReturnChannel(chanHost)
```

I am working on streamlining the ChannelHost integration with ChannelPool. I want to allow communication between the two by flowing Channel errors up to pool/group. It's a bit clunky currently but I am still thinking how best to do such a thing. Ideally all Channel errors (CloseErrors) would be subscribed to and perhaps AutoFlag the channels as dead and I can consolidate my code if that's determine reliable.

</p>
</details>

---

<details><summary>What happens during an outage?</summary>
<p>

Well, if you are using a ChannelPool w/ ConnectionPool, it will handle an outage, full or transient, just fine. The Connections/ConnectionHosts will be either heartbeat recovered or be replaced. The Channels will all have to be replaced during the next **GetChannel()** invocation.

There is one small catch though when using ChannelPools.  

Since dead Channels are replaced during a call of **GetChannel()** and you may have replaced all your ConnectionHosts, you may not fully rebuild all your channels. The reason for that is demand/load. I have done my best to force ChannelHost creation and distribution across the individual ConnectionHosts... but unless you are rapidly getting all ChannelHosts, you may never hit your original MaxChannelCount from your PoolConfig based on your use case scenarios. If you can't generate ChannelHost demand through **GetChannel()** calls, then it won't always rebuild. On the other hand, if your **GetChannel()** call count does increase, so to will your ChannelHost counts.

I intend to tweak things here. I have tested multiple back-to-back outages during tests/benches and it has allowed me to improve the user experience / system experience significantly - but refactoring could have brought about bugs. Like I said, I will keep reviewing my work and checking if there are any tweaks.

</p>
</details>

---

<details><summary>Click to see how to properly prepare for an outage!</summary>
<p>

Observe the following code example:

```golang
channelPool, err := pools.NewChannelPool(Seasoning.PoolConfig, nil, true)

iterations := 0
maxIterationCount := 100000

// Shutdown RabbitMQ server after entering loop, then start it again, to test reconnectivity.
for iterations < maxIterationCount {

	chanHost, err := channelPool.GetChannel()
	if err != nil {
		fmt.Printf("%s: Error - GetChannelHost: %s\r\n", time.Now(), err)
	} else {
		fmt.Printf("%s: GotChannelHost\r\n", time.Now())

		select {
		case <-chanHost.CloseErrors():
			fmt.Printf("%s: Error - ChannelClose: %s\r\n", time.Now(), err)
		default:
			break
		}

		letter := utils.CreateLetter("", "ConsumerTestQueue", nil)
		err := chanHost.Channel.Publish(
			letter.Envelope.Exchange,
			letter.Envelope.RoutingKey,
			letter.Envelope.Mandatory, // publish doesn't appear to work when true
			letter.Envelope.Immediate, // publish doesn't appear to work when true
			amqp.Publishing{
				ContentType: letter.Envelope.ContentType,
				Body:        letter.Body,
			},
		)

		if err != nil {
			fmt.Printf("%s: Error - ChannelPublish: %s\r\n", time.Now(), err)
			channelPool.FlagChannel(chanHost.ChannelID)
			fmt.Printf("%s: ChannelFlaggedForRemoval\r\n", time.Now())
		} else {
			fmt.Printf("%s: ChannelPublishSuccess\r\n", time.Now())
		}
	}
	channelPool.ReturnChannel(chanHost)
	iterations++
	time.Sleep(10 * time.Millisecond)
}

channelPool.Shutdown()
```

This is a very tight publish loop. This will blast thousands of messages per hour.

Simulating a server shutdown: `bin\rabbitmq-service.bat stop`

The entire thing loop will pause at **GetChannel()**. It will hault in **GetChannel()** as I preemptively determine the Channel's parent Connection is already closed. We then go into an infinite (but throttled) loop here. The loop consists of regenerating the Channel/ChannelHost (or even the Connection underneath).

What dictates the iteration time of these loops until success is the following:

```javascript
"ChannelPoolConfig": {
	"ErrorBuffer": 10,
	"SleepOnErrorInterval": 1000,
	"MaxChannelCount": 50,
	"MaxAckChannelCount": 50,
	"AckNoWait": false,
	"GlobalQosCount": 5
},
```
The related settings for outages are here:

 * ErrorBuffer is the buffer for the Error channel. Important to subscribe to the ChannelPool Error channel some where so it doesn't become blocking/full.
 * SleepOnErrorInterval is the built in sleep when an error or closed Channel is found.
   * This is the minimum interval waited when rebuilding the ChannelHosts.

```javascript
"ConnectionPoolConfig": {
	"URI": "amqp://guest:guest@localhost:5672/",
	"ErrorBuffer": 10,
	"SleepOnErrorInterval": 5000,
	"MaxConnectionCount": 10,
	"TLSConfig": {
		"EnableTLS": false,
		"PEMCertLocation": "test/catest.pem",
		"LocalCertLocation": "client/cert.ca",
		"CertServerName": "hostname-in-cert"
	}
}
```

The related settings for outages are here:

 * ErrorBuffer is the buffer for the Error channel. Important to subscribe to the ChannelPool Error channel some where so it doesn't become blocking.
 * SleepOnErrorInterval is the built in sleep when an error or closed Connection is found.
   * This is the minimum interval waited when rebuilding the ConnectionHosts.
   * I recommend this value to be higher than the ChannelHost interval.

</p>
</details>

---

<details><summary>Click here for more details on the Circuit Breaking!</summary>
<p>

We will use the above settings in the ChannelPool (**SleepOnErrorInterval = 1000**) and ConnectionPool (**SleepOnErrorInterval = 5000**) here is what will happen to the above code when publishing.

 1. RabbitMQ Server outage occurs.
 2. Everything pauses in place, creating infinite loops on **GetChannel()** (which calls **GetConnection()**).  
    * This can be a bit dangerous itself if you have thousands of goroutines calling **GetChannel()** so plan accordingly.
	* Some errors can occur in Consumers/Publishers/ChannelPools/ConnectionPools for in transit at the time of outage.
 3. RabbitMQ Server connectivity is restored.
 4. The loop iterations start finding connectivity, they build a connection.
	* The minimum wait time is 5 seconds for the ConnectionHost.
	* You will also start seeing very slow publishing.
 5. This same loop is building a ChannelHost.
    * The minimum wait time after ChannelHost was built is 1 second.
 6. The total time waited should be about 6 seconds.
 7. The next **GetChannel()** is called.
	* Because we use Round Robin connections, the next Connection in the pool is called.
	* We wait a minimum of time of 5 seconds for recreating the ConnectionHost, then 1 second again for the ChannelHost.
 8. This behavior continues until all Connections have been successfully restored.
 9. After ConnectionHosts, restoring the remaining ChannelHosts.
    * The minimum wait time is now 1 second, no longer the combined total of 6 seconds.
	* Slightly faster publshing can be observed.
 10. Once all Channels have been restored, the time wait between publishes is found in the publishing loop: **10 ms**.
    * The connectivity has been fully regenerated at this point.
	* Full speed publishing can now be observed.

So you make recognize this as a funky CircuitBreaker pattern.

Circuit Breaker Behaviors 

 * We don't spin up memory, we don't spin up CPU.
 * We don't spam connection requests to our RabbitMQ server.
 * Once connectivity is restored, we don't flood the RabbitMQ server.
   * This is slow-open.
   * The duration of this time becomes (time(connectionSleep + channelSleep)) * n) where ***n*** is the number of unopened Connections.
 * As connectivity is restored in Connections, we still see throttling behavior.
   * This is medium-open.
   * The duration of this time becomes (time(channelSleep) * n) where ***n*** is the number of still unopened Channel.
 * Once connectivity is fully open, publish rate should return to normal (pre-outage speed).
 * At any time, you can revert back to medium-open, slow-open, fully paused.
   * The loops never stop so you never have to worry about connectivity or reconnectivity.

All of this behavior depends on the healthy config settings that you determine upfront though - so this is all up to you!

Just remember Channels get closed or get killed all the time, you don't want this wait time too high. Connections rarely fully die, so you want this delay reasonably longer.

</p>
</details>

---

<details><summary>Click here for some Channel Pool benchmarks!</summary>
<p>

This is a raw AMQP publish test.  We create an AMQP connection, create an AMQP channel, and execute an AMQP publish looped.
MessageCount: 100,000
MessageSize: 2500 (2.5KB)

	PS C:\GitHub\personal\turbocookedrabbit> go.exe test -run "^(TestCreateSingleChannelAndPublish)$" -v
	=== RUN   TestCreateSingleChannelAndPublish
	--- PASS: TestCreateSingleChannelAndPublish (4.78s)
		main_benchpool_test.go:17: 2019-09-15 07:39:48.3578722 -0400 EDT m=+0.091806701: Benchmark Starts
		main_benchpool_test.go:49: 2019-09-15 07:39:53.1357496 -0400 EDT m=+4.869684101: Benchmark End
		main_benchpool_test.go:50: 2019-09-15 07:39:53.1357496 -0400 EDT m=+4.869684101: Time Elapsed 4.7778774s
		main_benchpool_test.go:51: 2019-09-15 07:39:53.1357496 -0400 EDT m=+4.869684101: Publish Errors 0
		main_benchpool_test.go:52: 2019-09-15 07:39:53.1357496 -0400 EDT m=+4.869684101: Msgs/s 20929.796148
		main_benchpool_test.go:53: 2019-09-15 07:39:53.1357496 -0400 EDT m=+4.869684101: KB/s 0.523245
	PASS
	ok      github.com/houseofcat/turbocookedrabbit 6.512s

Apples to Apples comparison using a ChannelPool. As you can see - the numbers went up - but should have been relatively the same. Shere is some variability with these tests. The important thing to note is that there isn't a significant reduction in performance.

	PS C:\GitHub\personal\turbocookedrabbit> go.exe test -run "^(TestGetSingleChannelFromPoolAndPublish)$" -v
	=== RUN   TestGetSingleChannelFromPoolAndPublish
	--- PASS: TestGetSingleChannelFromPoolAndPublish (4.32s)
		main_benchpool_test.go:59: 2019-09-15 07:47:19.0276488 -0400 EDT m=+0.090818001: Benchmark Starts
		main_benchpool_test.go:89: 2019-09-15 07:47:23.3516406 -0400 EDT m=+4.414809801: Benchmark End
		main_benchpool_test.go:90: 2019-09-15 07:47:23.3516406 -0400 EDT m=+4.414809801: Time Elapsed 4.3239918s
		main_benchpool_test.go:91: 2019-09-15 07:47:23.3516406 -0400 EDT m=+4.414809801: Publish Errors 0
		main_benchpool_test.go:92: 2019-09-15 07:47:23.3516406 -0400 EDT m=+4.414809801: Msgs/s 23126.778363
		main_benchpool_test.go:93: 2019-09-15 07:47:23.3516406 -0400 EDT m=+4.414809801: KB/s 0.578169
	PASS
	ok      github.com/houseofcat/turbocookedrabbit 4.506s

Apples to Apple Orange comparison same premise, but different ChannelHost per Publish.

	PS C:\GitHub\personal\turbocookedrabbit> go.exe test -run "^(TestGetMultiChannelFromPoolAndPublish)$" -v
	=== RUN   TestGetMultiChannelFromPoolAndPublish
	--- PASS: TestGetMultiChannelFromPoolAndPublish (3.72s)
		main_benchpool_test.go:99: 2019-09-15 07:40:44.1532264 -0400 EDT m=+0.086463601: Benchmark Starts
		main_benchpool_test.go:135: 2019-09-15 07:40:47.8682928 -0400 EDT m=+3.801530001: Benchmark End
		main_benchpool_test.go:136: 2019-09-15 07:40:47.8682928 -0400 EDT m=+3.801530001: Time Elapsed 3.7150664s
		main_benchpool_test.go:137: 2019-09-15 07:40:47.8682928 -0400 EDT m=+3.801530001: ChannelPool Errors 0
		main_benchpool_test.go:138: 2019-09-15 07:40:47.8682928 -0400 EDT m=+3.801530001: Publish Errors 0
		main_benchpool_test.go:139: 2019-09-15 07:40:47.8682928 -0400 EDT m=+3.801530001: Msgs/s 26917.419296
		main_benchpool_test.go:140: 2019-09-15 07:40:47.8682928 -0400 EDT m=+3.801530001: KB/s 0.672935
	PASS
	ok      github.com/houseofcat/turbocookedrabbit 3.893s

</p>
</details>

---

## The Topologer

<details><summary>How do I create/delete/bind queues and exchanges?</summary>
<p>

Coming from plain `streadway/amqp` there isn't too much to it. Call the right method with the right parameters.

I have however integrated those relatively painless methods now with a ChannelPool and added a `TopologyConfig` for a JSON style of batch topology creation/binding. The real advantages here is that I allow things in bulk and allow you to build topology from a **topology.json** file.

Creating an Exchange with a `models.Exchange`

```golang
err := top.CreateExchangeFromConfig(exchange) // models.Exchange
if err != nil {
    return err
}
```

Or if you prefer it more manual:

```golang
exchangeName := "FancyName"
exchangeType := "fanout"
passiveDeclare, durable, autoDelete, internal, noWait := false, false, false, false, false

err := top.CreateExchange(exchangeName, exchangeType, passiveDeclare, durable, autoDelete, internal, noWait, nil)
if err != nil {
    return err
}
```

Creating an Queue with a `models.Queue`

```golang
err := top.CreateQueueFromConfigeateQueue(queue) // models.Queue
if err != nil {
    return err
}
```

Or, again, if you prefer it more manual:

```golang
queueName := "FancyQueueName"
passiveDeclare, durable, autoDelete, exclusive, noWait := false, false, false, false, false

err := top.CreateQueue(queueName, passiveDeclare, durable, autoDelete, exclusive, noWait, nil)
if err != nil {
    return err
}
```

</p>
</details>

---

<details><summary>How do I do this in bulk?</summary>
<p>

Here I demonstrate the Topology as JSON (full sample is checked in as `testtopology.json`)

```javascript
{
	"Exchanges": [
		{
			"Name": "MyTestExchangeRoot",
			"Type": "direct",
			"PassiveDeclare": true,
			"Durable": true,
			"AutoDelete": false,
			"InternalOnly": false,
			"NoWait": true
		}
	],
	"Queues": [
		{
			"Name": "QueueAttachedToRoot",
			"PassiveDeclare": true,
			"Durable": true,
			"AutoDelete": false,
			"Exclusive": false,
			"NoWait": true
		}
	],
	"QueueBindings": [
		{
			"QueueName": "QueueAttachedToRoot",
			"ExchangeName": "MyTestExchangeRoot",
			"RoutingKey": "RoutingKeyRoot",
			"NoWait": true
		}
	],
	"ExchangeBindings":[
		{
			"ExchangeName": "MyTestExchange.Child01",
			"ParentExchangeName": "MyTestExchangeRoot",
			"RoutingKey": "ExchangeKey1",
			"NoWait": true
		}
	]
}
```

I have provided a helper method for turning it into a TopologyConfig.

```golang
topologyConfig, err := utils.ConvertJSONFileToTopologyConfig("testtopology.json")
```

Creating a simple and shareable ChannelPool.

```golang
channelPool, err := pools.NewChannelPool(Seasoning.PoolConfig, nil, false)
```

Using the ChannelPool to create our Topologer.

```golang
topologer := topology.NewTopologer(channelPool)
```

Assuming you have a blank slate RabbitMQ server, this shouldn't error out as long as you can connect to it.

```golang
ignoreErrors := false
err = topologer.BuildToplogy(topologyConfig, ignoreErrors)
```

Fin.

That's it really. In the future I will have more features. Just know that I think you can export your current Server configuration from the Server itself.

</p>
</details>

---
