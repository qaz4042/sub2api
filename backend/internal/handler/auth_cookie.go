package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	authRefreshTokenCookieName = "sub2api_refresh_token"
	authRefreshTokenCookiePath = "/api/v1/auth"
)

func (h *AuthHandler) setRefreshTokenCookie(c *gin.Context, refreshToken string) {
	if refreshToken == "" {
		return
	}

	maxAge := 30 * 24 * 60 * 60
	if h != nil && h.cfg != nil && h.cfg.JWT.RefreshTokenExpireDays > 0 {
		maxAge = int((time.Duration(h.cfg.JWT.RefreshTokenExpireDays) * 24 * time.Hour).Seconds())
	}

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     authRefreshTokenCookieName,
		Value:    refreshToken,
		Path:     authRefreshTokenCookiePath,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   isRequestHTTPS(c),
		SameSite: http.SameSiteLaxMode,
	})
}

func clearRefreshTokenCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     authRefreshTokenCookieName,
		Value:    "",
		Path:     authRefreshTokenCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isRequestHTTPS(c),
		SameSite: http.SameSiteLaxMode,
	})
}

func readRefreshTokenCookie(c *gin.Context) string {
	cookie, err := c.Request.Cookie(authRefreshTokenCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}
