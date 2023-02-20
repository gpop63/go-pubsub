package pubsub

import (
	"context"
	"testing"
)

func BenchmarkPublish(b *testing.B) {
	broker := NewBroker[string]()
	defer broker.Close()

	ctx := context.Background()
	sub, _ := broker.Subscribe(ctx, "foo.bar")
	go func() {
		for range sub.C() {
		}
	}()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		broker.Publish(ctx, "foo.bar", "hello")
	}
}

func BenchmarkPublishParallel(b *testing.B) {
	broker := NewBroker[string]()
	defer broker.Close()

	ctx := context.Background()
	sub, _ := broker.Subscribe(ctx, "foo.bar")
	go func() {
		for range sub.C() {
		}
	}()

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			broker.Publish(ctx, "foo.bar", "hello")
		}
	})
}

func BenchmarkPublishWildcardFanOut(b *testing.B) {
	broker := NewBroker[string]()
	defer broker.Close()

	ctx := context.Background()
	for i := 0; i < 100; i++ {
		sub, _ := broker.Subscribe(ctx, "foo.*")
		go func() {
			for range sub.C() {
			}
		}()
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		broker.Publish(ctx, "foo.bar", "hello")
	}
}

func BenchmarkPublishMultiLevelWildcard(b *testing.B) {
	broker := NewBroker[string]()
	defer broker.Close()

	ctx := context.Background()
	sub, _ := broker.Subscribe(ctx, "foo.**")
	go func() {
		for range sub.C() {
		}
	}()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		broker.Publish(ctx, "foo.bar.baz.qux", "hello")
	}
}

func BenchmarkPublishParallelMultiTopic(b *testing.B) {
	broker := NewBroker[string]()
	defer broker.Close()

	ctx := context.Background()
	topics := []string{"topic.a", "topic.b", "topic.c", "topic.d"}
	for _, t := range topics {
		sub, _ := broker.Subscribe(ctx, t)
		go func() {
			for range sub.C() {
			}
		}()
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			broker.Publish(ctx, topics[i%len(topics)], "hello")
			i++
		}
	})
}

func BenchmarkSubscribeUnsubscribe(b *testing.B) {
	broker := NewBroker[string]()
	defer broker.Close()

	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sub, _ := broker.Subscribe(ctx, "foo.bar")
		broker.Unsubscribe(sub)
	}
}
