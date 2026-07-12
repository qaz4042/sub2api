package service

import (
	"context"
	"sort"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var ErrPlatformConfigNotFound = infraerrors.NotFound("PLATFORM_CONFIG_NOT_FOUND", "platform config not found")

type PlatformConfig struct {
	Key         string    `json:"key"`
	Label       string    `json:"label"`
	Description string    `json:"description"`
	Enabled     bool      `json:"enabled"`
	Core        bool      `json:"core"`
	SortOrder   int       `json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type PlatformConfigRepository interface {
	List(ctx context.Context) ([]PlatformConfig, error)
	Get(ctx context.Context, key string) (*PlatformConfig, error)
	SetEnabled(ctx context.Context, key string, enabled bool) (*PlatformConfig, error)
}

type PlatformConfigService struct {
	repo                PlatformConfigRepository
	platformAccessCache interface{ InvalidatePlatformAccessCache() }
}

func NewPlatformConfigService(repo PlatformConfigRepository, platformAccessCache interface{ InvalidatePlatformAccessCache() }) *PlatformConfigService {
	return &PlatformConfigService{repo: repo, platformAccessCache: platformAccessCache}
}

func (s *PlatformConfigService) List(ctx context.Context) ([]PlatformConfig, error) {
	if s == nil || s.repo == nil {
		return DefaultPlatformConfigs(), nil
	}
	return s.repo.List(ctx)
}

func (s *PlatformConfigService) SetEnabled(ctx context.Context, key string, enabled bool) (*PlatformConfig, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.InternalServer("PLATFORM_CONFIG_REPO_MISSING", "platform config repository is not configured")
	}
	key = normalizePlatformKey(key)
	existing, err := s.repo.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if existing.Core && !enabled {
		return nil, infraerrors.BadRequest("CORE_PLATFORM_IMMUTABLE", "core platform cannot be disabled")
	}
	item, err := s.repo.SetEnabled(ctx, key, enabled)
	if err == nil {
		s.invalidatePlatformAccessCache()
	}
	return item, err
}

func (s *PlatformConfigService) invalidatePlatformAccessCache() {
	if s != nil && s.platformAccessCache != nil {
		s.platformAccessCache.InvalidatePlatformAccessCache()
	}
}

func normalizePlatformKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}

func DefaultPlatformConfigs() []PlatformConfig {
	now := time.Time{}
	items := []PlatformConfig{
		{Key: PlatformOpenAI, Label: "OpenAI", Description: "OpenAI-compatible interface.", Enabled: true, Core: true, SortOrder: 10, CreatedAt: now, UpdatedAt: now},
		{Key: PlatformAnthropic, Label: "Anthropic / Claude", Description: "Claude-compatible interface.", Enabled: true, SortOrder: 20, CreatedAt: now, UpdatedAt: now},
		{Key: PlatformGemini, Label: "Gemini", Description: "Google Gemini interface.", Enabled: true, SortOrder: 30, CreatedAt: now, UpdatedAt: now},
		{Key: PlatformAntigravity, Label: "Antigravity", Description: "Antigravity interface.", Enabled: true, SortOrder: 40, CreatedAt: now, UpdatedAt: now},
		{Key: PlatformGrok, Label: "Grok", Description: "Grok-compatible interface.", Enabled: true, SortOrder: 50, CreatedAt: now, UpdatedAt: now},
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].SortOrder < items[j].SortOrder
	})
	return items
}
