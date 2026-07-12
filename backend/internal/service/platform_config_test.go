//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type platformConfigServiceRepoStub struct {
	items        []PlatformConfig
	setKey       string
	setEnabled   bool
	setCallCount int
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

func (s *platformConfigServiceRepoStub) SetEnabled(_ context.Context, key string, enabled bool) (*PlatformConfig, error) {
	s.setKey = key
	s.setEnabled = enabled
	s.setCallCount++
	item, err := s.Get(context.Background(), key)
	if err != nil {
		return nil, err
	}
	item.Enabled = enabled
	return item, nil
}

type platformAccessInvalidatorStub struct{ calls int }

func (s *platformAccessInvalidatorStub) InvalidatePlatformAccessCache() { s.calls++ }

func TestPlatformConfigServiceSetEnabledProtectsCorePlatform(t *testing.T) {
	repo := &platformConfigServiceRepoStub{items: []PlatformConfig{{Key: PlatformOpenAI, Enabled: true, Core: true}}}
	invalidator := &platformAccessInvalidatorStub{}
	svc := NewPlatformConfigService(repo, invalidator)

	_, err := svc.SetEnabled(context.Background(), " OPENAI ", false)

	require.Error(t, err)
	require.Equal(t, 0, repo.setCallCount)
	require.Equal(t, 0, invalidator.calls)
}

func TestPlatformConfigServiceSetEnabledInvalidatesRuntimeCache(t *testing.T) {
	repo := &platformConfigServiceRepoStub{items: []PlatformConfig{{Key: PlatformAnthropic, Enabled: true}}}
	invalidator := &platformAccessInvalidatorStub{}
	svc := NewPlatformConfigService(repo, invalidator)

	item, err := svc.SetEnabled(context.Background(), " ANTHROPIC ", false)

	require.NoError(t, err)
	require.False(t, item.Enabled)
	require.Equal(t, PlatformAnthropic, repo.setKey)
	require.False(t, repo.setEnabled)
	require.Equal(t, 1, invalidator.calls)
}
