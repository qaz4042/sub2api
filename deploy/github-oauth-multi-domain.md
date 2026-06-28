# GitHub OAuth 多域名结论

## 背景

当前生产服务部署在 `my4g`，同一套 Sub2API 服务同时暴露：

- `https://portal.lizubin.online`
- `https://codex.lizubin.online`

后台“邮箱快捷登录”里的 GitHub OAuth 用于用户登录。

## 结论

GitHub OAuth App 对 `redirect_uri` 校验较严格。实测一个 GitHub OAuth App 配置为：

```text
https://portal.lizubin.online/api/v1/auth/oauth/github/callback
```

时，不能接受：

```text
https://codex.lizubin.online/api/v1/auth/oauth/github/callback
```

GitHub 会报：

```text
The redirect_uri is not associated with this application.
```

因此，`portal.lizubin.online` 和 `codex.lizubin.online` 需要使用不同的 GitHub OAuth App。

## 当前 App 分工

`portal.lizubin.online`：

- GitHub OAuth App ID: `3694795`
- Client ID: `Ov23li7u0QJ6sC1dWrsE`
- Callback URL: `https://portal.lizubin.online/api/v1/auth/oauth/github/callback`

`codex.lizubin.online`：

- GitHub OAuth App ID: `3694790`
- Client ID: `Ov23li7s7arryMeHppfp`
- Callback URL: `https://codex.lizubin.online/api/v1/auth/oauth/github/callback`

## 后端配置方向

默认 GitHub OAuth 配置保留给 `portal.lizubin.online`。

`codex.lizubin.online` 通过按 origin 覆盖的配置选择独立的 `client_id`、`client_secret` 和 `redirect_url`。

示例：

```bash
GITHUB_OAUTH_ALLOWED_REDIRECT_ORIGINS=https://portal.lizubin.online,https://codex.lizubin.online
GITHUB_OAUTH_ORIGIN_OVERRIDES='[{"origin":"https://codex.lizubin.online","client_id":"Ov23li7s7arryMeHppfp","client_secret":"<codex-app-secret>","redirect_url":"https://codex.lizubin.online/api/v1/auth/oauth/github/callback"}]'
```

注意：`client_secret` 只写入服务器环境，不写入文档和仓库。

## 2026-06-28 落地状态

已改为后台“邮箱快捷登录”列表配置，数据库只需要维护一组列表 `email_oauth_clients`，列表里可放多条 GitHub / Google 配置，每条按 `provider + origin` 区分。

当前线上配置：

- GitHub `portal.lizubin.online`：已启用，Client Secret 已配置。
- GitHub `codex.lizubin.online`：配置已建但禁用，当前缺少 Client Secret。
- Google `portal.lizubin.online` / `codex.lizubin.online`：均已启用，复用同一套 Google Client。

前后端行为：

- GitHub / Google 授权 start / callback 会按当前请求域名选择对应 OAuth 配置。
- 公开设置 `/api/v1/settings/public` 也会按当前请求域名过滤登录按钮。
- 当 `email_oauth_clients` 为空时，保持旧的全局配置兼容逻辑。
- 当列表存在时，某个 provider 只有在当前 `origin` 下启用且 `client_id` / `client_secret` 都存在，才会对前端返回 enabled。

已部署到 `my4g`：

```text
/opt/sub2api/releases/20260628-224646-public-oauth-origin
```

验证结果：

- `portal.lizubin.online`：GitHub `true`，Google `true`。
- `codex.lizubin.online`：GitHub `false`，Google `true`。

补充：`codex.lizubin.online` 外部 HTTPS 入口曾出现 TLS reset，本次应用层验证使用 `my4g` 本机 Host 头完成；该问题属于入口层，不属于 OAuth 配置选择逻辑。
