package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/redis/go-redis/v9"
)

// RedisBridge subscribes to Redis pub/sub and forwards events to the WebSocket Hub.
type RedisBridge struct {
	rdb *redis.Client
	hub *Hub
}

// NewRedisBridge creates a new RedisBridge.
func NewRedisBridge(rdb *redis.Client, hub *Hub) *RedisBridge {
	return &RedisBridge{rdb: rdb, hub: hub}
}

// Start subscribes to Redis channels and forwards events. Blocks until ctx is cancelled.
func (b *RedisBridge) Start(ctx context.Context) {
	pubsub := b.rdb.PSubscribe(ctx, "rootcauseway:*")
	defer pubsub.Close()

	ch := pubsub.Channel()
	slog.Info("redis bridge started, subscribing to rootcauseway:*")

	for {
		select {
		case <-ctx.Done():
			slog.Info("redis bridge stopping")
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			b.handleMessage(msg)
		}
	}
}

func (b *RedisBridge) handleMessage(msg *redis.Message) {
	var envelope models.EventEnvelope
	if err := json.Unmarshal([]byte(msg.Payload), &envelope); err != nil {
		slog.Warn("redis bridge: failed to parse event", "channel", msg.Channel, "error", err)
		return
	}

	event := Event{
		Type:      envelope.EventType,
		OrgID:     envelope.OrgID.String(),
		Data:      envelope.Payload,
		Timestamp: envelope.Timestamp.UTC().Format(time.RFC3339),
	}

	// Extract incident_id from payload if available
	if payloadBytes, err := json.Marshal(envelope.Payload); err == nil {
		var p struct {
			IncidentID string `json:"incident_id"`
		}
		if json.Unmarshal(payloadBytes, &p) == nil && p.IncidentID != "" {
			event.IncidentID = p.IncidentID
		}
	}

	b.hub.Broadcast(event)
}
