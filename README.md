# Sub2API

AI API 网关平台，用于将订阅类 AI 产品额度转换为可分发、可计费、可监控的 API Key 访问方式。平台负责鉴权、转发、调度、计费、限流与后台管理。

## Coding Agent 快速上下文

- 后端：Go、Gin、Ent，入口在 `backend/cmd/server`，核心模块在 `backend/internal`。
- 前端：Vue 3、Vite、TailwindCSS，源码在 `frontend/src`。
- 数据依赖：PostgreSQL 15+、Redis 7+。
- 部署材料：`deploy/`，支付配置：`docs/PAYMENT.md` / `docs/PAYMENT_CN.md`。
- 修改 `backend/ent/schema` 后，需要重新生成 Ent 与 Wire 相关代码。

## 核心功能

- 多上游账号管理：支持 OAuth、API Key 等账号类型。
- API Key 分发：为用户生成和管理平台侧访问密钥。
- 精细计费：支持 Token 级用量统计、余额与倍率计费。
- 智能调度：支持账号选择、粘性会话、分组隔离和负载分配。
- 并发与限流：可配置用户级、账号级并发及请求/Token 速率限制。
- 管理后台：提供用户、账号、额度、日志、监控和更新检查等管理入口。

## 本地开发

后端开发：

```bash
make dev-backend
```

默认连接 `127.0.0.1:5432` 的 PostgreSQL 和 `127.0.0.1:6379` 的 Redis，运行态文件生成在 `.dev-data/`。
首次运行会自动生成本地配置并执行迁移。默认数据库连接为 `postgres/postgres@127.0.0.1:5432/sub2api_dev`。
需要换数据库或账号时，新建未提交的 `.dev.env`：

```makefile
DEV_DATABASE_USER=your_user
DEV_DATABASE_PASSWORD=your_password
DEV_DATABASE_DBNAME=sub2api_dev
```

前端开发：

```bash
cd frontend
pnpm install
pnpm run dev
```

构建嵌入式前端后运行后端：

```bash
cd frontend && pnpm install && pnpm run build
cd ../backend && go run ./cmd/server
```

代码生成：

```bash
cd backend
go generate ./ent
go generate ./cmd/server
```

## 目录结构

```text
backend/   Go 后端服务、网关、配置、模型、业务逻辑与 HTTP handlers
frontend/  Vue 前端应用、页面、组件、状态管理与 API 调用
deploy/    Docker Compose、安装脚本和配置示例
docs/      部署、运维、运行模式、支付与合规相关文档
```

## 文档索引

- 站点运营思路：[docs/站点运营思路.md](docs/站点运营思路.md)
- 费用公告：[docs/费用公告.md](docs/费用公告.md)
- 使用指南：[docs/使用指南.md](docs/使用指南.md)
- 开发者解决方案：[docs/开发者解决方案.md](docs/开发者解决方案.md)
- 企业解决方案：[docs/企业解决方案.md](docs/企业解决方案.md)
- 部署说明：[docs/DEPLOYMENT.md](docs/DEPLOYMENT.md)
- 运维提示：[docs/OPERATIONS.md](docs/OPERATIONS.md)
- 运行模式与兼容：[docs/MODES.md](docs/MODES.md)
- 风险声明：[docs/DISCLAIMER.md](docs/DISCLAIMER.md)
- 支付配置：[docs/PAYMENT.md](docs/PAYMENT.md)、[docs/PAYMENT_CN.md](docs/PAYMENT_CN.md)
- 许可证：[LGPL-3.0-or-later](LICENSE)
