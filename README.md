# TurboCookedRabbit
 A user friendly RabbitMQ library written in Golang to help use <a href="https://github.com/streadway/amqp">streadway/amqp</a>.  
 Based on my work found at [CookedRabbit](https://github.com/houseofcat/RabbitMQ.Core).

[![Go Report Card](https://goreportcard.com/badge/github.com/houseofcat/turbocookedrabbit)](https://goreportcard.com/report/github.com/houseofcat/turbocookedrabbit)
<a title="" target="_blank" href="https://golangci.com/r/github.com/houseofcat/turbocookedrabbit"><img src="https://golangci.com/badges/github.com/houseofcat/turbocookedrabbit.svg"></a>  

<a title="Release" target="_blank" href="https://github.com/houseofcat/turbocookedrabbit/releases"><img src="https://img.shields.io/github/release/houseofcat/turbocookedrabbit.svg?style=flat-square"></a>  

### Work Recently Finished (v2.2.0)
 * Dependency updates and comment cleanup.
   * Now on Go 1.18.
 * All changes recommended by `golangci-lint` (mostly cosmetic).
 * Fixing the func named BuildToplogy -> BuildTopology.

### Notice
The v1 API is no longer supported. The tags will be kept to prevent breaking any live applications and the source for archival/forking purposes. No future work will be performed going forward, please migrate to `tcr` packages and `v2+`. Thanks!

### Goals
The major goals (now that everything is working well) are to aid quality of life and visibility. Be sure to visit tests for examples on how to do a variety of actions with the library. They are always kept up to date, even if the `README.md` falls short at times.

### Developer's v3 Notes

 * Golang 1.18.4 (2022/07/29)
 * RabbitMQ Server v3.10.6 (simple localhost)
 * Erlang v25.3
 * github.com/rabbitmq/amqp091-go

<details><summary>Click here to see how to migrate (or get V2) and breaking details!</summary>
<p>

## TO GET (including V2)
`go get -u "github.com/houseofcat/turbocookedrabbit/v2"`  

If I hotfix legacy
`go get -u "github.com/houseofcat/turbocookedrabbit/v1"` 

To continue using legacy up to v1.2.3
`go get -u "github.com/houseofcat/turbocookedrabbit"`  

## TO UPGRADE TO V2
Everything is unified to TCR.  
Convert your imports to a single `"github.com/houseofcat/turbocookedrabbit/v2/pkg/tcr"`.  
And where you have `pools.` or `models.` or `publisher.` or `utils.` replace it with just this `tcr.`

Why am I being so complicated? See below...

### Started Semantic Versioning

 * Separate go.mods.
 * Separate pkgs.

All legacy work will be done with a new module `/v1/pkg/***`.    
All v2 work will be done with a new module `/v2/pkg/tcr`.   

Apologies for all the confusion but if I don't do this, apparently I couldn't use the v2.0.0 tagging system and allow people to `@upgrade`. This is a stupid design choice by the `Go Modules` and was irrevocably harder to get VSCode to understand what the hell I was trying to do and therefore, not shit it's pants.  

Learn more about the fabulous world of PhD's at Google designing things!  
https://blog.golang.org/v2-go-modules  

### Breaking Change Notice (v1.x.x -> v2.0.0)
Decided to opt-in for a near total rewrite. Simpler. Cleaner. Easier to use. The AutoPublish is now using PublishWithConfirmation and so what I now try to emulate is "At Least Once" style of delivery.  

Reason For Rewrite?  
I am better at Golang, in general, and I now have a year of experience using my own software in a production environment. This rewrite has an even stronger auto-recovery mechanism and even harder Publish (PublishWithConfirmation) system. In some ways, performance has significantly increased. In other ways, it sacrified performance for resilience.  

</p>
</details>

### Basic Performance

<details><summary>Click for seeing Publisher performance!</summary>
<p>

Test Settings
```
i7 8700K @ 4.7GHz  
Samsung EVO 970  
RabbitMQ Server 3.8.5 / Erlang 23.0 installed on the same host.    
Single Queue Publish (Without Confirmations)  
Messages Transient Type, 1500-2500 bytes, wrapped themselves, uncompressed.  
100,000 Count Publish Test  
NON-DEBUG
```

```
Benchmark Starts: 2020-07-01 13:01:37.6260111 -0400 EDT m=+0.044880301
Benchmark End: 2020-07-01 13:01:40.6130211 -0400 EDT m=+3.031890301
Messages: 33478.294348 msg/s
```

</p>
</details>

<details><summary>Click for seeing AutoPublisher performance!</summary>
<p>

Test Settings
```
i7 8700K @ 4.7GHz  
Samsung EVO 970  
RabbitMQ Server 3.8.5 / Erlang 23.0 installed on the same host.   
Single Queue Publish (With Confirmations) / Single Consumer  
Messages Durable Type, 1500-2500 bytes, wrapped themselves, uncompressed.  
Two Hour Stress Test, Consumer/Publisher running in tandem.  
DEBUG  
```

```
Failed to Queue For Publishing: 0
Publisher Errors: 0
Messages Published: 25882186
Messages Failed to Publish: 132
Consumer Errors: 0
Messages Acked: 25883360
Messages Failed to Ack: 0
Messages Received: 25883360
Messages Unexpected: 0
Messages Duplicated: 0
PASS
```

AutoPublisher (PublishWithConfirmation) averaged around `3,594.75 msg/s`.  
Consumer averaged around `3,594.91 msg/s`. Probably limited to the AutoPublisher.  

</p>
</details>

## The Seasoning/Config

<p><details><summary>Click for details on creating a config from file!</summary>

The config.json is just a **quality of life** feature. You don't have to use it. I just like how easy it is to change configurations on the fly.

```golang
config, err := tcr.ConvertJSONFileToConfig("testseasoning.json")
```

The full structure `RabbitSeasoning` is available under `pkg/tcr/configs.go`

</p>
</details>

---

## The Publisher

<details><summary>Click for creating publisher examples!</summary>
<p>

Assuming you have a **ConnectionPool** already setup. Creating a publisher can be achieved like so:

```golang
publisher := tcr.NewPublisherFromConfig(Seasoning, ConnectionPool)
```

</p>
</details>

---

<details><summary>Click for simple publish example!</summary>
<p>

Once you have a publisher, you can perform a relatively simple publish.

```golang
letter := tcr.CreateMockLetter(1, "", "TcrTestQueue", nil)
publisher.Publish(letter)
```

This **CreateMockLetter** method creates a simple HelloWorld message letter with no ExchangeName and a QueueName/RoutingKey of `"TcrTestQueue"`. The helper function creates bytes for "h e l l o   w o r l d" as a message body.

The concept of a Letter may seem clunky but the real advantage is async publishing and replay-ability. And you still have `streadway/amqp` to rely on should prefer simple publshing straight to an `amqp.Channel`.

</p>
</details>

---

<details><summary>Click for simple publish with confirmation example!</summary>
<p>

We use PublishWithConfirmation when we want a more resilient publish mechanism. It will wait for a publish confirmation until timeout.

```golang
letter := tcr.CreateMockRandomLetter("TcrTestQueue")
publisher.PublishWithConfirmation(letter, time.Millisecond*500)

WaitLoop:
for {
	select {
	case receipt := <-publisher.PublishReceipts():
		if !receipt.Success {
			// log?
			// requeue?
			// break WaitLoop?
		}
	default:
		time.Sleep(time.Millisecond * 1)
	}
}
```

The default behavior for a RabbitService subscribed to a publisher's PublishReceipts() is to automatically retry `Success == false` receipts with `QueueLetter()`.

</p>
</details>

---

<details><summary>Click for simple publish with confirmation and context example!</summary>
<p>

```golang
ctx, cancel := context.WithTimeout(context.Background(), time.Duration(1)*time.Minute)
letter := tcr.CreateMockRandomLetter("TcrTestQueue")

publisher.PublishWithConfirmationContext(ctx, letter)

WaitLoop:
for {
	select {
	case receipt := <-publisher.PublishReceipts():
		if !receipt.Success {
			// log?
			// requeue?
			// break WaitLoop?
		}
	default:
		time.Sleep(time.Millisecond * 1)
	}
}

cancel()
```
</p>
</details>

---

<details><summary>Click for AutoPublish example!</summary>
<p>

Once you have a publisher, you can perform **StartAutoPublishing**!

```golang
// This tells the Publisher to start reading an **internal queue**, and process Publishing concurrently but with individual RabbitMQ channels.
publisher.StartAutoPublishing()

GetPublishReceipts:
for {
    select {
case receipt := <-publisher.PublishReceipts():
		if !receipt.Success {
			// log?
			// requeue?
			// break WaitLoop?
		}
    default:
        time.Sleep(1 * time.Millisecond)
    }
}
```
Then just invoke QueueLetter to queue up a letter on send. It returns false if it failed to queue up the letter to send. Usually that happens when Shutdown has been called.  

```golang
ok := publisher.QueueLetter(letter) // How simple is that!
```

...or more complex such as...

```golang
for _, letter := range letters {
    if publisher.QueueLetter(letter) {
		//report sucess
	}
}
```

So you can see why we use these message containers called **letter**. The letter has the **body** and **envelope** inside of it. It has everything you need to publish it. Think of it a small, highly configurable, **message body** and the intended **address**. This allows for async replay on failure.

Notice that you don't have anything to do with channels and connections (even on outage)!

</p>
</details>

---

<details><summary>Click for a more convoluted AutoPublish example!</summary>
<p>

Let's say the above example was too simple for you... ...let's up the over engineering a notch on what you can do with AutoPublish.

```golang
publisher.StartAutoPublish() // this will retry based on the Letter.RetryCount passed in.

timer := time.NewTimer(1 * time.Minute) // Stop Listening to notifications after 1 minute.

messageCount = 1000
connectionErrors := 0
successCount := 0
failureCount := 0

ReceivePublishConfirmations:
    for {
        select {
        case <-timer.C:
            break ReceivePublishConfirmations  
        case err := <-connectionPool.Errors():
            if err != nil {
                connectionErrors++ // Count ConnectionPool failures.
            }
            break
    	case publish := <-publisher.PublishReceipts():
            if publish.Success {
                successCount++
            } else {
                failureCount++
            }

            // I am only expecting to publish 1000 messages
            if successCount+failureCount == messageCount { 
                break ReceivePublishConfirmations
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
publisher.Shutdown(false)
```

</p>
</details>

---

## The Consumer

<details><summary>Click for simple Consumer usage example!</summary>
<p>

Consumer provides a simple Get and GetBatch much like the Publisher has a simple Publish.

```golang
delivery, err := consumer.Get("TcrTestQueue")
```

Exit Conditions:

 * On Error: Error Return, Nil Message Return
 * On Not Ok: Nil Error Return, Nil Message Return
 * On OK: Nil Error Return, Message Returned

We also provide a simple Batch version of this call.


```golang
delivery, err := consumer.GetBatch("TcrTestQueue", 10)
```

Exit Conditions:

 * On Error: Error Return, Nil Messages Return
 * On Not Ok: Nil Error Return, Available Messages Return (0 upto (nth - 1) message)
 * When BatchSize is Reached: Nil Error Return, All Messages Return (n messages)

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
		"Enabled": true,
		"QueueName": "TcrTestQueue",
		"ConsumerName": "TurboCookedRabbitConsumer-Ackable",
		"AutoAck": false,
		"Exclusive": false,
		"NoWait": false,
		"QosCountOverride": 100,
		"SleepOnErrorInterval": 0,
		"SleepOnIdleInterval": 0
	}
},
```

And finding this object after it was loaded from a JSON file.

```golang
consumerConfig, ok := config.ConsumerConfigs["TurboCookedRabbitConsumer-AutoAck"]
```

Creating the Consumer from Config after creating a ConnectionPool.

```golang
consumer := consumer.NewConsumerFromConfig(consumerConfig, connectionPool)
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

Alternatively you could provide an action for the consumer (this will bypass your internal message buffer).

```golang
consumer.StartConsumingWithAction(
		func(msg *tcr.ReceivedMessage) {
			if err := msg.Acknowledge(); err != nil {
				fmt.Printf("Error acking message: %v\r\n", msg.Body)
			}
		})
```

</p>
</details>

---

<details><summary>Wait! What the hell is coming out of <-ReceivedMessages()</summary>
<p>

Great question. I toyed with the idea of returning Letters like Publisher uses (and I may still at some point) but for now you receive a `tcr.ReceivedMessage`.

***But... why***? Because the payload/data/message body is in there but, more importantly, it contains the means of quickly acking the message! It didn't feel right being merged with a `tcr.Letter`. I may revert and use the base `amqp.Delivery` which does all this and more... I just didn't want users to have to also pull in `streadway/amqp` to simplify their imports. If you were already using it wouldn't be an issue. This design is still being code reviewed in my head.

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
    case message := <-consumer.ReceivedMessages(): // View Messages
        fmt.Printf("Message Received: %s\r\n", string(message.Body))
    case err := <-consumer.Errors(): // View Consumer errors
        /* Handle */
    case err := <-ConnectionPool.Errors(): // View ConnectionPool errors
        /* Handle */
    default:
        time.Sleep(100 * time.Millisecond)
        break
    }
}
```

Here you may trigger StopConsuming with this

```golang
immediately := false
flushMessages := false
err = consumer.StopConsuming(immediately, flushMessages)
```

But be mindful there are Channel Buffers internally that may be full and goroutines waiting to add even more.

I have provided some tools that can be used to help with this. You will see them sprinkled periodically through my tests.

```golang
consumer.FlushStop() // could have been called more than once.
consumer.FlushErrors() // errors can quickly build up if you stop listening to them
consumer.FlushMessages() // lets say the ackable messages you have can't be acked and you just need to flush them all out of memory
```

Becareful with `FlushMessages()`. If you are `autoAck = true` and receiving ackAble messages, this is not safe. You will **wipe them from your memory** and ***they are not still in the original queue!*** If you were using manual ack, i.e. `autoAck = false` then you are free to do so safely. Your next consumer will pick up where you left off.

Here I demonstrate a very busy ***ConsumerLoop***. Just replace all the counter variables with logging and then an action performed with the message and this could be a production microservice loop.

```golang
ConsumeLoop:
	for {
		select {
		case <-timeOut:
			break ConsumeLoop
		case receipt := <-publisher.PublishReceipts():
			if receipt.Success {
				fmt.Printf("%s: Published Success - LetterID: %d\r\n", time.Now(), receipt.LetterID)
				messagesPublished++
			} else {
				fmt.Printf("%s: Published Failed Error - LetterID: %d\r\n", time.Now(), receipt.LetterID)
				messagesFailedToPublish++
			}
		case err := <-ConnectionPool.Errors():
			fmt.Printf("%s: ConnectionPool Error - %s\r\n", time.Now(), err)
			connectionPoolErrors++
		case err := <-consumer.Errors():
			fmt.Printf("%s: Consumer Error - %s\r\n", time.Now(), err)
			consumerErrors++
		case message := <-consumer.ReceivedMessages():
			messagesReceived++
			fmt.Printf("%s: ConsumedMessage\r\n", time.Now())
			go func(msg *tcr.ReceivedMessage) {
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

<details><summary>ConnectionPools, how do they even work?!</summary>
<p>

ChannelPools have been removed for simplification. Unfortunately for the ConnectionPool, there is still a bit of complexity here. If you have one Connection, I generally recommend around 5 Channels to be built on top of each connection. Your mileage may vary so be sure to test!

Ex.) ConnectionCount: 5 => ChannelCount: 25  

I allow most features to be configurable via PoolConfig.  

```javascript
"PoolConfig": {
	"URI": "amqp://guest:guest@localhost:5672/",
	"ConnectionName": "TurboCookedRabbit",
	"SleepOnErrorInterval": 100,
	"MaxCacheChannelCount": 50,
	"MaxConnectionCount": 10,
	"Heartbeat": 30,
	"ConnectionTimeout": 10,
	"TLSConfig": {
		"EnableTLS": false,
		"PEMCertLocation": "test/catest.pem",
		"LocalCertLocation": "client/cert.ca",
		"CertServerName": "hostname-in-cert"
	}
}
```

There is a chance for a pause/delay/lag when there are no Connections/Channels available. High performance on your system may require fine tuning and benchmarking. The thing is though, you can't just add Connections and Channels evenly. Connections, server side, are not an infinite resource (channel construction/destruction isn't really either!). You can't keep just adding connections though so I alleviate that by keeping them cached/pooled for you.

The following code demonstrates one super important part with ConnectionPools: **flag erred Channels**. RabbitMQ server closes Channels on error, meaning this little guy is dead. You normally won't know it's dead until the next time you use it - and that can mean messages lost. By flagging the channel as having had an error, when returning it, we process the dead channel and attempt replace it.

```golang
// Publish sends a single message to the address on the letter using a cached ChannelHost.
// Subscribe to PublishReceipts to see success and errors.
// For proper resilience (at least once delivery guarantee over shaky network) use PublishWithConfirmation
func (pub *Publisher) Publish(letter *Letter, skipReceipt bool) {

	chanHost := pub.ConnectionPool.GetChannelFromPool()

	err := chanHost.Channel.Publish(
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

	if !skipReceipt {
		pub.publishReceipt(letter, err)
	}

	pub.ConnectionPool.ReturnChannel(chanHost, err != nil)
}
```

</p>
</details>

---

<details><summary>Click here to see how to build a Connection and Channel Pool!</summary>
<p>

Um... this is the easy relatively easy to do with configs.

```golang
cp, err := tcr.NewConnectionPool(Seasoning.PoolConfig)
if err != nil {
	// blow up
}
```

</p>
</details>

---

<details><summary>Click here to see how to get and use a Connection!</summary>
<p>

This isn't really necessary to use `amqp.Connection` directly in my library, but you are free to do so.

```golang
connHost, err := ConnectionPool.GetConnection()
channel, err := connHost.Connection.Channel()
if err != nil {
	ConnectionPool.ReturnConnection(connHost, true)
}
ConnectionPool.ReturnConnection(connHost, false)
```

This ChannelHost is like a wrapper around the AmqpChannel that adds a few features like Errors and ReturnMessages. You also don't have to use my Publisher, Consumer, and Topologer, or RabbitService. You can use the ConnectionPool by itself if you just like the idea of backing your already existing code behind a ConnectionPool that has recovery and TCP socket load balancing!

The Publisher/Consumer/Topologer all use code similar to this and should help provide a simple understanding of the ConnectionPool.

```golang
chanHost := ConnectionPool.GetChannelFromPool()
err := chanHost.Channel.Publish(
		exchangeName,
		routingKey,
		mandatory,
		immediate,
		amqp.Publishing{
			ContentType: contentType,
			Body:        body,
		},
    )
ConnectionPool.ReturnChannel(chanHost, err != nil)
```

</p>
</details>

---

<details><summary>Click here to see how to get and use a Channel!</summary>
<p>

```golang
chanHost := ConnectionPool.GetChannelFromPool()

ConnectionPool.ReturnChannel(chanHost, false)
```

This ChannelHost is like a wrapper around the AmqpChannel that adds a few features like Errors and ReturnMessages. You also don't have to use my Publisher, Consumer, and Topologer, or RabbitService. You can use the ConnectionPool yourself if you just like the idea of backing your already existing code behind a ConnectionPool that has recovery and TCP socket load balancing!

The Publisher/Consumer/Topologer all use code similar to this!

```golang
chanHost := ConnectionPool.GetChannelFromPool()
err := chanHost.Channel.Publish(
		exchangeName,
		routingKey,
		mandatory,
		immediate,
		amqp.Publishing{
			ContentType: contentType,
			Body:        body,
		},
    )
ConnectionPool.ReturnChannel(chanHost, err != nil)
```

</p>
</details>

---

<details><summary>Click here to see how to get a transient Channel!</summary>
<p>

This should look like pretty standard RabbitMQ code once you get a normal `amqp.Channel` out.

```golang
ackable := true
channel := ConnectionPool.GetTransientChannel(ackable)
defer channel.Close() // remember to close when you are done!
```

</p>
</details>

---

<details><summary>What happens during an outage?</summary>
<p>

You will get errors performing actions. You indicate to the library your action failed, `err != nil`, and we will try restoring connectivity for you.

</p>
</details>

---

<details><summary>Click to see how one may properly prepare for an outage!</summary>
<p>

Observe a retry publish strategy with the following code example:

```golang
cp, err := tcr.NewConnectionPool(Seasoning.PoolConfig)
if err != nil {
	// blow up
}

iterations := 0
retryCount := 10

for iterations < retryCount {

	chanHost := ConnectionPool.GetChannelFromPool() // we are always getting channels on each publish

	letter := tcr.CreateMockRandomLetter("TcrTestQueue")

	err := chanHost.Channel.Publish(
		letter.Envelope.Exchange,
		letter.Envelope.RoutingKey,
		letter.Envelope.Mandatory,
		letter.Envelope.Immediate,
		amqp.Publishing{
			ContentType: letter.Envelope.ContentType,
			Body:        letter.Body,
		},
	)

	if err == nil {
		ConnectionPool.ReturnChannel(chanHost, false) // we are always returning the channels
		break 
	}

	ConnectionPool.ReturnChannel(chanHost, true) // we are always returning the channels
	time.Sleep(10 * time.Second)
	iterations++

	if iterations == 10 {
		break
	}
}

cp.Shutdown()
```

You can create publishing tests in loops while manually shutting down the RabbitMQ connections! This is great for chaos engineering testing!  

Severing all connections:  
`C:\Program Files\RabbitMQ Server\rabbitmq_server-3.8.5\sbin>rabbitmqctl.bat close_all_connections "suck it, trebek"`  

SleepOnErrorInterval aids in slowing the system down when errors are occurring. Stops you from rapidly taking down your RabbitMQ nodes when they are experiencing issues.

```javascript
"PoolConfig": {
	"URI": "amqp://guest:guest@localhost:5672/",
	"ConnectionName": "TurboCookedRabbit",
	"SleepOnErrorInterval": 100,
	"MaxCacheChannelCount": 50,
	"MaxConnectionCount": 10,
	"Heartbeat": 30,
	"ConnectionTimeout": 10,
	"TLSConfig": {
		"EnableTLS": false,
		"PEMCertLocation": "test/catest.pem",
		"LocalCertLocation": "client/cert.ca",
		"CertServerName": "hostname-in-cert"
	}
},
```

</p>
</details>

---

## The Topologer

<details><summary>How do I create/delete/bind queues and exchanges?</summary>
<p>

Coming from plain `streadway/amqp` there isn't too much to it. Call the right method with the right parameters.

I have however integrated those relatively painless methods now with a ConnectionPool and added a `TopologyConfig` for a JSON style of batch topology creation/binding. The real advantages here is that I allow things in bulk and allow you to build topology from a **topology.json** file.

Creating an Exchange with a `tcr.Exchange`

```golang
err := top.CreateExchangeFromConfig(exchange) // tcr.Exchange
if err != nil {
    return err
}
```

Or if you prefer it more manual:

```golang
exchangeName := "FancyName"
exchangeType := "fanout"
passiveDeclare, durable, autoDelete, internal, noWait := false, true, false, false, false

err := top.CreateExchange(exchangeName, exchangeType, passiveDeclare, durable, autoDelete, internal, noWait, nil)
if err != nil {
    return err
}
```

Creating an Queue with a `tcr.Queue`

```golang
err := top.CreateQueueFromConfig(queue) // tcr.Queue
if err != nil {
    return err
}
```

Or, again, if you prefer it more manual:

```golang
queueName := "FancyQueueName"
passiveDeclare, durable, autoDelete, exclusive, noWait := false, true, false, false, false

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
			"PassiveDeclare": false,
			"Durable": true,
			"AutoDelete": false,
			"InternalOnly": false,
			"NoWait": false
		},
		{
			"Name": "MyTestExchange.Child01",
			"Type": "direct",
			"PassiveDeclare": false,
			"Durable": true,
			"AutoDelete": false,
			"InternalOnly": false,
			"NoWait": false
		},
		{
			"Name": "MyTestExchange.Child02",
			"Type": "direct",
			"PassiveDeclare": false,
			"Durable": true,
			"AutoDelete": false,
			"InternalOnly": false,
			"NoWait": false
		}
	],
	"Queues": [
		{
			"Name": "QueueAttachedToRoot",
			"PassiveDeclare": false,
			"Durable": true,
			"AutoDelete": false,
			"Exclusive": false,
			"NoWait": false
		},
		{
			"Name": "QueueAttachedToExch01",
			"PassiveDeclare": false,
			"Durable": true,
			"AutoDelete": false,
			"Exclusive": false,
			"NoWait": false
		},
		{
			"Name": "QueueAttachedToExch02",
			"PassiveDeclare": false,
			"Durable": true,
			"AutoDelete": false,
			"Exclusive": false,
			"NoWait": false
		}
	],
	"QueueBindings": [
		{
			"QueueName": "QueueAttachedToRoot",
			"ExchangeName": "MyTestExchangeRoot",
			"RoutingKey": "RoutingKeyRoot",
			"NoWait": false
		},
		{
			"QueueName": "QueueAttachedToExch01",
			"ExchangeName": "MyTestExchange.Child01",
			"RoutingKey": "RoutingKey1",
			"NoWait": false
		},
		{
			"QueueName": "QueueAttachedToExch02",
			"ExchangeName": "MyTestExchange.Child02",
			"RoutingKey": "RoutingKey2",
			"NoWait": false
		}
	],
	"ExchangeBindings": [
		{
			"ExchangeName": "MyTestExchange.Child01",
			"ParentExchangeName": "MyTestExchangeRoot",
			"RoutingKey": "ExchangeKey1",
			"NoWait": false
		},
		{
			"ExchangeName": "MyTestExchange.Child02",
			"ParentExchangeName": "MyTestExchange.Child01",
			"RoutingKey": "ExchangeKey2",
			"NoWait": false
		}
	]
}
```

I have provided a helper method for turning it into a TopologyConfig.

```golang
topologyConfig, err := tcr.ConvertJSONFileToTopologyConfig("testtopology.json")
```

Creating a simple and shareable ConnectionPool.

```golang
cp, err := tcr.NewConnectionPool(Seasoning.PoolConfig)
```

Using the ConnectionPool to create our Topologer.

```golang
topologer := tcr.NewTopologer(cp)
```

Assuming you have a blank slate RabbitMQ server, this shouldn't error out as long as you can connect to it.

```golang
ignoreErrors := false
err = topologer.BuildTopology(topologyConfig, ignoreErrors)
```

Fin.

That's it really. In the future I will have more features. Just know that I think you can export your current Server configuration from the Server itself.

</p>
</details>

---

## The RabbitService

<details><summary>Click here to see how RabbitService simplifies things even more!</summary>
<p>

Here I demonstrate the steps of loading the JSON configuration and creating a new RabbitService!

```golang
service, err := tcr.NewRabbitService(Seasoning, "", "", nil, nil)
```

or with encryption/salt added  

```golang
service, err := tcr.NewRabbitService(Seasoning, "PasswordyPassword", "SaltySalt", nil, nil)
```  

optionally providing actions for processing errors and publish receipts

```golang
service, err := tcr.NewRabbitService(
    Seasoning,
    "PasswordyPassword",
    "SaltySalt",
	processPublishReceipts, //func(*PublishReceipt)
	processError) //func(error)
```

RabbitService provides default behaviors for these functions when they are `nil`. On Error for example, we write to console. On PublishReceipts that are unsuccesful, we requeue the message for Publish on your behalf using the AutoPublisher.

The service has direct access to a Publisher and Topologer

```golang
rs.Topologer.CreateExchangeFromConfig(exchange)
skipReceipt := true
rs.Publisher.Publish(letter, skipReceipt)
```

The consumer section is more complicated but I read the map of consumers that were in config and built them out for you to use when ready:

```golang
var consumer *consumer.Consumer
consumer, err := rs.GetConsumer("MyConsumer")
consumer.StartConsuming()
```

And don't forget to subscribe to **ReceivedMessages()** when using **StartConsuming()** to actually get them out of the internal buffer!

</p>
</details>

---

<details><summary>But wait, there's more!</summary>
<p>

The service allows JSON Marshalling, Argon2 hashing, Aes-128/192/256 bit encryption, and GZIP/ZSTD compression.  
***Note: ZSTD is from 3rd party library and it's working but in Beta - if worried use the standard vanilla GZIP.***

Setting Up Hashing (required for Encryption):
```golang
password := "SuperStreetFighter2Turbo"
salt := "MBisonDidNothingWrong"

Service.SetHashForEncryption(password, salt)
```

The password/passphrase is your responsibility on keeping it safe. I recommend a Key Vault of some flavor.

We set the **HashKey** internally to the Service so you can do seamless encryption during Service.Publish and what you have in the corresponding **Configs** added to **RabbitSeasoning**. Here are some decent settings for Argon2 hashing.

```javascript
"EncryptionConfig" : {
	"Enabled": true,
	"Type": "aes",
	"TimeConsideration": 1,
	"MemoryMultiplier": 64,
	"Threads": 2
},
"CompressionConfig": {
	"Enabled": true,
	"Type": "gzip"
},
```

And all of this is built-in into the Service level Publisher.

Here are some examples...

JSON Marshalled Data Example:
```golang
Service.Config.EncryptionConfig.Enabled = false
Service.Config.CompressionConfig.Enabled = false

wrapData := false
data := interface{}
err := Service.Publish(data, "MyExchange", "MyQueue", wrapData)
if err != nil {
	
}
```
Isn't that easy?

Let's add compression!

 1. Marshal interface{} into bytes.
 2. Compress bytes.
 3. Publish.

```golang
Service.Config.EncryptionConfig.Enabled = false
Service.Config.CompressionConfig.Enabled = true
Service.Config.CompressionConfig.Type = "gzip"

wrapData := false
data := interface{}
err := Service.Publish(data, "MyExchange", "MyQueue", wrapData)
if err != nil {
	
}
```

To reverse it into a struct!

 * Consume Message (get your bytes)
 * Decompress Bytes (with matching type)
 * Unmarshal bytes to your struct!
 * Profit!

What about Encryption?

Well if you are following my config example, we will encrypt using a SymmetricKey / AES-256 bit, with nonce and a salty 32-bit HashKey from Argon2.

```golang
Service.Config.EncryptionConfig.Enabled = true
Service.Config.CompressionConfig.Enabled = false

wrapData := false
data := interface{}
err := Service.Publish(data, "MyExchange", "MyQueue", wrapData)
if err != nil {
	
}
```

Boom, finished! That's it. You have encrypted your entire payload in the queue. Nobody can read it without your passphrase and salt.

So to reverse it into a struct, you need to:

 * Consume Message (get your bytes)
 * Decrypt Bytes (with matching type)
 * Unmarshal bytes to your struct!
 * Profit!

 What about Comcryption (a word I just made up)?

 Good lord, fine!

The steps this takes is this:
  1. Marshal interface{} into bytes.
  2. Compress bytes.
  3. Encrypt bytes.
  4. Publish.

 ```golang
Service.Config.EncryptionConfig.Enabled = true
Service.Config.CompressionConfig.Enabled = true

wrapData := false
data := interface{}
err := Service.Publish(data, "MyExchange", "MyQueue", wrapData)
if err != nil {
	
}
```

So to reverse comcryption, you need to:

 * Consume Message (get your bytes)
 * Decrypt Bytes (with matching type)
 * Decompress bytes (with matching type)
 * Unmarshal bytes to your struct!

Depending on your payloads, if it's tons of random bytes/strings, compression won't do much for you - probably even increase size. AES encryption only adds little byte size overhead for the nonce I believe.

Here is a possible ***good*** use case for comcryption. It is a beefy 5KB+ JSON string of dynamic, but not random, sensitive data. Quite possibly PII/PCI user data dump. Think list of Credit Cards, Transactions, or HIPAA data. Basically anything you would see in GDPR bingo!

So healthy sized JSONs generally compress well ~ 85-97% at times.
If it's sensitive, it needs to be encrypted.
Smaller (compressed) bytes encrypt faster.
Comcryption!

So what's the downside? It's slow, might need tweaking still... but ***it's slow***. At least compared to plain publishing.

 ### SECURITY WARNING
 
 This doesn't really apply to my use cases, however, some forms of deflate/gzip, combined with some protocols, created a vulnerability by compressing and then encrypting. 

 I would be terrible if I didn't make you aware of CRIME and BREACH attacks.
 https://crypto.stackexchange.com/questions/29972/is-there-an-existing-cryptography-algorithm-method-that-both-encrypts-and-comp/29974#29974

So you can choose wisely :)

