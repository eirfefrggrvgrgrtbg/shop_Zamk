package auctions

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type AuctionRealtimeEvent struct {
	EventType       string         `json:"eventType"`
	AuctionID       uuid.UUID      `json:"auctionId"`
	LotID           *uuid.UUID     `json:"lotId,omitempty"`
	BidID           *uuid.UUID     `json:"bidId,omitempty"`
	CurrentBidCents *int64         `json:"currentBidCents,omitempty"`
	BidStepCents    *int64         `json:"bidStepCents,omitempty"`
	EndsAt          *time.Time     `json:"endsAt,omitempty"`
	LotStatus       *LotStatus     `json:"lotStatus,omitempty"`
	AuctionStatus   *AuctionStatus `json:"auctionStatus,omitempty"`
	ServerTime      time.Time      `json:"serverTime"`
}

type SSEHub struct {
	mu          sync.RWMutex
	subscribers map[uuid.UUID]map[chan AuctionRealtimeEvent]bool
}

func NewSSEHub() *SSEHub {
	return &SSEHub{
		subscribers: make(map[uuid.UUID]map[chan AuctionRealtimeEvent]bool),
	}
}

func (h *SSEHub) Subscribe(auctionID uuid.UUID) chan AuctionRealtimeEvent {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.subscribers[auctionID] == nil {
		h.subscribers[auctionID] = make(map[chan AuctionRealtimeEvent]bool)
	}

	// Create a buffered channel to avoid blocking the broadcaster
	ch := make(chan AuctionRealtimeEvent, 64)
	h.subscribers[auctionID][ch] = true
	return ch
}

func (h *SSEHub) Unsubscribe(auctionID uuid.UUID, ch chan AuctionRealtimeEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.subscribers[auctionID]; ok {
		if _, exists := clients[ch]; exists {
			delete(clients, ch)
			close(ch)
			if len(clients) == 0 {
				delete(h.subscribers, auctionID)
			}
		}
	}
}

func (h *SSEHub) Broadcast(auctionID uuid.UUID, event AuctionRealtimeEvent) {
	if event.ServerTime.IsZero() {
		event.ServerTime = time.Now()
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	clients, ok := h.subscribers[auctionID]
	if !ok {
		return
	}

	for ch := range clients {
		select {
		case ch <- event:
		default:
			// If channel is full, drop the event to avoid blocking
		}
	}
}
