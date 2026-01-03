package events

import "sync"

// Handler 事件处理器
type Handler func(event Event)

// Bus 事件总线
type Bus struct {
	mu       sync.RWMutex
	handlers map[EventType][]Handler
}

// NewBus 创建新的事件总线
func NewBus() *Bus {
	return &Bus{
		handlers: make(map[EventType][]Handler),
	}
}

// Subscribe 订阅指定类型的事件
func (b *Bus) Subscribe(eventType EventType, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], handler)
}

// SubscribeAll 订阅所有事件
func (b *Bus) SubscribeAll(handler Handler) {
	b.Subscribe(EventAll, handler)
}

// Unsubscribe 取消订阅（通过重置该类型的所有处理器）
func (b *Bus) Unsubscribe(eventType EventType) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.handlers, eventType)
}

// Publish 发布事件（异步执行所有处理器）
func (b *Bus) Publish(event Event) {
	b.mu.RLock()
	// 复制处理器列表，避免在锁内执行用户代码
	handlers := make([]Handler, 0)
	handlers = append(handlers, b.handlers[event.Type()]...)
	handlers = append(handlers, b.handlers[EventAll]...)
	b.mu.RUnlock()

	// 异步执行所有处理器
	for _, h := range handlers {
		go h(event)
	}
}

// PublishSync 发布事件（同步执行所有处理器）
func (b *Bus) PublishSync(event Event) {
	b.mu.RLock()
	// 复制处理器列表，避免在锁内执行用户代码
	handlers := make([]Handler, 0)
	handlers = append(handlers, b.handlers[event.Type()]...)
	handlers = append(handlers, b.handlers[EventAll]...)
	b.mu.RUnlock()

	// 同步执行所有处理器
	for _, h := range handlers {
		h(event)
	}
}

// HasSubscribers 检查是否有订阅者
func (b *Bus) HasSubscribers(eventType EventType) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.handlers[eventType]) > 0 || len(b.handlers[EventAll]) > 0
}

// Clear 清除所有订阅
func (b *Bus) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers = make(map[EventType][]Handler)
}
