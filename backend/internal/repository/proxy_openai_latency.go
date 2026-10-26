package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

func openAIProxyLatencyKey(id int64) string { return fmt.Sprintf("proxy:openai-latency:%d", id) }

func (c *proxyLatencyCache) GetOpenAIProxyLatency(ctx context.Context, id int64) (*service.OpenAIProxyLatencyEntry, error) {
	payload, err := c.rdb.Get(ctx, openAIProxyLatencyKey(id)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entry service.OpenAIProxyLatencyEntry
	if err := json.Unmarshal(payload, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

func (c *proxyLatencyCache) SetOpenAIProxyLatency(ctx context.Context, id int64, entry *service.OpenAIProxyLatencyEntry) error {
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, openAIProxyLatencyKey(id), payload, 24*time.Hour).Err()
}
