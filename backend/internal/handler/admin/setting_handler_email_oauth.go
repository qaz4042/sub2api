package admin

import (
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// emailOAuthClientsToDTO 将管理端需要的客户端元数据转换为 DTO，永不返回 Secret 明文。
func emailOAuthClientsToDTO(items []service.EmailOAuthClientSetting) []dto.EmailOAuthClient {
	if len(items) == 0 {
		return []dto.EmailOAuthClient{}
	}
	out := make([]dto.EmailOAuthClient, 0, len(items))
	for _, item := range items {
		out = append(out, dto.EmailOAuthClient{
			ID:                     item.ID,
			Provider:               item.Provider,
			Name:                   item.Name,
			Origin:                 item.Origin,
			Enabled:                item.Enabled,
			ClientID:               item.ClientID,
			ClientSecretConfigured: item.ClientSecretConfigured || strings.TrimSpace(item.ClientSecret) != "",
			RedirectURL:            item.RedirectURL,
			FrontendRedirectURL:    item.FrontendRedirectURL,
			SortOrder:              item.SortOrder,
		})
	}
	return out
}

func firstEmailOAuthClient(items []service.EmailOAuthClientSetting, provider string) *service.EmailOAuthClientSetting {
	for i := range items {
		if strings.EqualFold(strings.TrimSpace(items[i].Provider), provider) {
			return &items[i]
		}
	}
	return nil
}

func emailOAuthLegacyFields(item *service.EmailOAuthClientSetting, defaultFrontendRedirect string) (string, string, string, string) {
	if item == nil {
		return "", "", "", defaultFrontendRedirect
	}
	return strings.TrimSpace(item.ClientID), strings.TrimSpace(item.ClientSecret), strings.TrimSpace(item.RedirectURL), firstNonEmpty(item.FrontendRedirectURL, defaultFrontendRedirect)
}

func emailOAuthClientSecretKey(item service.EmailOAuthClientSetting) string {
	return strings.ToLower(strings.TrimSpace(item.Provider)) + "|" + strings.TrimRight(strings.ToLower(strings.TrimSpace(item.Origin)), "/") + "|" + strings.TrimSpace(item.ClientID)
}

// validateEmailOAuthClientSettings 校验来源、回调地址和启用客户端的凭据完整性。
// previous 仅用于判断留空的 Secret 是否能沿用已有配置。
func validateEmailOAuthClientSettings(items, previous []service.EmailOAuthClientSetting) error {
	previousSecrets := make(map[string]bool, len(previous))
	for _, item := range previous {
		previousSecrets[emailOAuthClientSecretKey(item)] = strings.TrimSpace(item.ClientSecret) != "" || item.ClientSecretConfigured
	}
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		provider := strings.ToLower(strings.TrimSpace(item.Provider))
		if provider != "github" && provider != "google" {
			return fmt.Errorf("Email OAuth provider must be github or google")
		}
		origin := strings.TrimSpace(item.Origin)
		if origin == "" {
			return fmt.Errorf("Email OAuth origin is required")
		}
		if err := config.ValidateAbsoluteHTTPOrigin(origin); err != nil {
			return fmt.Errorf("Email OAuth origin must be an absolute http(s) origin")
		}
		key := provider + "|" + strings.TrimRight(strings.ToLower(origin), "/")
		if _, ok := seen[key]; ok {
			return fmt.Errorf("Email OAuth origin cannot be duplicated for the same provider")
		}
		seen[key] = struct{}{}
		if strings.TrimSpace(item.RedirectURL) == "" {
			return fmt.Errorf("Email OAuth redirect URL is required")
		}
		if err := config.ValidateAbsoluteHTTPURL(item.RedirectURL); err != nil {
			return fmt.Errorf("Email OAuth redirect URL must be an absolute http(s) URL")
		}
		if strings.TrimSpace(item.FrontendRedirectURL) == "" {
			return fmt.Errorf("Email OAuth frontend redirect URL is required")
		}
		if err := config.ValidateFrontendRedirectURL(item.FrontendRedirectURL); err != nil {
			return fmt.Errorf("Email OAuth frontend redirect URL is invalid")
		}
		if !item.Enabled {
			continue
		}
		if strings.TrimSpace(item.ClientID) == "" {
			return fmt.Errorf("Email OAuth Client ID is required when enabled")
		}
		if strings.TrimSpace(item.ClientSecret) == "" && !previousSecrets[emailOAuthClientSecretKey(item)] {
			return fmt.Errorf("Email OAuth Client Secret is required when enabled")
		}
	}
	return nil
}
