# GitHub OAuth 多域名配置

同一套服务通过多个域名提供前端时，OAuth 提供方可能要求每个回调地址使用独立的 OAuth App。配置 `GITHUB_OAUTH_ALLOWED_REDIRECT_ORIGINS` 后，服务只接受精确匹配的 `scheme://host`，不会为未登记域名展示或启动 GitHub 登录。

```bash
GITHUB_OAUTH_ALLOWED_REDIRECT_ORIGINS=https://portal.example.com,https://codex.example.com
GITHUB_OAUTH_ORIGIN_OVERRIDES='[{"origin":"https://codex.example.com","client_id":"Ov23...","client_secret":"<server-secret>","redirect_url":"https://codex.example.com/api/v1/auth/oauth/github/callback"}]'
```

默认 GitHub 客户端配置继续服务第一个域名；覆盖项中的 `client_secret` 只放在服务器环境变量中，不提交到仓库。Google 可复用同一客户端时，只需配置允许的来源，并确认各回调地址已登记。
