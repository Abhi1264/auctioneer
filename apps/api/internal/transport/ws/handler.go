package ws

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	conn    *websocket.Conn
	send    chan []byte
	closeFn func()
}

func (c *Client) close() {
	if c.closeFn != nil {
		c.closeFn()
	}
	_ = c.conn.Close()
}

type Handler struct {
	hub      *Hub
	logger   *slog.Logger
	upgrader websocket.Upgrader
}

func NewHandler(hub *Hub, logger *slog.Logger) *Handler {
	return &Handler{
		hub:    hub,
		logger: logger,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(_ *http.Request) bool { return true },
		},
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	auctionID := r.URL.Query().Get("auction_id")
	if auctionID == "" {
		http.Error(w, "auction_id is required", http.StatusBadRequest)
		return
	}
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Warn("websocket upgrade failed", "err", err)
		return
	}

	client := &Client{
		conn: conn,
		send: make(chan []byte, h.hub.queueDepth),
	}
	client.closeFn = func() {
		h.hub.Unregister(auctionID, client)
	}
	h.hub.Register(auctionID, client)

	go h.writePump(client)
	h.readPump(auctionID, client)
}

func (h *Handler) readPump(_ string, c *Client) {
	defer c.close()
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (h *Handler) writePump(c *Client) {
	ticker := time.NewTicker(20 * time.Second)
	defer func() {
		ticker.Stop()
		c.close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
