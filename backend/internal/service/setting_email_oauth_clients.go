package service

import (
	"encoding/json"
	"log/slog"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

func normalizeEmailOAuthProvider(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "github":
		return "github"
	case "google":
		return "google"
	default:
		return ""
	}
}

func emailOAuthClientOrigin(raw string) string {
	origin := strings.TrimSpace(raw)
	if normalized := normalizeEmailOAuthOrigin(origin); normalized != "" {
		return normalized
	}
	return strings.TrimRight(origin, "/")
}

func parseEmailOAuthClients(raw string) []EmailOAuthClientSetting {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var items []EmailOAuthClientSetting
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		slog.Warn("parse email_oauth_clients failed", "error", err)
		return nil
	}
	return normalizeEmailOAuthClients(items, false)
}

// normalizeEmailOAuthClients 清理非法 provider、重复 Origin，并按 sort_order 稳定排序。
// keepEmptySecret 仅用于保存路径，允许调用方先保留空 Secret 再与旧配置合并。
func normalizeEmailOAuthClients(items []EmailOAuthClientSetting, keepEmptySecret bool) []EmailOAuthClientSetting {
	if len(items) == 0 {
		return nil
	}
	normalized := make([]EmailOAuthClientSetting, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		provider := normalizeEmailOAuthProvider(item.Provider)
		origin := emailOAuthClientOrigin(item.Origin)
		if provider == "" && origin == "" && strings.TrimSpace(item.ClientID) == "" && strings.TrimSpace(item.RedirectURL) == "" {
			continue
		}
		if provider == "" {
			continue
		}
		key := provider + "|" + origin
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		secret := strings.TrimSpace(item.ClientSecret)
		if !keepEmptySecret && secret == "" && item.ClientSecretConfigured {
			secret = ""
		}
		normalized = append(normalized, EmailOAuthClientSetting{
			ID:                     strings.TrimSpace(item.ID),
			Provider:               provider,
			Name:                   strings.TrimSpace(item.Name),
			Origin:                 origin,
			Enabled:                item.Enabled,
			ClientID:               strings.TrimSpace(item.ClientID),
			ClientSecret:           secret,
			ClientSecretConfigured: item.ClientSecretConfigured || secret != "",
			RedirectURL:            strings.TrimSpace(item.RedirectURL),
			FrontendRedirectURL:    strings.TrimSpace(item.FrontendRedirectURL),
			SortOrder:              item.SortOrder,
		})
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		if normalized[i].SortOrder == normalized[j].SortOrder {
			if normalized[i].Provider == normalized[j].Provider {
				return normalized[i].Origin < normalized[j].Origin
			}
			return normalized[i].Provider < normalized[j].Provider
		}
		return normalized[i].SortOrder < normalized[j].SortOrder
	})
	return normalized
}

func emailOAuthClientsForProvider(items []EmailOAuthClientSetting, provider string) []EmailOAuthClientSetting {
	provider = normalizeEmailOAuthProvider(provider)
	if provider == "" || len(items) == 0 {
		return nil
	}
	out := make([]EmailOAuthClientSetting, 0, len(items))
	for _, item := range items {
		if normalizeEmailOAuthProvider(item.Provider) == provider {
			out = append(out, item)
		}
	}
	return out
}

func legacyEmailOAuthClient(provider string, cfg config.EmailOAuthProviderConfig) (EmailOAuthClientSetting, bool) {
	provider = normalizeEmailOAuthProvider(provider)
	if provider == "" {
		return EmailOAuthClientSetting{}, false
	}
	origin := ""
	if cfg.RedirectURL != "" {
		if normalized := normalizeEmailOAuthOrigin(cfg.RedirectURL); normalized != "" {
			origin = normalized
		}
	}
	if origin == "" && len(cfg.AllowedRedirectOrigins) > 0 {
		origin = emailOAuthClientOrigin(cfg.AllowedRedirectOrigins[0])
	}
	if strings.TrimSpace(cfg.ClientID) == "" && strings.TrimSpace(cfg.ClientSecret) == "" && strings.TrimSpace(cfg.RedirectURL) == "" {
		return EmailOAuthClientSetting{}, false
	}
	name := "GitHub"
	if provider == "google" {
		name = "Google"
	}
	return EmailOAuthClientSetting{
		ID:                     provider + "-default",
		Provider:               provider,
		Name:                   name,
		Origin:                 origin,
		Enabled:                cfg.Enabled,
		ClientID:               strings.TrimSpace(cfg.ClientID),
		ClientSecret:           strings.TrimSpace(cfg.ClientSecret),
		ClientSecretConfigured: strings.TrimSpace(cfg.ClientSecret) != "",
		RedirectURL:            strings.TrimSpace(cfg.RedirectURL),
		FrontendRedirectURL:    strings.TrimSpace(cfg.FrontendRedirectURL),
	}, true
}

