package admin

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type AdminUserLookup interface {
	GetByID(ctx context.Context, id int64) (*service.User, error)
}

type adminPasswordRequest struct {
	Password string `json:"password"`
}

func requireAdminPassword(c *gin.Context, userLookup AdminUserLookup) bool {
	password := c.GetHeader("x-admin-password")
	if password == "" && c.Request != nil && c.Request.Body != nil {
		var req adminPasswordRequest
		if err := c.ShouldBindJSON(&req); err == nil {
			password = req.Password
		}
	}
	if password == "" {
		response.BadRequest(c, "password is required")
		return false
	}

	sub, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return false
	}
	if userLookup == nil {
		response.InternalError(c, "admin password verification is unavailable")
		return false
	}

	user, err := userLookup.GetByID(c.Request.Context(), sub.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return false
	}
	if !user.CheckPassword(password) {
		response.BadRequest(c, "incorrect admin password")
		return false
	}
	return true
}
