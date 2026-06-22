package service

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var platformKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

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

type PlatformConfigInput struct {
	Key         string
	Label       string
	Description string
	Enabled     bool
	SortOrder   int
}

type PlatformConfigUpdate struct {
	Label       *string
	Description *string
	Enabled     *bool
	SortOrder   *int
}

type PlatformConfigRepository interface {
	List(ctx context.Context) ([]PlatformConfig, error)
	Get(ctx context.Context, key string) (*PlatformConfig, error)
	Create(ctx context.Context, input PlatformConfigInput) (*PlatformConfig, error)
	Update(ctx context.Context, key string, input PlatformConfigUpdate) (*PlatformConfig, error)
	Delete(ctx context.Context, key string) error
}

type PlatformConfigService struct {
	repo                PlatformConfigRepository
	platformAccessCache interface{ InvalidatePlatformAccessCache() }
}

func NewPlatformConfigService(repo PlatformConfigRepository, platformAccessCache interface{ InvalidatePlatformAccessCache() }) *PlatformConfigService {
	return &PlatformConfigService{
		repo:                repo,
		platformAccessCache: platformAccessCache,
	}
}

func (s *PlatformConfigService) List(ctx context.Context) ([]PlatformConfig, error) {
	if s == nil || s.repo == nil {
		return DefaultPlatformConfigs(), nil
	}
	return s.repo.List(ctx)
}

func (s *PlatformConfigService) Create(ctx context.Context, input PlatformConfigInput) (*PlatformConfig, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.InternalServer("PLATFORM_CONFIG_REPO_MISSING", "platform config repository is not configured")
	}
	normalized, err := normalizePlatformConfigInput(input)
	if err != nil {
		return nil, err
	}
	item, err := s.repo.Create(ctx, normalized)
	if err == nil {
		s.invalidatePlatformAccessCache()
	}
	return item, err
}

func (s *PlatformConfigService) Update(ctx context.Context, key string, input PlatformConfigUpdate) (*PlatformConfig, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.InternalServer("PLATFORM_CONFIG_REPO_MISSING", "platform config repository is not configured")
	}
	key = normalizePlatformKey(key)
	if !isValidPlatformKey(key) {
		return nil, infraerrors.BadRequest("INVALID_PLATFORM_KEY", "invalid platform key")
	}
	existing, err := s.repo.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if existing.Core && input.Enabled != nil && !*input.Enabled {
		return nil, infraerrors.BadRequest("CORE_PLATFORM_IMMUTABLE", "core platform cannot be disabled")
	}
	if input.Label != nil {
		label := strings.TrimSpace(*input.Label)
		if label == "" {
			return nil, infraerrors.BadRequest("INVALID_PLATFORM_LABEL", "platform label is required")
		}
		input.Label = &label
	}
	if input.Description != nil {
		description := strings.TrimSpace(*input.Description)
		input.Description = &description
	}
	item, err := s.repo.Update(ctx, key, input)
	if err == nil {
		s.invalidatePlatformAccessCache()
	}
	return item, err
}

func (s *PlatformConfigService) Delete(ctx context.Context, key string) error {
	if s == nil || s.repo == nil {
		return infraerrors.InternalServer("PLATFORM_CONFIG_REPO_MISSING", "platform config repository is not configured")
	}
	key = normalizePlatformKey(key)
	if !isValidPlatformKey(key) {
		return infraerrors.BadRequest("INVALID_PLATFORM_KEY", "invalid platform key")
	}
	existing, err := s.repo.Get(ctx, key)
	if err != nil {
		return err
	}
	if existing.Core {
		return infraerrors.BadRequest("CORE_PLATFORM_IMMUTABLE", "core platform cannot be deleted")
	}
	if err := s.repo.Delete(ctx, key); err != nil {
		return err
	}
	s.invalidatePlatformAccessCache()
	return nil
}

func (s *PlatformConfigService) invalidatePlatformAccessCache() {
	if s != nil && s.platformAccessCache != nil {
		s.platformAccessCache.InvalidatePlatformAccessCache()
	}
}

func normalizePlatformConfigInput(input PlatformConfigInput) (PlatformConfigInput, error) {
	input.Key = normalizePlatformKey(input.Key)
	input.Label = strings.TrimSpace(input.Label)
	input.Description = strings.TrimSpace(input.Description)
	if !isValidPlatformKey(input.Key) {
		return input, infraerrors.BadRequest("INVALID_PLATFORM_KEY", "invalid platform key")
	}
	if input.Label == "" {
		return input, infraerrors.BadRequest("INVALID_PLATFORM_LABEL", "platform label is required")
	}
	return input, nil
}

func normalizePlatformKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}

func isValidPlatformKey(key string) bool {
	return platformKeyPattern.MatchString(key)
}

func DefaultPlatformConfigs() []PlatformConfig {
	now := time.Time{}
	items := []PlatformConfig{
		{Key: PlatformOpenAI, Label: "OpenAI", Description: "Core OpenAI-compatible interface. Always enabled.", Enabled: true, Core: true, SortOrder: 10, CreatedAt: now, UpdatedAt: now},
		{Key: PlatformAnthropic, Label: "Anthropic / Claude", Description: "Claude-compatible interface.", Enabled: false, SortOrder: 20, CreatedAt: now, UpdatedAt: now},
		{Key: PlatformGemini, Label: "Gemini", Description: "Google Gemini interface.", Enabled: false, SortOrder: 30, CreatedAt: now, UpdatedAt: now},
		{Key: PlatformAntigravity, Label: "Antigravity", Description: "Antigravity interface.", Enabled: false, SortOrder: 40, CreatedAt: now, UpdatedAt: now},
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].SortOrder == items[j].SortOrder {
			return items[i].Key < items[j].Key
		}
		return items[i].SortOrder < items[j].SortOrder
	})
	return items
}
