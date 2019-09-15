package main_test

import (
	"context"
	"runtime/trace"
	"testing"
	"time"

	"github.com/houseofcat/turbocookedrabbit/pools"
	"github.com/houseofcat/turbocookedrabbit/utils"
	"github.com/streadway/amqp"
)

func TestCreateSingleChannelAndPublish(t *testing.T) {

	startTime := time.Now()
	t.Logf("%s: Benchmark Starts\r\n", startTime)

	messageCount := 100000
	messageSize := 2500
	publishErrors := 0

	letter := utils.CreateLetter("", "ConsumerTestQueue", utils.RandomBytes(messageSize))

	amqpConn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		t.Error(err)
		return
	}
	amqpChan, err := amqpConn.Channel()
	if err != nil {
		t.Error(err)
		return
	}

	for i := 0; i < messageCount; i++ {

		err := amqpChan.Publish(
			letter.Envelope.Exchange,
			letter.Envelope.RoutingKey,
			letter.Envelope.Mandatory,
			true,
			amqp.Publishing{
				ContentType: letter.Envelope.ContentType,
				Body:        letter.Body,
			})
		if err != nil {
			publishErrors++
		}
	}

	amqpChan.Close()
	amqpConn.Close()

	duration := time.Since(startTime)

	t.Logf("%s: Benchmark End\r\n", time.Now())
	t.Logf("%s: Time Elapsed %s\r\n", time.Now(), duration)
	t.Logf("%s: Publish Errors %d\r\n", time.Now(), publishErrors)
	t.Logf("%s: Msgs/s %f\r\n", time.Now(), float64(messageCount)/duration.Seconds())
	t.Logf("%s: KB/s %f\r\n", time.Now(), (float64(messageSize)/duration.Seconds())/1000)
}

func TestGetSingleChannelFromPoolAndPublish(t *testing.T) {

	startTime := time.Now()
	t.Logf("%s: Benchmark Starts\r\n", startTime)

	messageCount := 100000
	messageSize := 2500

	letter := utils.CreateLetter("", "ConsumerTestQueue", utils.RandomBytes(messageSize))

	channelHost, err := ChannelPool.GetChannel()
	if err != nil {
		t.Error(err)
		return
	}
	publishErrors := 0

	for i := 0; i < messageCount; i++ {

		err := channelHost.Channel.Publish(
			letter.Envelope.Exchange,
			letter.Envelope.RoutingKey,
			letter.Envelope.Mandatory,
			letter.Envelope.Immediate,
			amqp.Publishing{
				ContentType: letter.Envelope.ContentType,
				Body:        letter.Body,
			})
		if err != nil {
			if publishErrors == 0 {
				t.Error(err)
			}
			publishErrors++
		}
	}

	ChannelPool.ReturnChannel(channelHost, false)

	duration := time.Since(startTime)

	t.Logf("%s: Benchmark End\r\n", time.Now())
	t.Logf("%s: Time Elapsed %s\r\n", time.Now(), duration)
	t.Logf("%s: Publish Errors %d\r\n", time.Now(), publishErrors)
	t.Logf("%s: Msgs/s %f\r\n", time.Now(), float64(messageCount)/duration.Seconds())
	t.Logf("%s: KB/s %f\r\n", time.Now(), (float64(messageSize)/duration.Seconds())/1000)
}

func TestGetMultiChannelFromPoolAndPublish(t *testing.T) {

	startTime := time.Now()
	t.Logf("%s: Benchmark Starts\r\n", startTime)

	poolErrors := 0
	publishErrors := 0
	messageCount := 100000
	messageSize := 2500

	letter := utils.CreateLetter("", "ConsumerTestQueue", utils.RandomBytes(messageSize))

	for i := 0; i < messageCount; i++ {
		channelHost, err := ChannelPool.GetChannel()
		if err != nil {
			poolErrors++
			continue
		}

		err = channelHost.Channel.Publish(
			letter.Envelope.Exchange,
			letter.Envelope.RoutingKey,
			letter.Envelope.Mandatory,
			letter.Envelope.Immediate,
			amqp.Publishing{
				ContentType: letter.Envelope.ContentType,
				Body:        letter.Body,
			})
		if err != nil {
			publishErrors++
			ChannelPool.ReturnChannel(channelHost, true)
			continue
		}

		ChannelPool.ReturnChannel(channelHost, false)
	}

	duration := time.Since(startTime)

	t.Logf("%s: Benchmark End\r\n", time.Now())
	t.Logf("%s: Time Elapsed %s\r\n", time.Now(), duration)
	t.Logf("%s: ChannelPool Errors %d\r\n", time.Now(), poolErrors)
	t.Logf("%s: Publish Errors %d\r\n", time.Now(), publishErrors)
	t.Logf("%s: Msgs/s %f\r\n", time.Now(), float64(messageCount)/duration.Seconds())
	t.Logf("%s: KB/s %f\r\n", time.Now(), (float64(messageSize)/duration.Seconds())/1000)
}

func BenchmarkGetChannelAndPublish(b *testing.B) {
	b.ReportAllocs()
	_, task := trace.NewTask(context.Background(), "BenchmarkGetChannel")
	defer task.End()

	channelPool, _ := pools.NewChannelPool(Seasoning.PoolConfig, nil, true)
	letter := utils.CreateLetter("", "ConsumerTestQueue", utils.RandomBytes(1000))
	poolErrors := 0

	for i := 0; i < 1000; i++ {
		channelHost, err := channelPool.GetChannel()
		if err != nil {
			poolErrors++
			continue
		}
		channelHost.Channel.Publish(
			letter.Envelope.Exchange,
			letter.Envelope.RoutingKey,
			letter.Envelope.Mandatory,
			letter.Envelope.Immediate,
			amqp.Publishing{
				ContentType: letter.Envelope.ContentType,
				Body:        letter.Body,
			})

		channelPool.ReturnChannel(channelHost, false)
	}
}
