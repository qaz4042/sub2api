# Google 授权登录回调卡住故障记录

记录日期：2026-06-22  
实例：`portal.lizubin.online`  
服务器：`my2g`

## 一、问题现象

用户点击 Google 授权登录后，浏览器可以正常跳转到 Google 授权页，也可以回到 Sub2API 回调地址，但回调阶段明显卡住。

线上日志中可以看到：

```text
/api/v1/auth/oauth/google/start    302    约 3ms
/api/v1/auth/oauth/google/callback 302    约 120005ms
/auth/oauth/callback               200    约 2ms
```

这说明：

- Google 登录入口可以正常生成授权地址。
- Google 回调请求可以进入 Sub2API 后端。
- 卡住点不在前端路由，也不是 Nginx 没有转发回调。
- 卡住点发生在后端 callback 内部，主要怀疑是服务端换取 token 或获取 userinfo 阶段出站访问超时。

## 二、涉及链路

普通 Google 登录使用的是 Sub2API 的 email OAuth 链路：

```text
前端登录按钮
  -> /api/v1/auth/oauth/google/start
  -> Google 授权页
  -> /api/v1/auth/oauth/google/callback
  -> 后端访问 Google token/userinfo 接口
  -> /auth/oauth/callback
  -> 前端完成登录或显示错误
```

当前 Google 登录回调地址应为：

```text
https://portal.lizubin.online/api/v1/auth/oauth/google/callback
```

不要和通用 OIDC 回调或其他第三方登录回调混用：

```text
/auth/oidc/callback
/api/v1/auth/oauth/oidc/callback
/api/oauth/ruge/callback
```

## 三、根因

根因是服务器/容器出站访问 Google 相关接口不可用。

具体表现：

- Sub2API 容器没有配置通用出站代理环境变量。
- Mihomo 容器虽然在运行，但 Sub2API 没有默认通过 Mihomo 出站。
- Mihomo 原规则主要覆盖 OpenAI/ChatGPT 相关域名，未覆盖 Google OAuth 需要的域名。
- Google OAuth callback 后端需要访问：
  - `oauth2.googleapis.com`
  - `openidconnect.googleapis.com`
- 这些域名未通过代理时会超时，导致 callback 请求卡到约 120 秒。

同时，日志中还出现 GitHub raw 远程价格同步超时，进一步说明这是通用出站访问问题，不是单一 Google OAuth 配置问题。

## 四、修复动作

### 1. 备份线上配置

修复前已备份：

```text
/opt/sub2api/docker-compose.yml.bak-google-oauth-20260622-042859
/opt/sub2api/mihomo/config.yaml.bak-google-oauth-20260622-042859
```

### 2. 补充 Mihomo 代理规则

在 `/opt/sub2api/mihomo/config.yaml` 中将以下域名加入代理规则：

```yaml
- DOMAIN-SUFFIX,googleapis.com,OpenAI-SG
- DOMAIN-SUFFIX,google.com,OpenAI-SG
- DOMAIN-SUFFIX,githubusercontent.com,OpenAI-SG
```

其中 Google OAuth 关键域名是 `googleapis.com` 和 `google.com`。

### 3. 给 Sub2API 容器配置出站代理

在 `/opt/sub2api/docker-compose.yml` 的 Sub2API 服务中加入：

```yaml
- UPDATE_PROXY_URL=${UPDATE_PROXY_URL:-http://mihomo:7890}
- HTTP_PROXY=${HTTP_PROXY:-http://mihomo:7890}
- HTTPS_PROXY=${HTTPS_PROXY:-http://mihomo:7890}
- NO_PROXY=${NO_PROXY:-localhost,127.0.0.1,::1,postgres,redis,mihomo,sub2api}
- http_proxy=${HTTP_PROXY:-http://mihomo:7890}
- https_proxy=${HTTPS_PROXY:-http://mihomo:7890}
- no_proxy=${NO_PROXY:-localhost,127.0.0.1,::1,postgres,redis,mihomo,sub2api}
```

### 4. 固定本地镜像标签

线上 Compose 原来使用：

```yaml
image: weishaw/sub2api:latest
```

但当前服务器本地实际使用的是：

```yaml
image: sub2api:0.0.0-my2g.20260618.1
```

修复时已将 Compose 固定到本地镜像标签，避免重启时 Docker 尝试访问 Docker Hub 拉取 `latest` 并失败。

### 5. 修复 healthcheck 被代理影响的问题

添加 `HTTP_PROXY/HTTPS_PROXY` 后，容器内 healthcheck 的 `wget http://localhost:8080/health` 会被代理环境影响，导致 Docker 误判为 `unhealthy`。

已将 healthcheck 改为取消代理环境后访问本机：

```yaml
test:
  - CMD-SHELL
  - env -u HTTP_PROXY -u HTTPS_PROXY -u http_proxy -u https_proxy wget -q -T 5 -O /dev/null http://localhost:8080/health
```

## 五、验证结果

### 1. 容器状态

修复后确认以下容器均为 healthy：

```text
sub2api
sub2api-postgres
sub2api-redis
sub2api-mihomo
```

### 2. 健康检查

```text
GET http://127.0.0.1:8080/health -> 200
```

### 3. Google 出站访问

Sub2API 容器内直接访问 Google OAuth 相关接口已经自动走 Mihomo：

```text
https://oauth2.googleapis.com/token                  约 0.18s 返回
https://openidconnect.googleapis.com/v1/userinfo     约 0.19s 返回
```

未携带参数或 token 时返回 `404` / `401` 属于正常快速响应，关键是已经不再超时。

### 4. Callback 时延

用无效 code 测试 callback 链路：

```text
/api/v1/auth/oauth/google/callback?code=invalid_test_code&state=...
```

修复前：

```text
302    约 120005ms
```

修复后：

```text
302    约 0.20s
```

这说明后端 callback 已经不再卡住，真实用户重新发起一次 Google 登录即可验证完整登录流程。

## 六、后续注意事项

- Google 控制台中的授权回调地址应保持为：

```text
https://portal.lizubin.online/api/v1/auth/oauth/google/callback
```

- 旧的授权 code 不要重复刷新使用，授权 code 通常只能使用一次且很快过期。
- 修改代理环境变量后，要同步检查容器 healthcheck，避免本机健康检查被代理劫持。
- 如果未来启用 GitHub、OIDC、如歌或其他第三方登录，也要确认后端 token/userinfo/JWKS 接口可以从服务器出站访问。
- 不要为了 OAuth 登录全局放开私网、HTTP 或任意公网访问；应按实际域名补充代理规则和安全白名单。