</p>
</details>

---

<details><summary>Wait... what was that wrap boolean?</summary>
<p>

I knew I forgot something!

Consider the following example, here we are performing Comcryption.

```golang
Service.Config.EncryptionConfig.Enabled = true
Service.Config.CompressionConfig.Enabled = true

wrapData := false
data := interface{}
err := Service.Publish(data, "MyExchange", "MyQueue", wrapData)
if err != nil {
	
}
```

The problem here is that the message could leave you blinded by the dark! I tried to enhance this process, by wrapping your bits.  
If you wrap your message, it is always of type **tcr.WrappedBody** coming back out.  

The following change (with the above code)...

```golang
wrapData = true
```

...produces this message wrapper.

```javascript
{
	"LetterID": 0,
	"Body": {
		"Encrypted": true,
		"EncryptionType": "aes",
		"Compressed": true,
		"CompressionType": "gzip",
		"UTCDateTime": "2019-09-22T19:13:55Z",
		"Data": "+uLJxH1YC1u5KzJUGTImKcaTccSY3gXsMaCoHneJDF+9/9JDaX/Fort92w8VWyTiKqgQj+2gqIaAXyHwFObtjL3RAxTn5uF/QIguvuZ+/2X8qn/+QDByuCY3qkRKu3HHzmwd+GPfgNacyaQgS2/hD2uoFrwR67W332CHWA=="
	}
}
```

