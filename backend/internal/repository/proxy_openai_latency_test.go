package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestOpenAIProxyLatencyCacheIsolationAndExpiry(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := &proxyLatencyCache{rdb: client}
	ctx := context.Background()
	generic := &service.ProxyLatencyInfo{Success: true, Message: "generic"}
	require.NoError(t, cache.SetProxyLatency(ctx, 11, generic))
	entry := &service.OpenAIProxyLatencyEntry{Fingerprint: "fingerprint", Result: service.OpenAIProxyLatency{Success: true, Message: "openai", CheckedAt: time.Now().UTC()}}
	require.NoError(t, cache.SetOpenAIProxyLatency(ctx, 11, entry))
	got, err := cache.GetOpenAIProxyLatency(ctx, 11)
	require.NoError(t, err)
	require.Equal(t, entry, got)
	genericResults, err := cache.GetProxyLatencies(ctx, []int64{11})
	require.NoError(t, err)
	require.Equal(t, "generic", genericResults[11].Message)
	server.FastForward(24 * time.Hour)
	got, err = cache.GetOpenAIProxyLatency(ctx, 11)
	require.NoError(t, err)
	require.Nil(t, got)
	require.True(t, server.Exists(proxyLatencyKey(11)))
}
