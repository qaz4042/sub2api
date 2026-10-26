package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const openAIProxyLatencyTarget = "https://api.openai.com/v1/models"

// OpenAIProxyLatency is independent of generic proxy health and exit information.
type OpenAIProxyLatency struct {
	Success    bool      `json:"success"`
	LatencyMs  *int64    `json:"latency_ms,omitempty"`
	HTTPStatus int       `json:"http_status,omitempty"`
	Message    string    `json:"message"`
	CheckedAt  time.Time `json:"checked_at"`
}

type OpenAIProxyLatencyEntry struct {
	Fingerprint string             `json:"fingerprint"`
	Result      OpenAIProxyLatency `json:"result"`
}

// Optional cache capability avoids expanding the upstream latency contract.
type OpenAIProxyLatencyCache interface {
	GetOpenAIProxyLatency(context.Context, int64) (*OpenAIProxyLatencyEntry, error)
	SetOpenAIProxyLatency(context.Context, int64, *OpenAIProxyLatencyEntry) error
}

func openAIProxyFingerprint(proxy *Proxy) string {
	payload, _ := json.Marshal([]any{proxy.Protocol, proxy.Host, proxy.Port, proxy.Username, proxy.Password})
	return fmt.Sprintf("%x", sha256.Sum256(payload))
}

func (s *adminServiceImpl) GetOpenAIProxyLatency(ctx context.Context, id int64) (*OpenAIProxyLatency, error) {
	proxy, err := s.proxyRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	cache, ok := s.proxyLatencyCache.(OpenAIProxyLatencyCache)
	if !ok {
		return nil, nil
	}
	entry, err := cache.GetOpenAIProxyLatency(ctx, id)
	if err != nil {
		return nil, err
	}
	if entry == nil || entry.Fingerprint != openAIProxyFingerprint(proxy) {
		return nil, nil
	}
	return &entry.Result, nil
}

func (s *adminServiceImpl) TestOpenAIProxyLatency(ctx context.Context, id int64) (*OpenAIProxyLatency, error) {
	proxy, err := s.proxyRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	prober, ok := s.proxyProber.(ProxyTargetProber)
	if !ok {
		return nil, fmt.Errorf("OpenAI proxy target probing is unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	status, latency, probeErr := prober.ProbeProxyTarget(ctx, proxy.URL(), openAIProxyLatencyTarget)
	result := &OpenAIProxyLatency{HTTPStatus: status, CheckedAt: time.Now(), Message: "OpenAI connection failed"}
	switch {
	case probeErr != nil:
		// Do not return transport errors, which can contain proxy credentials.
		result.Message = "OpenAI connection failed or timed out"
	case status == http.StatusOK || status == http.StatusUnauthorized:
		result.Success = true
		result.LatencyMs = &latency
		result.Message = "OpenAI API reachable (connectivity only)"
	default:
		result.Message = fmt.Sprintf("OpenAI returned HTTP %d", status)
	}
	if cache, ok := s.proxyLatencyCache.(OpenAIProxyLatencyCache); ok {
		// A canceled probe must still replace an older successful cached result.
		cacheCtx, cacheCancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cacheCancel()
		if err := cache.SetOpenAIProxyLatency(cacheCtx, id, &OpenAIProxyLatencyEntry{Fingerprint: openAIProxyFingerprint(proxy), Result: *result}); err != nil {
			return nil, fmt.Errorf("failed to save OpenAI proxy latency: %w", err)
		}
	}
	return result, nil
}
