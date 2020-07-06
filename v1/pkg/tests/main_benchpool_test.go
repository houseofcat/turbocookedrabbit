package main_test

import (
	"context"
	"runtime/trace"
	"testing"

	"github.com/houseofcat/turbocookedrabbit/v1/pkg/pools"
	"github.com/houseofcat/turbocookedrabbit/v1/pkg/utils"
	"github.com/streadway/amqp"
)

func BenchmarkGetMultiChannelAndPublish(b *testing.B) {
	b.ReportAllocs()
	_, task := trace.NewTask(context.Background(), "BenchmarkGetMultiChannelAndPublish")
	defer task.End()

	messageSize := 1000
	channelPool, _ := pools.NewChannelPool(Seasoning.PoolConfig, nil, true)
	letter := utils.CreateMockRandomLetter("ConsumerTestQueue")
	poolErrors := 0

	for i := 0; i < messageSize; i++ {
		channelHost, err := channelPool.GetChannel()
		if err != nil {
			poolErrors++
			continue
		}

		go func() {
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
				b.Log(err)
			}
		}()

		channelPool.ReturnChannel(channelHost, false)
	}
}

func BenchmarkGetSingleChannelAndPublish(b *testing.B) {
	b.ReportAllocs()
	_, task := trace.NewTask(context.Background(), "BenchmarkGetSingleChannelAndPublish")
	defer task.End()

	messageSize := 1000
	channelPool, _ := pools.NewChannelPool(Seasoning.PoolConfig, nil, true)
	letter := utils.CreateMockRandomLetter("ConsumerTestQueue")

	var err error
	channelHost, err := channelPool.GetChannel()
	if err != nil {
		b.Log(err.Error())
	}

	for i := 0; i < messageSize; i++ {
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
			b.Log(err)
		}
	}

	channelPool.ReturnChannel(channelHost, false)
}
