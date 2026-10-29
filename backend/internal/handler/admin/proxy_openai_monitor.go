package admin

import (
	"database/sql"

	"github.com/Wei-Shaw/sub2api/internal/features/proxyopen"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func ProvideProxyOpenAIMonitor(db *sql.DB, prober service.ProxyExitInfoProber) *proxyopen.Runner {
	runner := proxyopen.New(&proxyopen.Store{DB: db}, prober)
	runner.Start()
	return runner
}
func ProvideProxyMonitorHandler(adminService service.AdminService, monitor *proxyopen.Runner) *ProxyHandler {
	h := NewProxyHandler(adminService)
	h.monitor = monitor
	return h
}
func (h *ProxyHandler) RegisterOpenAIMonitor(group *gin.RouterGroup) {
	if h.monitor != nil {
		h.monitor.Register(group)
	}
}
