//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestGetEmailOAuthProviderConfig_UsesDomainScopedClients(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingKeyEmailOAuthClients: `[
				{
					"id":"portal-github",
					"provider":"github",
					"name":"Portal GitHub",
					"origin":"https://portal.example.com",
					"enabled":true,
					"client_id":"portal-client",
					"client_secret":"portal-secret",
					"redirect_url":"https://portal.example.com/api/v1/auth/oauth/github/callback",
					"frontend_redirect_url":"/auth/oauth/callback",
					"sort_order":0
				},
				{
					"id":"codex-github",
					"provider":"github",
					"name":"Codex GitHub",
					"origin":"https://codex.example.com",
					"enabled":true,
					"client_id":"codex-client",
					"client_secret":"codex-secret",
					"redirect_url":"https://codex.example.com/api/v1/auth/oauth/github/callback",
					"frontend_redirect_url":"/auth/oauth/callback",
					"sort_order":1
				}
			]`,
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	got, err := svc.GetEmailOAuthProviderConfig(context.Background(), "github")
	require.NoError(t, err)
	require.True(t, got.Enabled)
	require.Equal(t, "portal-client", got.ClientID)
	require.Equal(t, []string{"https://portal.example.com", "https://codex.example.com"}, got.AllowedRedirectOrigins)
	require.Len(t, got.OriginOverrides, 2)
	require.Equal(t, "codex-client", got.OriginOverrides[1].ClientID)
	require.Equal(t, "https://codex.example.com/api/v1/auth/oauth/github/callback", got.OriginOverrides[1].RedirectURL)
}

func TestParseSettings_EmailOAuthClientsFallbackToLegacySettings(t *testing.T) {
	svc := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, &config.Config{})

	got := svc.parseSettings(map[string]string{
		SettingKeyGitHubOAuthEnabled:             "true",
		SettingKeyGitHubOAuthClientID:            "legacy-client",
		SettingKeyGitHubOAuthClientSecret:        "legacy-secret",
		SettingKeyGitHubOAuthRedirectURL:         "https://portal.example.com/api/v1/auth/oauth/github/callback",
		SettingKeyGitHubOAuthFrontendRedirectURL: "/auth/oauth/callback",
	})

	require.Len(t, got.EmailOAuthClients, 1)
	require.Equal(t, "github", got.EmailOAuthClients[0].Provider)
	require.Equal(t, "https://portal.example.com", got.EmailOAuthClients[0].Origin)
	require.True(t, got.EmailOAuthClients[0].ClientSecretConfigured)
}
