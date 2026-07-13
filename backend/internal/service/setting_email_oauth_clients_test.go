//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type emailOAuthSettingsRepoStub struct {
	values  map[string]string
	updates map[string]string
}

func (s *emailOAuthSettingsRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *emailOAuthSettingsRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	panic("unexpected GetValue call")
}

func (s *emailOAuthSettingsRepoStub) Set(ctx context.Context, key, value string) error {
	panic("unexpected Set call")
}

func (s *emailOAuthSettingsRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *emailOAuthSettingsRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	s.updates = make(map[string]string, len(settings))
	for key, value := range settings {
		s.updates[key] = value
		s.values[key] = value
	}
	return nil
}

func (s *emailOAuthSettingsRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	out := make(map[string]string, len(s.values))
	for key, value := range s.values {
		out[key] = value
	}
	return out, nil
}

func (s *emailOAuthSettingsRepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

func TestEmailOAuthClientsFromSettingsFallsBackToLegacyConfig(t *testing.T) {
	svc := NewSettingService(&emailOAuthSettingsRepoStub{}, &config.Config{
		GitHubOAuth: config.EmailOAuthProviderConfig{
			Enabled:             true,
			ClientID:            "legacy-client",
			ClientSecret:        "legacy-secret",
			RedirectURL:         "https://legacy.example.com/api/v1/auth/oauth/github/callback",
			FrontendRedirectURL: "/auth/oauth/callback",
		},
	})

	clients := svc.emailOAuthClientsFromSettings(map[string]string{})
	require.Len(t, clients, 1)
	require.Equal(t, "github", clients[0].Provider)
	require.Equal(t, "legacy-client", clients[0].ClientID)
	require.Equal(t, "https://legacy.example.com", clients[0].Origin)
}

func TestSettingService_GetPublicSettingsForOriginUsesDatabaseEmailOAuthClients(t *testing.T) {
	repo := &emailOAuthSettingsRepoStub{values: map[string]string{
		SettingKeyEmailOAuthClients: `[{"id":"portal","provider":"github","origin":"https://portal.example.com/","enabled":true,"client_id":"portal-client","client_secret":"portal-secret","redirect_url":"https://portal.example.com/api/v1/auth/oauth/github/callback","frontend_redirect_url":"/auth/oauth/callback"},{"id":"codex","provider":"github","origin":"https://codex.example.com","enabled":true,"client_id":"codex-client","client_secret":"codex-secret","redirect_url":"https://codex.example.com/api/v1/auth/oauth/github/callback","frontend_redirect_url":"/auth/oauth/callback"}]`,
	}}
	svc := NewSettingService(repo, &config.Config{})

	portal, err := svc.GetPublicSettingsForOrigin(context.Background(), "https://portal.example.com")
	require.NoError(t, err)
	require.True(t, portal.GitHubOAuthEnabled)

	codex, err := svc.GetPublicSettingsForOrigin(context.Background(), "https://codex.example.com")
	require.NoError(t, err)
	require.True(t, codex.GitHubOAuthEnabled)

	unknown, err := svc.GetPublicSettingsForOrigin(context.Background(), "https://unknown.example.com")
	require.NoError(t, err)
	require.False(t, unknown.GitHubOAuthEnabled)
	require.True(t, svc.PublicSettingsOriginScopingEnabled())
}

func TestSettingService_UpdateSettings_EmailOAuthKeepsBlankSecretsAndClearsLegacySecrets(t *testing.T) {
	previous := []EmailOAuthClientSetting{{
		ID:                     "portal",
		Provider:               "github",
		Origin:                 "https://portal.example.com",
		Enabled:                true,
		ClientID:               "portal-client",
		ClientSecret:           "portal-secret",
		ClientSecretConfigured: true,
		RedirectURL:            "https://portal.example.com/api/v1/auth/oauth/github/callback",
		FrontendRedirectURL:    "/auth/oauth/callback",
	}}
	previousJSON, err := json.Marshal(previous)
	require.NoError(t, err)
	repo := &emailOAuthSettingsRepoStub{values: map[string]string{
		SettingKeyEmailOAuthClients:              string(previousJSON),
		SettingKeyGitHubOAuthEnabled:             "true",
		SettingKeyGitHubOAuthClientID:            "portal-client",
		SettingKeyGitHubOAuthClientSecret:        "portal-secret",
		SettingKeyGitHubOAuthRedirectURL:         previous[0].RedirectURL,
		SettingKeyGitHubOAuthFrontendRedirectURL: "/auth/oauth/callback",
	}}
	svc := NewSettingService(repo, &config.Config{})

	err = svc.UpdateSettings(context.Background(), &SystemSettings{
		UpdateEmailOAuthClients: true,
		EmailOAuthClients: []EmailOAuthClientSetting{{
			ID:                  "portal",
			Provider:            "github",
			Origin:              "https://portal.example.com",
			Enabled:             true,
			ClientID:            "portal-client",
			RedirectURL:         previous[0].RedirectURL,
			FrontendRedirectURL: "/auth/oauth/callback",
		}},
		GitHubOAuthEnabled:             true,
		GitHubOAuthClientID:            "portal-client",
		GitHubOAuthClientSecret:        "portal-secret",
		GitHubOAuthRedirectURL:         previous[0].RedirectURL,
		GitHubOAuthFrontendRedirectURL: "/auth/oauth/callback",
	})
	require.NoError(t, err)
	require.Contains(t, repo.updates[SettingKeyEmailOAuthClients], "portal-secret")
	require.Equal(t, "portal-secret", repo.updates[SettingKeyGitHubOAuthClientSecret])

	err = svc.UpdateSettings(context.Background(), &SystemSettings{
		UpdateEmailOAuthClients: true,
		EmailOAuthClients:       []EmailOAuthClientSetting{},
	})
	require.NoError(t, err)
	require.Equal(t, "[]", repo.updates[SettingKeyEmailOAuthClients])
	require.Equal(t, "", repo.updates[SettingKeyGitHubOAuthClientSecret])
	require.Equal(t, "", repo.updates[SettingKeyGoogleOAuthClientSecret])
}

func TestSettingService_UpdateSettings_WithoutEmailOAuthUpdateDoesNotReadOAuthSettings(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	require.NoError(t, svc.UpdateSettings(context.Background(), &SystemSettings{}))
	require.NotContains(t, repo.updates, SettingKeyEmailOAuthClients)
}
