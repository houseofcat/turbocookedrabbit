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
	connectionPool, _ := pools.NewConnectionPool(Seasoning.PoolConfig)
	letter := utils.CreateMockRandomLetter("ConsumerTestQueue")

	for i := 0; i < messageSize; i++ {
		channelHost := connectionPool.GetChannel(false)

		go func() {
			_ = channelHost.Channel.Publish(
				letter.Envelope.Exchange,
				letter.Envelope.RoutingKey,
				letter.Envelope.Mandatory,
				letter.Envelope.Immediate,
				amqp.Publishing{
					ContentType: letter.Envelope.ContentType,
					Body:        letter.Body,
				})
		}()

		channelHost.Close()
	}
}

func BenchmarkGetSingleChannelAndPublish(b *testing.B) {
	b.ReportAllocs()
	_, task := trace.NewTask(context.Background(), "BenchmarkGetSingleChannelAndPublish")
	defer task.End()

	messageSize := 1000
	connectionPool, _ := pools.NewConnectionPool(Seasoning.PoolConfig)
	letter := utils.CreateMockRandomLetter("ConsumerTestQueue")

	channelHost := connectionPool.GetChannel(false)

	for i := 0; i < messageSize; i++ {
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
			b.Fatal(err)
		}
	}

	channelHost.Close()
}
