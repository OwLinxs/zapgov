package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

const channel = "zapgov:events"

type broadcast struct {
	Origin   string `json:"origin"`
	TenantID int64  `json:"tenant_id"`
	Event    Event  `json:"event"`
}

// ConnectRedis shares invalidation events across API replicas and the worker.
// PostgreSQL remains authoritative; the frontend polling recovers missed events.
func (h *Hub) ConnectRedis(ctx context.Context, addr string) func() {
	client := redis.NewClient(&redis.Options{Addr: addr})
	h.origin = fmt.Sprintf("%d", time.Now().UnixNano())
	h.broadcast = func(e Event) {
		raw, err := json.Marshal(broadcast{Origin: h.origin, TenantID: e.TenantID, Event: e})
		if err != nil {
			return
		}
		op, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := client.Publish(op, channel, raw).Err(); err != nil {
			slog.Warn("SSE: Redis indisponível", "err", err)
		}
	}
	sub := client.Subscribe(ctx, channel)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-sub.Channel():
				if !ok {
					return
				}
				var b broadcast
				if json.Unmarshal([]byte(msg.Payload), &b) != nil || b.Origin == h.origin {
					continue
				}
				b.Event.TenantID = b.TenantID
				h.publishLocal(b.Event)
			}
		}
	}()
	return func() { sub.Close(); client.Close() }
}
