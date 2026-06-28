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

