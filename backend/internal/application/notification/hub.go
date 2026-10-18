package notification

import "sync"

// Hub 管理每个用户当前在线的 SSE 连接。
type Hub struct {
	mu          sync.RWMutex
	subscribers map[uint64]map[chan Output]struct{}
}

func NewHub() *Hub {
	return &Hub{subscribers: make(map[uint64]map[chan Output]struct{})}
}

func (h *Hub) Subscribe(userID uint64) (<-chan Output, func()) {
	channel := make(chan Output, 16)
	h.mu.Lock()
	if h.subscribers[userID] == nil {
		h.subscribers[userID] = make(map[chan Output]struct{})
	}
	h.subscribers[userID][channel] = struct{}{}
	h.mu.Unlock()
	return channel, func() {
		h.mu.Lock()
		delete(h.subscribers[userID], channel)
		if len(h.subscribers[userID]) == 0 {
			delete(h.subscribers, userID)
		}
		h.mu.Unlock()
	}
}

func (h *Hub) Publish(userID uint64, value Output) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for channel := range h.subscribers[userID] {
		select {
		case channel <- value:
		default:
			// 慢连接主动断开，由客户端携带 Last-Event-ID 重连补偿，避免静默丢消息。
			close(channel)
			delete(h.subscribers[userID], channel)
		}
	}
	if len(h.subscribers[userID]) == 0 {
		delete(h.subscribers, userID)
	}
}
