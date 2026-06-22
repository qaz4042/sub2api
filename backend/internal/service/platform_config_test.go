package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type platformConfigServiceRepoStub struct {
	items []PlatformConfig
}

func (s *platformConfigServiceRepoStub) List(context.Context) ([]PlatformConfig, error) {
	return s.items, nil
}

func (s *platformConfigServiceRepoStub) Get(_ context.Context, key string) (*PlatformConfig, error) {
	for i := range s.items {
		if s.items[i].Key == key {
			item := s.items[i]
			return &item, nil
		}
	}
	return nil, ErrPlatformConfigNotFound
}

func (s *platformConfigServiceRepoStub) Create(_ context.Context, input PlatformConfigInput) (*PlatformConfig, error) {
	item := PlatformConfig{
		Key:         input.Key,
		Label:       input.Label,
		Description: input.Description,
		Enabled:     input.Enabled,
		SortOrder:   input.SortOrder,
	}
	s.items = append(s.items, item)
	return &item, nil
}

func (s *platformConfigServiceRepoStub) Update(_ context.Context, key string, input PlatformConfigUpdate) (*PlatformConfig, error) {
	for i := range s.items {
		if s.items[i].Key != key {
			continue
		}
		if input.Label != nil {
			s.items[i].Label = *input.Label
		}
		if input.Description != nil {
			s.items[i].Description = *input.Description
		}
		if input.Enabled != nil {
			s.items[i].Enabled = *input.Enabled
		}
		if input.SortOrder != nil {
			s.items[i].SortOrder = *input.SortOrder
		}
		item := s.items[i]
		return &item, nil
	}
	return nil, ErrPlatformConfigNotFound
}

func (s *platformConfigServiceRepoStub) Delete(_ context.Context, key string) error {
	for i := range s.items {
		if s.items[i].Key == key {
			s.items = append(s.items[:i], s.items[i+1:]...)
			return nil
		}
	}
	return ErrPlatformConfigNotFound
}

type platformAccessCacheStub struct {
	invalidations int
}

func (s *platformAccessCacheStub) InvalidatePlatformAccessCache() {
	s.invalidations++
}

func TestPlatformConfigServiceCRUDInvalidatesAccessCache(t *testing.T) {
	repo := &platformConfigServiceRepoStub{}
	cache := &platformAccessCacheStub{}
	svc := NewPlatformConfigService(repo, cache)
	ctx := context.Background()

	created, err := svc.Create(ctx, PlatformConfigInput{
		Key:         " Custom_Platform ",
		Label:       " Custom ",
		Description: " Test ",
		Enabled:     true,
		SortOrder:   50,
	})
	require.NoError(t, err)
	require.Equal(t, "custom_platform", created.Key)
	require.Equal(t, "Custom", created.Label)
	require.False(t, created.Core)

	enabled := false
	updated, err := svc.Update(ctx, created.Key, PlatformConfigUpdate{Enabled: &enabled})
	require.NoError(t, err)
	require.False(t, updated.Enabled)

	require.NoError(t, svc.Delete(ctx, created.Key))
	require.Equal(t, 3, cache.invalidations)
}

func TestPlatformConfigServiceProtectsCorePlatform(t *testing.T) {
	repo := &platformConfigServiceRepoStub{items: []PlatformConfig{
		{Key: PlatformOpenAI, Label: "OpenAI", Enabled: true, Core: true},
	}}
	svc := NewPlatformConfigService(repo, &platformAccessCacheStub{})
	ctx := context.Background()
	disabled := false

	_, err := svc.Update(ctx, PlatformOpenAI, PlatformConfigUpdate{Enabled: &disabled})
	require.Error(t, err)
	require.Error(t, svc.Delete(ctx, PlatformOpenAI))
}
