package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/redis/go-redis/v9"
)

// Defaults for the durable event stream. See contracts/events/redis-events.yaml.
const (
	// DefaultEventStream is the Redis Stream all backend events are XADDed to.
	DefaultEventStream = "rootcauseway:events"
	// DefaultEventStreamMaxLen bounds the stream size (approximate trimming).
	DefaultEventStreamMaxLen int64 = 100000
)

// eventRedisClient abstracts the Redis commands used by RedisEventPublisher,
// so the publisher can be unit-tested with a fake client.
type eventRedisClient interface {
	Publish(ctx context.Context, channel string, message interface{}) *redis.IntCmd
	XAdd(ctx context.Context, a *redis.XAddArgs) *redis.StringCmd
}

// WithStreamConfig overrides the durable stream name and max length.
// Zero/empty values keep the defaults. Returns the publisher for chaining.
func (p *RedisEventPublisher) WithStreamConfig(streamName string, maxLen int64) *RedisEventPublisher {
	if streamName != "" {
		p.streamName = streamName
	}
	if maxLen > 0 {
		p.streamMaxLen = maxLen
	}
	return p
}

// publishDual writes the event durably to the Redis Stream (XADD) and also
// PUBLISHes it on the legacy pub/sub channel. The pub/sub write is kept as a
// dual-write because the WebSocket bridge (internal/ws) relies on pub/sub for
// ephemeral browser updates. The stream is the durable transport consumed by
// agent-service via the "agent-service" consumer group.
func (p *RedisEventPublisher) publishDual(ctx context.Context, channel string, event models.EventEnvelope) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event envelope: %w", err)
	}

	var errs []error

	// Durable write first: XADD with approximate MAXLEN trimming to bound memory.
	if err := p.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: p.streamName,
		MaxLen: p.streamMaxLen,
		Approx: true,
		Values: map[string]interface{}{
			"org_id":       event.OrgID.String(),
			"event_type":   event.EventType,
			"payload":      string(data),
			"published_at": event.Timestamp.UTC().Format(time.RFC3339Nano),
		},
	}).Err(); err != nil {
		errs = append(errs, fmt.Errorf("xadd to stream %s: %w", p.streamName, err))
	}

	// Ephemeral write: keep pub/sub for the WebSocket bridge.
	if err := p.rdb.Publish(ctx, channel, data).Err(); err != nil {
		errs = append(errs, fmt.Errorf("publish to channel %s: %w", channel, err))
	}

	return errors.Join(errs...)
}