func (s *SettingService) emailOAuthClientsFromSettings(settings map[string]string) []EmailOAuthClientSetting {
	clients := parseEmailOAuthClients(settings[SettingKeyEmailOAuthClients])
	if len(clients) > 0 {
		return clients
	}
	// 数据库尚未迁移到列表格式时，将旧的单客户端字段虚拟成列表，保证后台可平滑编辑。
	out := make([]EmailOAuthClientSetting, 0, 2)
	if item, ok := legacyEmailOAuthClient("github", s.effectiveEmailOAuthConfigWithoutClients(settings, "github")); ok {
		out = append(out, item)
	}
	if item, ok := legacyEmailOAuthClient("google", s.effectiveEmailOAuthConfigWithoutClients(settings, "google")); ok {
		out = append(out, item)
	}
	return normalizeEmailOAuthClients(out, true)
}

func (s *SettingService) effectiveEmailOAuthConfigWithoutClients(settings map[string]string, provider string) config.EmailOAuthProviderConfig {
	cfg := s.emailOAuthBaseConfig(provider)
	switch normalizeEmailOAuthProvider(provider) {
	case "github":
		if raw, ok := settings[SettingKeyGitHubOAuthEnabled]; ok {
			cfg.Enabled = raw == "true"
		}
		cfg.ClientID = firstNonEmpty(settings[SettingKeyGitHubOAuthClientID], cfg.ClientID)
		cfg.ClientSecret = firstNonEmpty(settings[SettingKeyGitHubOAuthClientSecret], cfg.ClientSecret)
		cfg.RedirectURL = firstNonEmpty(settings[SettingKeyGitHubOAuthRedirectURL], cfg.RedirectURL)
		cfg.FrontendRedirectURL = firstNonEmpty(settings[SettingKeyGitHubOAuthFrontendRedirectURL], cfg.FrontendRedirectURL, defaultGitHubOAuthFrontend)
	case "google":
		if raw, ok := settings[SettingKeyGoogleOAuthEnabled]; ok {
			cfg.Enabled = raw == "true"
		}
		cfg.ClientID = firstNonEmpty(settings[SettingKeyGoogleOAuthClientID], cfg.ClientID)
		cfg.ClientSecret = firstNonEmpty(settings[SettingKeyGoogleOAuthClientSecret], cfg.ClientSecret)
		cfg.RedirectURL = firstNonEmpty(settings[SettingKeyGoogleOAuthRedirectURL], cfg.RedirectURL)
		cfg.FrontendRedirectURL = firstNonEmpty(settings[SettingKeyGoogleOAuthFrontendRedirectURL], cfg.FrontendRedirectURL, defaultGoogleOAuthFrontend)
	}
	return cfg
}

// MergeEmailOAuthClientSecrets 恢复前端脱敏后留空的 Secret；未匹配到旧客户端时保持为空，
// 由 handler 的校验逻辑阻止启用配置在缺少 Secret 时保存。
func MergeEmailOAuthClientSecrets(next, previous []EmailOAuthClientSetting) []EmailOAuthClientSetting {
	if len(next) == 0 {
		return []EmailOAuthClientSetting{}
	}
	previousByKey := make(map[string]EmailOAuthClientSetting, len(previous))
	for _, item := range previous {
		key := normalizeEmailOAuthProvider(item.Provider) + "|" + emailOAuthClientOrigin(item.Origin) + "|" + strings.TrimSpace(item.ClientID)
		previousByKey[key] = item
	}
	for i := range next {
		if strings.TrimSpace(next[i].ClientSecret) != "" {
			next[i].ClientSecretConfigured = true
			continue
		}
		key := normalizeEmailOAuthProvider(next[i].Provider) + "|" + emailOAuthClientOrigin(next[i].Origin) + "|" + strings.TrimSpace(next[i].ClientID)
		if previousItem, ok := previousByKey[key]; ok && strings.TrimSpace(previousItem.ClientSecret) != "" {
			next[i].ClientSecret = strings.TrimSpace(previousItem.ClientSecret)
			next[i].ClientSecretConfigured = true
		}
	}
	return next
}

func applyEmailOAuthClientsToConfig(cfg config.EmailOAuthProviderConfig, clients []EmailOAuthClientSetting, provider string) config.EmailOAuthProviderConfig {
	providerClients := emailOAuthClientsForProvider(clients, provider)
	if len(providerClients) == 0 {
		return cfg
	}
	// 列表存在时以列表为准，并将启用项转换为现有配置结构的 Origin 白名单和覆盖项。
	cfg.Enabled = false
	origins := make([]string, 0, len(providerClients))
	overrides := make([]config.EmailOAuthOriginOverride, 0, len(providerClients))
	defaultApplied := false
	for _, item := range providerClients {
		if !item.Enabled {
			continue
		}
		origin := emailOAuthClientOrigin(item.Origin)
		if origin != "" {
			origins = append(origins, origin)
		}
		if !defaultApplied {
			cfg.Enabled = true
			cfg.ClientID = item.ClientID
			cfg.ClientSecret = item.ClientSecret
			cfg.RedirectURL = item.RedirectURL
			cfg.FrontendRedirectURL = firstNonEmpty(item.FrontendRedirectURL, cfg.FrontendRedirectURL)
			defaultApplied = true
		}
		if origin != "" {
			overrides = append(overrides, config.EmailOAuthOriginOverride{
				Origin:       origin,
				ClientID:     item.ClientID,
				ClientSecret: item.ClientSecret,
				RedirectURL:  item.RedirectURL,
			})
		}
	}
	if defaultApplied {
		cfg.AllowedRedirectOrigins = origins
		cfg.OriginOverrides = overrides
	}
	return cfg
}
