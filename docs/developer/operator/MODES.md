# 运行模式与兼容

## Simple Mode

Simple Mode 面向个人或内部团队，用于快速访问而不启用完整 SaaS 功能。

- 启用：设置环境变量 `RUN_MODE=simple`。
- 差异：隐藏 SaaS 相关功能，并跳过计费流程。
- 生产环境：还需要设置 `SIMPLE_MODE_CONFIRM=true` 才允许启动。

## Antigravity

Sub2API 支持 Antigravity 账号，授权后可使用 Claude 与 Gemini 专用入口：

| Endpoint | Model |
| --- | --- |
| `/antigravity/v1/messages` | Claude models |
| `/antigravity/v1beta/` | Gemini models |

Claude Code 示例：

```bash
export ANTHROPIC_BASE_URL="http://localhost:8080/antigravity"
export ANTHROPIC_AUTH_TOKEN="sk-xxx"
```

Antigravity 账号支持可选的 hybrid scheduling。启用后，通用入口 `/v1/messages` 和 `/v1beta/` 也可能路由到 Antigravity 账号。

注意：Anthropic Claude 与 Antigravity Claude 不应混在同一会话上下文中使用，请通过分组隔离。

## Sora

Sora 相关能力当前因上游集成与媒体交付问题暂不可依赖。现有 `gateway.sora_*` 配置项保留，但在问题解决前不应作为生产稳定能力使用。

## Claude Code 已知问题

在 Claude Code 中，Plan Mode 可能无法自动退出。临时处理方式是按 `Shift + Tab` 手动退出 Plan Mode，再输入确认或拒绝。
