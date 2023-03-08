package pubsub_test

import (
	"context"
	"fmt"

	"github.com/gpop63/go-pubsub"
)

func ExampleBroker() {
	ctx := context.Background()
	b := pubsub.NewBroker[string]()
	defer b.Close()

	sub, _ := b.Subscribe(ctx, "greetings.english")
	defer sub.Close()

	b.Publish(ctx, "greetings.english", "hello")

	msg := <-sub.C()
	fmt.Println(msg.Payload)
	// Output: hello
}

func ExampleBroker_wildcard() {
	ctx := context.Background()
	b := pubsub.NewBroker[string]()
	defer b.Close()

	// * matches exactly one segment
	sub, _ := b.Subscribe(ctx, "orders.*")
	defer sub.Close()

	b.Publish(ctx, "orders.created", "order-123")

	msg := <-sub.C()
	fmt.Println(msg.Payload)
	// Output: order-123
}

func ExampleBroker_multiLevelWildcard() {
	ctx := context.Background()
	b := pubsub.NewBroker[string]()
	defer b.Close()

	// ** matches one or more segments
	sub, _ := b.Subscribe(ctx, "metrics.**")
	defer sub.Close()

	b.Publish(ctx, "metrics.cpu.host1", "0.95")

	msg := <-sub.C()
	fmt.Println(msg.Topic, msg.Payload)
	// Output: metrics.cpu.host1 0.95
}

func ExampleWithFilter() {
	ctx := context.Background()
	b := pubsub.NewBroker[int]()
	defer b.Close()

	// Only receive even numbers
	sub, _ := b.Subscribe(ctx, "numbers.*", pubsub.WithFilter(func(msg pubsub.Message[int]) bool {
		return msg.Payload%2 == 0
	}))
	defer sub.Close()

	b.Publish(ctx, "numbers.all", 1)
	b.Publish(ctx, "numbers.all", 2)
	b.Publish(ctx, "numbers.all", 3)
	b.Publish(ctx, "numbers.all", 4)

	fmt.Println((<-sub.C()).Payload)
	fmt.Println((<-sub.C()).Payload)
	// Output:
	// 2
	// 4
}
