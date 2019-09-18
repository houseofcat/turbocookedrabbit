package main_test

import (
	"context"
	"runtime/trace"
	"testing"

	"github.com/houseofcat/turbocookedrabbit/pools"
	"github.com/houseofcat/turbocookedrabbit/utils"
	"github.com/streadway/amqp"
)

func BenchmarkGetMultiChannelAndPublish(b *testing.B) {
	b.ReportAllocs()
	_, task := trace.NewTask(context.Background(), "BenchmarkGetMultiChannelAndPublish")
	defer task.End()

	messageSize := 1000
	channelPool, _ := pools.NewChannelPool(Seasoning.PoolConfig, nil, true)
	letter := utils.CreateLetter(1, "", "ConsumerTestQueue", utils.RandomBytes(messageSize))
	poolErrors := 0

	for i := 0; i < 1000; i++ {
		channelHost, err := channelPool.GetChannel()
		if err != nil {
			poolErrors++
			continue
		}

		go channelHost.Channel.Publish(
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

func BenchmarkGetSingleChannelAndPublish(b *testing.B) {
	b.ReportAllocs()
	_, task := trace.NewTask(context.Background(), "BenchmarkGetSingleChannelAndPublish")
	defer task.End()

	messageSize := 1000
	channelPool, _ := pools.NewChannelPool(Seasoning.PoolConfig, nil, true)
	letter := utils.CreateLetter(1, "", "ConsumerTestQueue", utils.RandomBytes(messageSize))

	channelHost, err := channelPool.GetChannel()
	if err != nil {
		b.Log(err.Error())
	}

	for i := 0; i < 1000; i++ {
		channelHost.Channel.Publish(
			letter.Envelope.Exchange,
			letter.Envelope.RoutingKey,
			letter.Envelope.Mandatory,
			letter.Envelope.Immediate,
			amqp.Publishing{
				ContentType: letter.Envelope.ContentType,
				Body:        letter.Body,
			})
	}

	channelPool.ReturnChannel(channelHost, false)
}
