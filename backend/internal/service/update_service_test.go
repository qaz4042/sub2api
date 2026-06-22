//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type updateServiceCacheStub struct {
	data string
}

func (s *updateServiceCacheStub) GetUpdateInfo(context.Context) (string, error) {
	if s.data == "" {
		return "", errors.New("cache miss")
	}
	return s.data, nil
}

func (s *updateServiceCacheStub) SetUpdateInfo(_ context.Context, data string, _ time.Duration) error {
	s.data = data
	return nil
}

type updateServiceGitHubClientStub struct {
	release *GitHubRelease
}

func (s *updateServiceGitHubClientStub) FetchLatestRelease(context.Context, string) (*GitHubRelease, error) {
	return s.release, nil
}

func (s *updateServiceGitHubClientStub) DownloadFile(context.Context, string, string, int64) error {
	panic("DownloadFile should not be called when no update is available")
}

func (s *updateServiceGitHubClientStub) FetchChecksumFile(context.Context, string) ([]byte, error) {
	panic("FetchChecksumFile should not be called when no update is available")
}

func TestUpdateServicePerformUpdateNoUpdateReturnsSentinel(t *testing.T) {
	svc := NewUpdateService(
		&updateServiceCacheStub{},
		&updateServiceGitHubClientStub{
			release: &GitHubRelease{
				TagName: "v0.1.132",
				Name:    "v0.1.132",
			},
		},
		"0.1.132",
		"release",
	)

	err := svc.PerformUpdate(context.Background())

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNoUpdateAvailable))
	require.ErrorIs(t, err, ErrNoUpdateAvailable)
}

func TestUpdateServiceUnversionedBuildSkipsUpstreamUpdate(t *testing.T) {
	svc := NewUpdateService(
		&updateServiceCacheStub{},
		&updateServiceGitHubClientStub{},
		"0.0.0-my2g.20260618.1",
		"release",
	)

	info, err := svc.CheckUpdate(context.Background(), true)

	require.NoError(t, err)
	require.Equal(t, "0.0.0-my2g.20260618.1", info.CurrentVersion)
	require.Equal(t, info.CurrentVersion, info.LatestVersion)
	require.False(t, info.HasUpdate)
	require.Nil(t, info.ReleaseInfo)
}

func TestIsUnversionedBuild(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{version: "0.0.0-my2g.20260618.1", want: true},
		{version: "v0.0.0+custom.1", want: true},
		{version: "0.0.0", want: false},
		{version: "0.1.138-rc.1", want: false},
		{version: "0.1.138", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			require.Equal(t, tt.want, isUnversionedBuild(tt.version))
		})
	}
}
