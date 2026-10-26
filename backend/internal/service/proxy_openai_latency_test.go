package service

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

type openAILatencyRepoStub struct {
	ProxyRepository
	proxy *Proxy
}

func (r *openAILatencyRepoStub) GetByID(context.Context, int64) (*Proxy, error) { return r.proxy, nil }

type openAILatencyProberStub struct {
	status int
	err    error
	calls  int
}

func (p *openAILatencyProberStub) ProbeProxy(context.Context, string) (*ProxyExitInfo, int64, error) {
	panic("OpenAI test must not request exit/IP services")
}
func (p *openAILatencyProberStub) ProbeProxyTarget(_ context.Context, proxyURL, target string) (int, int64, error) {
	if proxyURL != "http://proxy.example:8080" || target != "https://api.openai.com/v1/models" {
		panic("unexpected probe target")
	}
	p.calls++
	return p.status, 123, p.err
}

type openAILatencyCacheStub struct {
	ProxyLatencyCache
	entry *OpenAIProxyLatencyEntry
}

func (c *openAILatencyCacheStub) GetOpenAIProxyLatency(context.Context, int64) (*OpenAIProxyLatencyEntry, error) {
	return c.entry, nil
}
func (c *openAILatencyCacheStub) SetOpenAIProxyLatency(_ context.Context, _ int64, entry *OpenAIProxyLatencyEntry) error {
	c.entry = entry
	return nil
}

func TestOpenAIProxyLatencyIsolatedAndCached(t *testing.T) {
	for _, status := range []int{200, 401, 301, 403, 407, 429, 500, 502} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			repo := &openAILatencyRepoStub{proxy: &Proxy{ID: 1, Protocol: "http", Host: "proxy.example", Port: 8080}}
			prober := &openAILatencyProberStub{status: status}
			cache := &openAILatencyCacheStub{}
			svc := &adminServiceImpl{proxyRepo: repo, proxyProber: prober, proxyLatencyCache: cache}
			result, err := svc.TestOpenAIProxyLatency(context.Background(), 1)
			require.NoError(t, err)
			require.Equal(t, status == 200 || status == 401, result.Success)
			require.Equal(t, status, result.HTTPStatus)
			if result.Success {
				require.EqualValues(t, 123, *result.LatencyMs)
			} else {
				require.Nil(t, result.LatencyMs)
			}
			loaded, err := svc.GetOpenAIProxyLatency(context.Background(), 1)
			require.NoError(t, err)
			require.Equal(t, result, loaded)
			require.Equal(t, 1, prober.calls, "reading history must not trigger a network probe")
			repo.proxy.Password = "changed-secret"
			loaded, err = svc.GetOpenAIProxyLatency(context.Background(), 1)
			require.NoError(t, err)
			require.Nil(t, loaded, "changed credentials must invalidate history")
		})
	}
}

func TestOpenAIProxyLatencyRedactsErrorsAndReplacesSuccess(t *testing.T) {
	cache := &openAILatencyCacheStub{entry: &OpenAIProxyLatencyEntry{Result: OpenAIProxyLatency{Success: true}}}
	svc := &adminServiceImpl{
		proxyRepo:         &openAILatencyRepoStub{proxy: &Proxy{ID: 1, Protocol: "http", Host: "proxy.example", Port: 8080}},
		proxyProber:       &openAILatencyProberStub{err: errors.New("http://user:secret@proxy.example")},
		proxyLatencyCache: cache,
	}
	result, err := svc.TestOpenAIProxyLatency(context.Background(), 1)
	require.NoError(t, err)
	require.False(t, result.Success)
	require.NotContains(t, result.Message, "secret")
	require.False(t, cache.entry.Result.Success)
}
