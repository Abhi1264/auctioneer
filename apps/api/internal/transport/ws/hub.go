package ws

import (
	"encoding/json"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type Message struct {
	Type      string `json:"type"`
	AuctionID string `json:"auction_id"`
	BidID     string `json:"bid_id,omitempty"`
	UserID    string `json:"user_id,omitempty"`
	Amount    int64  `json:"amount_cents,omitempty"`
	Version   int64  `json:"version,omitempty"`
	Timestamp int64  `json:"timestamp_ms,omitempty"`
}

type subscription struct {
	auctionID string
	client    *Client
}

type Hub struct {
	queueDepth int
	dropped    prometheus.Counter

	register   chan subscription
	unregister chan subscription
	broadcast  chan targetedMessage

	mu      sync.RWMutex
	clients map[string]map[*Client]struct{}
}

type targetedMessage struct {
	auctionID string
	payload   []byte
}

func NewHub(queueDepth int, dropped prometheus.Counter) *Hub {
	return &Hub{
		queueDepth: queueDepth,
		dropped:    dropped,
		register:   make(chan subscription, 1024),
		unregister: make(chan subscription, 1024),
		broadcast:  make(chan targetedMessage, 4096),
		clients:    make(map[string]map[*Client]struct{}),
	}
}

func (h *Hub) Register(auctionID string, c *Client) {
	h.register <- subscription{auctionID: auctionID, client: c}
}

func (h *Hub) Unregister(auctionID string, c *Client) {
	h.unregister <- subscription{auctionID: auctionID, client: c}
}

func (h *Hub) Broadcast(auctionID string, msg Message) {
	payload, _ := json.Marshal(msg)
	h.broadcast <- targetedMessage{auctionID: auctionID, payload: payload}
}

func (h *Hub) Run() {
	for {
		select {
		case sub := <-h.register:
			h.mu.Lock()
			if _, ok := h.clients[sub.auctionID]; !ok {
				h.clients[sub.auctionID] = make(map[*Client]struct{})
			}
			h.clients[sub.auctionID][sub.client] = struct{}{}
			h.mu.Unlock()
		case sub := <-h.unregister:
			h.mu.Lock()
			if set, ok := h.clients[sub.auctionID]; ok {
				delete(set, sub.client)
				if len(set) == 0 {
					delete(h.clients, sub.auctionID)
				}
			}
			h.mu.Unlock()
		case msg := <-h.broadcast:
			h.mu.RLock()
			var dropped []*Client
			for client := range h.clients[msg.auctionID] {
				select {
				case client.send <- msg.payload:
				default:
					h.dropped.Inc()
					dropped = append(dropped, client)
				}
			}
			h.mu.RUnlock()
			if len(dropped) == 0 {
				continue
			}
			h.mu.Lock()
			set := h.clients[msg.auctionID]
			for _, client := range dropped {
				if set != nil {
					delete(set, client)
				}
				client.close()
			}
			if set != nil && len(set) == 0 {
				delete(h.clients, msg.auctionID)
			}
			h.mu.Unlock()
		}
	}
}

func (h *Hub) KnownAuctions() map[string]struct{} {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make(map[string]struct{}, len(h.clients))
	for auctionID := range h.clients {
		out[auctionID] = struct{}{}
	}
	return out
}
