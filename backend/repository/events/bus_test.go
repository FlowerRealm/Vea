package events

import "testing"

func TestBus_PublishSync_CallsTypeAndAllHandlers(t *testing.T) {
	t.Parallel()

	bus := NewBus()

	calls := make(chan EventType, 2)
	bus.Subscribe(EventNodeCreated, func(event Event) {
		calls <- event.Type()
	})
	bus.SubscribeAll(func(event Event) {
		calls <- event.Type()
	})

	bus.PublishSync(NodeEvent{EventType: EventNodeCreated})

	got1 := <-calls
	got2 := <-calls

	if got1 != EventNodeCreated || got2 != EventNodeCreated {
		t.Fatalf("unexpected calls: %v, %v", got1, got2)
	}
}
