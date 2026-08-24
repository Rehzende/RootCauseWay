package database

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeEventRedis captures Publish/XAdd calls for assertions.
type fakeEventRedis struct {
	publishChannels []string
	publishPayloads [][]byte
	xaddArgs        []*redis.XAddArgs

	publishErr error
	xaddErr    error
}

func (f *fakeEventRedis) Publish(ctx context.Context, channel string, message interface{}) *redis.IntCmd {
	f.publishChannels = append(f.publishChannels, channel)
	if b, ok := message.([]byte); ok {
		f.publishPayloads = append(f.publishPayloads, b)
	}
	cmd := redis.NewIntCmd(ctx)
	if f.publishErr != nil {
		cmd.SetErr(f.publishErr)
	} else {
		cmd.SetVal(1)
	}
	return cmd
}

func (f *fakeEventRedis) XAdd(ctx context.Context, a *redis.XAddArgs) *redis.StringCmd {
	f.xaddArgs = append(f.xaddArgs, a)
	cmd := redis.NewStringCmd(ctx)
	if f.xaddErr != nil {
		cmd.SetErr(f.xaddErr)
	} else {
		cmd.SetVal("1-0")
	}
	return cmd
}

func newTestPublisher(fake *fakeEventRedis) *RedisEventPublisher {
	return &RedisEventPublisher{
		rdb:          fake,
		streamName:   DefaultEventStream,
		streamMaxLen: DefaultEventStreamMaxLen,
	}
}

func testEnvelope() models.EventEnvelope {
	return models.EventEnvelope{
		EventID:   uuid.New(),
		EventType: "alert.received",
		OrgID:     uuid.New(),
		Timestamp: time.Now().UTC(),
		Payload:   map[string]string{"incident_id": uuid.NewString()},
	}
}

func TestRedisEventPublisher_DualWritesStreamAndPubSub(t *testing.T) {
	fake := &fakeEventRedis{}
	p := newTestPublisher(fake)
	event := testEnvelope()
	channel := "rootcauseway:" + event.OrgID.String() + ":alert.received"

	err := p.Publish(context.Background(), channel, event)
	require.NoError(t, err)

	// Pub/sub write kept for the WebSocket bridge.
	require.Len(t, fake.publishChannels, 1)
	assert.Equal(t, channel, fake.publishChannels[0])

	// Durable stream write.
	require.Len(t, fake.xaddArgs, 1)
	args := fake.xaddArgs[0]
	assert.Equal(t, DefaultEventStream, args.Stream)
	assert.Equal(t, DefaultEventStreamMaxLen, args.MaxLen)
	assert.True(t, args.Approx, "MAXLEN trimming must be approximate (~)")

	values, ok := args.Values.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, event.OrgID.String(), values["org_id"])
	assert.Equal(t, "alert.received", values["event_type"])
	assert.NotEmpty(t, values["published_at"])

	// The payload field carries the full event envelope JSON (identical to
	// what is published on pub/sub) so consumers can swap transports.
	var decoded models.EventEnvelope
	require.NoError(t, json.Unmarshal([]byte(values["payload"].(string)), &decoded))
	assert.Equal(t, event.EventID, decoded.EventID)
	assert.Equal(t, event.EventType, decoded.EventType)
	assert.Equal(t, event.OrgID, decoded.OrgID)
	require.Len(t, fake.publishPayloads, 1)
	assert.JSONEq(t, string(fake.publishPayloads[0]), values["payload"].(string))
}

func TestRedisEventPublisher_WithStreamConfig(t *testing.T) {
	fake := &fakeEventRedis{}
	p := newTestPublisher(fake).WithStreamConfig("custom:stream", 42)

	err := p.Publish(context.Background(), "chan", testEnvelope())
	require.NoError(t, err)

	require.Len(t, fake.xaddArgs, 1)
	assert.Equal(t, "custom:stream", fake.xaddArgs[0].Stream)
	assert.Equal(t, int64(42), fake.xaddArgs[0].MaxLen)
}

func TestRedisEventPublisher_WithStreamConfig_KeepsDefaultsForZeroValues(t *testing.T) {
	p := newTestPublisher(&fakeEventRedis{}).WithStreamConfig("", 0)
	assert.Equal(t, DefaultEventStream, p.streamName)
	assert.Equal(t, DefaultEventStreamMaxLen, p.streamMaxLen)
}

func TestRedisEventPublisher_XAddErrorStillPublishesAndReturnsError(t *testing.T) {
	fake := &fakeEventRedis{xaddErr: errors.New("stream down")}
	p := newTestPublisher(fake)

	err := p.Publish(context.Background(), "chan", testEnvelope())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stream down")
	// Pub/sub write must still be attempted so the WS bridge keeps working.
	assert.Len(t, fake.publishChannels, 1)
}

func TestRedisEventPublisher_PublishErrorStillXAddsAndReturnsError(t *testing.T) {
	fake := &fakeEventRedis{publishErr: errors.New("pubsub down")}
	p := newTestPublisher(fake)

	err := p.Publish(context.Background(), "chan", testEnvelope())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pubsub down")
	// Durable write must still happen.
	assert.Len(t, fake.xaddArgs, 1)
}
