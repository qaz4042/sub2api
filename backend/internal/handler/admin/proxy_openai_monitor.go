package admin

import (
	"database/sql"

	"github.com/Wei-Shaw/sub2api/internal/features/proxyopen"
	"github.com/Wei-Shaw/sub2api/internal/features/proxyquality"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ProxyMonitorRuntime composes optional features at the host boundary. Each
// feature retains its own storage, policy, API and lifecycle.
type ProxyMonitorRuntime struct {
	*proxyopen.Runner
	quality *proxyquality.Runner
}

func ProvideProxyOpenAIMonitor(db *sql.DB, prober service.ProxyExitInfoProber, gateway *service.OpenAIGatewayService) *ProxyMonitorRuntime {
	runner := &ProxyMonitorRuntime{Runner: proxyopen.New(&proxyopen.Store{DB: db}, prober), quality: proxyquality.New(&proxyquality.Store{DB: db})}
	gateway.SetOpenAIAccountObserver(runner.quality)
	runner.Runner.Start()
	runner.quality.Start()
	return runner
}
func (r *ProxyMonitorRuntime) Stop() {
	if r == nil {
		return
	}
	r.quality.Stop()
	r.Runner.Stop()
}
func (r *ProxyMonitorRuntime) Register(g *gin.RouterGroup) {
	r.Runner.Register(g)
	r.quality.Register(g)
}
func ProvideProxyMonitorHandler(adminService service.AdminService, monitor *ProxyMonitorRuntime) *ProxyHandler {
	h := NewProxyHandler(adminService)
	h.monitor = monitor
	return h
}
func (h *ProxyHandler) RegisterOpenAIMonitor(group *gin.RouterGroup) {
	if h.monitor != nil {
		h.monitor.Register(group)
	}
}
