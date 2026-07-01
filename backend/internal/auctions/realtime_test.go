package auctions

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSSEHub(t *testing.T) {
	hub := NewSSEHub()
	auctionID := uuid.New()

	// Test Subscription
	ch1 := hub.Subscribe(auctionID)
	ch2 := hub.Subscribe(auctionID)

	if len(hub.subscribers[auctionID]) != 2 {
		t.Errorf("expected 2 subscribers, got %d", len(hub.subscribers[auctionID]))
	}

	// Test Broadcast
	event := AuctionRealtimeEvent{
		EventType: "bid_accepted",
		AuctionID: auctionID,
	}

	hub.Broadcast(auctionID, event)

	select {
	case ev := <-ch1:
		if ev.EventType != "bid_accepted" {
			t.Errorf("expected event type bid_accepted, got %s", ev.EventType)
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for event on ch1")
	}

	select {
	case ev := <-ch2:
		if ev.EventType != "bid_accepted" {
			t.Errorf("expected event type bid_accepted, got %s", ev.EventType)
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for event on ch2")
	}

	// Test Unsubscribe
	hub.Unsubscribe(auctionID, ch1)
	if len(hub.subscribers[auctionID]) != 1 {
		t.Errorf("expected 1 subscriber, got %d", len(hub.subscribers[auctionID]))
	}

	hub.Unsubscribe(auctionID, ch2)
	if len(hub.subscribers) != 0 {
		t.Errorf("expected 0 active auctions in hub, got %d", len(hub.subscribers))
	}
}