You definitely can't tell this is M. Bison's Social Security Number, but can see the **metadata**.

The idea around this *metadata* is that it could help identify when a passphrase was used to create this, then you can determine which key was live based on ***UTCDateTime***. This means that you have to work out key rotations from your end of things.

The inner Data deserializes to **[]byte**, which means based on a consumed **tcr.WrappedBody**, you know immediately if it is a compressed, encrypted, or just a JSON []byte.

</p>
</details>

---

<details><summary>I think I bitshifted to the 4th dimension... how the hell do I get my object/struct back?</summary>
<p>

I am going to assume we are Comcrypting, so adjust this example to your needs

First we get our data out of a Consumer, once we have a **tcr.Body.Data** []byte, we can begin reversing it.

```golang
var json = jsoniter.ConfigFastest // optional - can use built-in json if you prefer

message := <-consumer.Messages() // get comprypted message

wrappedBody := &tcr.WrappedBody{}
err = json.Unmarshal(message.Body, wrappedBody) // unmarshal as ModdedLetter
if err != nil {
	// I probably have a bug.
}

buffer := bytes.NewBuffer(wrappedBody.Body.Data)

// Helper function to get the original data (pre-wrapped) out based on current service settings.
// You would have to remember to write a decompress/decrypt step without this for comcrypted messages.
err = tcr.ReadPayload(buffer, Service.Config.CompressionConfig, Service.Config.EncryptionConfig)
if err != nil {
	// I probably have a bug.
}

myStruct := &MyStruct{}
err = json.Unmarshal(buffer.Bytes(), myStruct) // unmarshal as actual type!
if err != nil {
	// You probably have a bug!
}
```

</p>
</details>

---
