package admin

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type stubAdminUserLookup struct {
	user *service.User
	err  error
}

func newStubAdminUserLookup(password string) *stubAdminUserLookup {
	user := &service.User{ID: 1, Email: "admin@example.com", Role: service.RoleAdmin, Status: service.StatusActive}
	_ = user.SetPassword(password)
	return &stubAdminUserLookup{user: user}
}

func (s *stubAdminUserLookup) GetByID(ctx context.Context, id int64) (*service.User, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.user, nil
}
