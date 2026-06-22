package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type platformAccessRepoStub struct {
	values map[string]string
}

func (s *platformAccessRepoStub) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}
func (s *platformAccessRepoStub) GetValue(context.Context, string) (string, error) {
	return "", ErrSettingNotFound
}
func (s *platformAccessRepoStub) Set(context.Context, string, string) error            { return nil }
func (s *platformAccessRepoStub) SetMultiple(context.Context, map[string]string) error { return nil }
func (s *platformAccessRepoStub) GetAll(context.Context) (map[string]string, error) {
	return s.values, nil
}
func (s *platformAccessRepoStub) Delete(context.Context, string) error { return nil }
func (s *platformAccessRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			result[key] = value
		}
	}
	return result, nil
}

type platformConfigRepoStub struct {
	items []PlatformConfig
}

func (s *platformConfigRepoStub) List(context.Context) ([]PlatformConfig, error) {
	return s.items, nil
}

func (s *platformConfigRepoStub) Get(_ context.Context, key string) (*PlatformConfig, error) {
	for i := range s.items {
		if s.items[i].Key == key {
			item := s.items[i]
			return &item, nil
		}
	}
	return nil, ErrPlatformConfigNotFound
}

func (s *platformConfigRepoStub) Create(context.Context, PlatformConfigInput) (*PlatformConfig, error) {
	return nil, nil
}

func (s *platformConfigRepoStub) Update(context.Context, string, PlatformConfigUpdate) (*PlatformConfig, error) {
	return nil, nil
}

func (s *platformConfigRepoStub) Delete(context.Context, string) error {
	return nil
}

func TestSettingServicePlatformAccessDefaultsToOpenAIOnly(t *testing.T) {
	svc := NewSettingService(&platformAccessRepoStub{values: map[string]string{}}, nil)
	ctx := context.Background()

	require.True(t, svc.IsPlatformEnabled(ctx, PlatformOpenAI))
	require.False(t, svc.IsPlatformEnabled(ctx, PlatformAnthropic))
	require.False(t, svc.IsPlatformEnabled(ctx, PlatformGemini))
	require.False(t, svc.IsPlatformEnabled(ctx, PlatformAntigravity))
}

func TestSettingServicePlatformAccessHonorsEnabledSwitches(t *testing.T) {
	svc := NewSettingService(&platformAccessRepoStub{values: map[string]string{}}, nil)
	svc.SetPlatformConfigRepository(&platformConfigRepoStub{items: []PlatformConfig{
		{Key: PlatformOpenAI, Enabled: true, Core: true},
		{Key: PlatformAnthropic, Enabled: true},
		{Key: PlatformGemini, Enabled: true},
		{Key: PlatformAntigravity, Enabled: true},
	}})
	ctx := context.Background()

	require.True(t, svc.IsPlatformEnabled(ctx, PlatformAnthropic))
	require.True(t, svc.IsPlatformEnabled(ctx, PlatformGemini))
	require.True(t, svc.IsPlatformEnabled(ctx, PlatformAntigravity))
}
