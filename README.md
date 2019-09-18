# TurboCookedRabbit
 A user friendly RabbitMQ written in Golang.  
 Based on my work found at [CookedRabbit](https://github.com/houseofcat/CookedRabbit).

[![Go Report Card](https://goreportcard.com/badge/github.com/houseofcat/turbocookedrabbit)](https://goreportcard.com/report/github.com/houseofcat/turbocookedrabbit)

### Developer's Notes
It was programmed against the following:

 * Golang 1.13.0
 * RabbitMQ Server v3.7.17 (simple localhost)
 * Erlang v22.0 (OTP v10.4)
 * Streadway/Amqp Latest

Issues with more advanced setups? I will need an intimate description of the setup. Without it, I more than likely won't be able to resolve it. I can accept PRs if you want to test out fixes that resolve things for yourself.

If you see something syntactically wrong, do speak up! I am, relatively speaking, an idiot ^.^. I am still new to golang. My background is in performant infrastructure development, using C# and the .NET/NetCore ecosystem... so if any `golang wizards` want to provide advice or criticisms, please do!

I also don't have the kind of free time I used to. I apologize in advance but, hey, that's life. So keep in mind that I am not paid to do this - this isn't my job, this isn't a corporate sponsorship - suffice to say the golden rule works wonders on me / mind your Ps and Qs.

### Basic Performance

JSON Config Used: `testseason.json`
Benchmark Ran: `BenchmarkPublishConsumeAckForDuration` in `main_bench_test.go`

    1x Publisher AutoPublish ~ 21000 msg/s - Single Queue - Small Message Sizes
    1x Consumer Consumption ~ 2000-3000 msg/s - Single Queue - Small Message Sizes

Stress Test - 2 hours of Publish/Consume

	PS C:\GitHub\personal\turbocookedrabbit> go test -timeout 121m -run "^(TestStressPublishConsumeAckForDuration)$" -v

	=== RUN   TestStressPublishConsumeAckForDuration
	2019-09-15 12:07:08.9556184 -0400 EDT m=+0.100359901: Benchmark Starts
	2019-09-15 14:07:08.9566168 -0400 EDT m=+7200.101358301: Est. Benchmark End
	Messages Acked: 3662418
	Messages Failed to Ack: 0
	Messages Received: 3662418
	ChannelPool Errors: 0
	ConnectionPool Errors: 0
	Consumer Errors: 0
	Messages Published: 3662418
	Messages Failed to Publish: 0
	2019-09-15 14:07:08.9711087 -0400 EDT m=+7200.115850201: Benchmark Finished
	--- PASS: TestStressPublishConsumeAckForDuration (7200.02s)
	PASS
	ok      github.com/houseofcat/turbocookedrabbit 7201.863s

### Work Recently Finished
 * Solidify Connections/Pools outage handling.
 * Solidify Consumers outage handling.
   * Publishers were still working.
     * Blocking introduced on QueueLetter.
 * Publisher AutoPublish performance up to par!
 * Properly handle total outages server side.
 * Refactor the reconnection logic.
   * Now everything stops/pauses until connectivity is restored.
 * RabbitMQ Topology Creation/Destruction support.
 * Started Profiling and including Benchmark/Profile .svgs.

### Current Known Issues
 * ~CPU/MEM spin up on a total server outage even after adjusting config.~
   * ~Channels in ChannelPool aren't redistributing evenly over Connections.~
 * ~Consumer stops working after server outage restore.~
   * ~Publisher is still working though.~
 * ~AutoPublisher is a tad on the slow side. Might be the underlying Channel/QueueLetter.~
   * ~Raw looped Publish shows higher performance.~
 * README needs small comments/updates related to new work finished on 9/13/2019 - 7:10 PM EST.

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
			"Heartbeat": 5,
			"ConnectionTimeout": 10,
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
		"SleepOnQueueFullInterval": 100,
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
letter := utils.CreateLetter(1, "", "TestQueueName", nil)
publisher.Publish(letter)
```

This **CreateLetter** method creates a simple HelloWorld message letter with no ExchangeName and a QueueName/RoutingKey of TestQueueName. The body is nil, the helper function creates bytes for "h e l l o   w o r l d".

The concept of a Letter may seem clunky on a single publish. I don't disagree and you still have `streadway/amqp` to rely on. The **letter** idea makes more sense with **AutoPublish**.

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
            /* Handle Requeue or a manual Re-Publish */
        }
    default:
        time.Sleep(1 * time.Millisecond)
    }
}
```

This tells the Publisher to start reading an **internal queue**, and process Publishing concurrently.

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

So you can see why we use these message containers called **letter**. The letter has the **body** and **envelope** inside of it. It has everything you need to publish it. Think of it a small, highly configurable, **unit of work** and **address**.

Notice that you don't have anything to do with channels and connections (even on outage)!

</p>
</details>

---

<details><summary>Click for a more convoluted AutoPublish example!</summary>
<p>

Let's say the above example was too simple for you... ...let's up the over engineering a notch on what you can do with AutoPublish.

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
// channelPool.Shutdown() // don't forget to cleanup (if you have a pointer to your channel pool nearby)!
```

</p>
</details>

---

<details><summary>Click for a clean AutoPublish benchmark!</summary>
<p>

Early on the performance was not really there on Publish - some 500 msgs/s. Which is great, but not the numbers found during development. Somewhere along the way I introduced one too many race conditions. Also aggressively throttled configurations don't help either. Any who, I isolated the components and benched just AutoPublish and with a few tweaks - I started seeing raw concurrent/parallel Publishing performance for a single a Publisher!

Ran this benchmark with the following Publisher settings and distributed over 10 queues (i % 10).

```javascript
"PublisherConfig":{
	"SleepOnIdleInterval": 0,
	"SleepOnQueueFullInterval": 1,
	"SleepOnErrorInterval": 1000,
	"LetterBuffer": 10000,
	"MaxOverBuffer": 2000,
	"NotificationBuffer": 1000
}
```

	PS C:\GitHub\personal\turbocookedrabbit> go.exe test -benchmem -run=^$ github.com/houseofcat/turbocookedrabbit/publisher -bench "^(BenchmarkAutoPublishRandomLetters)$" -v
	goos: windows
	goarch: amd64
	pkg: github.com/houseofcat/turbocookedrabbit/publisher
	BenchmarkAutoPublishRandomLetters-8            1        7346832700 ns/op        563734704 B/op   4525448 allocs/op
	--- BENCH: BenchmarkAutoPublishRandomLetters-8
		publisher_bench_test.go:21: 2019-09-15 18:58:57.6932202 -0400 EDT m=+0.107877301: Purging Queues...
		publisher_bench_test.go:37: 2019-09-15 18:58:57.6972462 -0400 EDT m=+0.111903301: Building Letters
		publisher_bench_test.go:42: 2019-09-15 18:58:58.9048792 -0400 EDT m=+1.319536301: Finished Building Letters
		publisher_bench_test.go:43: 2019-09-15 18:58:58.9048792 -0400 EDT m=+1.319536301: Total Size Created: 199.844457 MB
		publisher_bench_test.go:62: 2019-09-15 18:58:58.9058787 -0400 EDT m=+1.320535801: Queueing Letters
		publisher_bench_test.go:67: 2019-09-15 18:59:02.669778 -0400 EDT m=+5.084435101: Finished Queueing letters after 3.7638993s
		publisher_bench_test.go:68: 2019-09-15 18:59:02.669778 -0400 EDT m=+5.084435101: 26568.192194 Msg/s
		publisher_bench_test.go:74: 2019-09-15 18:59:04.6839535 -0400 EDT m=+7.098610601: Purging Queues...
	PASS
	ok      github.com/houseofcat/turbocookedrabbit/publisher       10.092s

Noice!

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
		"Heartbeat": 5,
		"ConnectionTimeout": 10,
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

		letter := utils.CreateLetter(1, "", "ConsumerTestQueue", nil)
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
	"Heartbeat": 5,
	"ConnectionTimeout": 10,
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

	PS C:\GitHub\personal\turbocookedrabbit> go.exe test -timeout 30s github.com/houseofcat/turbocookedrabbit/pools -run "^(TestCreateSingleChannelAndPublish)$" -v
	=== RUN   TestCreateSingleChannelAndPublish
	--- PASS: TestCreateSingleChannelAndPublish (4.57s)
		pools_test.go:51: 2019-09-15 14:48:11.615081 -0400 EDT m=+0.085770701: Benchmark Starts
		pools_test.go:95: 2019-09-15 14:48:16.1879969 -0400 EDT m=+4.658686601: Benchmark End
		pools_test.go:96: 2019-09-15 14:48:16.1879969 -0400 EDT m=+4.658686601: Time Elapsed 4.5729159s
		pools_test.go:97: 2019-09-15 14:48:16.1879969 -0400 EDT m=+4.658686601: Publish Errors 0
		pools_test.go:98: 2019-09-15 14:48:16.1879969 -0400 EDT m=+4.658686601: Publish Actual 100000
		pools_test.go:99: 2019-09-15 14:48:16.1879969 -0400 EDT m=+4.658686601: Msgs/s 21867.885215
		pools_test.go:100: 2019-09-15 14:48:16.1879969 -0400 EDT m=+4.658686601: MB/s 54.669713
	PASS
	ok      github.com/houseofcat/turbocookedrabbit/pools   6.188s

Apples to Apples comparison using a ChannelPool. As you can see - the numbers went up - but should have been relatively the same. There is some variability with these tests. The important thing to note is that there isn't a significant reduction in performance. You shouldn't see more or less performance - that is the target!

	PS C:\GitHub\personal\turbocookedrabbit> go.exe test -timeout 30s github.com/houseofcat/turbocookedrabbit/pools -run "^(TestGetSingleChannelFromPoolAndPublish)" -v
	=== RUN   TestGetSingleChannelFromPoolAndPublish
	--- PASS: TestGetSingleChannelFromPoolAndPublish (4.30s)
		pools_test.go:106: 2019-09-15 14:50:01.2111296 -0400 EDT m=+0.104896201: Benchmark Starts
		pools_test.go:146: 2019-09-15 14:50:05.5139474 -0400 EDT m=+4.407714001: Benchmark End
		pools_test.go:147: 2019-09-15 14:50:05.5140242 -0400 EDT m=+4.407790801: Time Elapsed 4.3028178s
		pools_test.go:148: 2019-09-15 14:50:05.5140242 -0400 EDT m=+4.407790801: Publish Errors 0
		pools_test.go:149: 2019-09-15 14:50:05.5140242 -0400 EDT m=+4.407790801: Publish Actual 100000
		pools_test.go:150: 2019-09-15 14:50:05.5140623 -0400 EDT m=+4.407828901: Msgs/s 23240.584345
		pools_test.go:151: 2019-09-15 14:50:05.5140623 -0400 EDT m=+4.407828901: MB/s 58.101461
	PASS
	ok      github.com/houseofcat/turbocookedrabbit/pools   4.507s

Apples to Apple-Orange-Hybrid comparison. Exact same premise, but different ChannelHost per Publish allowing us to publish concurrently. I was just showing off at this point.

	PS C:\GitHub\personal\turbocookedrabbit> go test -timeout 10s github.com/houseofcat/turbocookedrabbit/pools -run "^(TestGetMultiChannelFromPoolAndPublish)" -v
	=== RUN   TestGetMultiChannelFromPoolAndPublish
	--- PASS: TestGetMultiChannelFromPoolAndPublish (4.95s)
		pools_test.go:157: 2019-09-15 14:53:41.2687154 -0400 EDT m=+0.091933501: Benchmark Starts
		pools_test.go:204: 2019-09-15 14:53:46.2171263 -0400 EDT m=+5.040344401: Benchmark End
		pools_test.go:205: 2019-09-15 14:53:46.2171263 -0400 EDT m=+5.040344401: Time Elapsed 2.9471258s
		pools_test.go:206: 2019-09-15 14:53:46.2171263 -0400 EDT m=+5.040344401: ChannelPool Errors 0
		pools_test.go:207: 2019-09-15 14:53:46.2171263 -0400 EDT m=+5.040344401: Publish Errors 0
		pools_test.go:208: 2019-09-15 14:53:46.2171263 -0400 EDT m=+5.040344401: Publish Actual 100000
		pools_test.go:209: 2019-09-15 14:53:46.2171263 -0400 EDT m=+5.040344401: Msgs/s 33931.364586
		pools_test.go:210: 2019-09-15 14:53:46.2171263 -0400 EDT m=+5.040344401: MB/s 84.828411
	PASS
	ok      github.com/houseofcat/turbocookedrabbit/pools   5.143s

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
