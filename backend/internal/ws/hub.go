package ws

import (
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096
)

// Event represents a WebSocket event broadcast to clients.
type Event struct {
	Type       string      `json:"type"`
	IncidentID string      `json:"incident_id,omitempty"`
	OrgID      string      `json:"org_id,omitempty"`
	Data       interface{} `json:"data"`
	Timestamp  string      `json:"timestamp"`
}

// Client represents a single WebSocket connection.
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	orgID  string
	userID string
	topics map[string]bool
	mu     sync.RWMutex
}

// clientMessage represents an incoming message from a client.
type clientMessage struct {
	Action string   `json:"action"`
	Topics []string `json:"topics,omitempty"`
}

// Hub maintains the set of active clients and broadcasts events.
type Hub struct {
	clients    map[string]map[*Client]bool // orgID -> clients
	broadcast  chan Event
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]map[*Client]bool),
		broadcast:  make(chan Event, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the hub's main loop. Run this as a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if h.clients[client.orgID] == nil {
				h.clients[client.orgID] = make(map[*Client]bool)
			}
			h.clients[client.orgID][client] = true
			h.mu.Unlock()
			slog.Info("ws client registered", "org_id", client.orgID, "user_id", client.userID)

		case client := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.clients[client.orgID]; ok {
				if _, ok := clients[client]; ok {
					delete(clients, client)
					close(client.send)
					if len(clients) == 0 {
						delete(h.clients, client.orgID)
					}
				}
			}
			h.mu.Unlock()
			slog.Info("ws client unregistered", "org_id", client.orgID, "user_id", client.userID)

		case event := <-h.broadcast:
			h.mu.RLock()
			orgClients := h.clients[event.OrgID]
			h.mu.RUnlock()

			data, err := json.Marshal(event)
			if err != nil {
				slog.Error("ws marshal error", "error", err)
				continue
			}

			for client := range orgClients {
				if !client.matchesTopic(event) {
					continue
				}
				select {
				case client.send <- data:
				default:
					h.unregister <- client
				}
			}
		}
	}
}

// Broadcast sends an event to all matching clients.
func (h *Hub) Broadcast(event Event) {
	if event.Timestamp == "" {
		event.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	h.broadcast <- event
}

func (c *Client) matchesTopic(event Event) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.topics) == 0 {
		return true // no filter = receive all
	}
	if c.topics["all"] {
		return true
	}
	if event.IncidentID != "" {
		return c.topics["incident:"+event.IncidentID]
	}
	return true
}

// NewClient creates a new Client and registers it with the hub.
func NewClient(hub *Hub, conn *websocket.Conn, orgID, userID string) *Client {
	client := &Client{
		hub:    hub,
		conn:   conn,
		send:   make(chan []byte, 256),
		orgID:  orgID,
		userID: userID,
		topics: make(map[string]bool),
	}
	hub.register <- client
	return client
}

// ReadPump reads messages from the WebSocket connection.
func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				slog.Warn("ws read error", "error", err)
			}
			break
		}
		c.handleMessage(message)
	}
}

// WritePump writes messages to the WebSocket connection.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleMessage(data []byte) {
	var msg clientMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}

	switch strings.ToLower(msg.Action) {
	case "subscribe":
		c.mu.Lock()
		for _, t := range msg.Topics {
			c.topics[t] = true
		}
		c.mu.Unlock()
	case "unsubscribe":
		c.mu.Lock()
		for _, t := range msg.Topics {
			delete(c.topics, t)
		}
		c.mu.Unlock()
	case "ping":
		resp, _ := json.Marshal(map[string]string{"type": "pong"})
		select {
		case c.send <- resp:
		default:
		}
	}
}
