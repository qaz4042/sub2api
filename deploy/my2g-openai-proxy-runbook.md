# 国内服务器访问 OpenAI 的代理方案

## 结论

国内服务器通常不能稳定直连 `api.openai.com`、`auth.openai.com`、`chatgpt.com` 等 OpenAI/ChatGPT 相关域名。当前 Sub2API 部署优先采用“账号级出站代理”闭环，而不是先做整机透明代理。

推荐链路：

```text
用户请求
  -> https://portal.lizubin.online
  -> Sub2API
  -> OpenAI 账号绑定的 proxy_id
  -> Mihomo/远程代理
  -> OpenAI/ChatGPT 上游
```

## 账号级代理与整机代理的取舍

优先选择账号级代理：

- 只影响绑定代理的 OpenAI/ChatGPT 账号，影响面小。
- 不改代码，只需要在 Sub2API 后台新增代理并绑定账号。
- 不影响 Nginx、Certbot、Docker、系统更新、PostgreSQL、Redis 等服务器基础组件。
- 支持不同账号绑定不同出口 IP，便于账号隔离和风控控制。
- 排障路径更短：可以单独判断账号、代理、上游和 Sub2API 配置问题。

不优先选择整机代理/TUN：

- 影响范围大，规则写错可能影响证书续期、Docker 拉镜像、系统更新或内网访问。
- Docker 容器不会天然继承宿主机“系统代理”，除非显式传入代理环境变量，或使用 TUN/透明代理接管网络层。
- 对当前目标来说属于基础设施改造，不是最小业务闭环。

只有在服务器上很多程序都需要访问被墙资源，且不想逐个配置代理时，再考虑整机 TUN/透明代理。

## Mihomo 与 ClashX Meta 的关系

二者不是完全同一层次：

```text
ClashX Meta = macOS 图形客户端
Mihomo      = Clash.Meta 代理核心/内核
```

本地 Mac 上的关系：

```text
ClashX Meta App
  -> 菜单栏 UI、节点选择、系统代理/TUN 开关
  -> 调用 Mihomo/Clash.Meta 核心
  -> 机场节点
```

服务器上的推荐形态：

```text
Docker/systemd
  -> Mihomo 核心
  -> 机场节点
```

服务器不需要安装 ClashX Meta 图形客户端，通常直接运行 Mihomo 容器或 systemd 服务即可。ClashX Meta 使用的 Clash/Mihomo YAML 订阅配置通常可以给 Mihomo 直接使用。

## 推荐部署形态：Mihomo Sidecar

在 `/opt/sub2api/` 下运行一个 Mihomo 容器，与 `sub2api` 加入同一个 Docker 网络。Sub2API 后台只配置一个逻辑代理入口：

```text
http://mihomo:7890
```

或：

```text
socks5://mihomo:7891
```

示例 Compose 覆盖文件 `/opt/sub2api/compose.mihomo.yml`：

```yaml
services:
  mihomo:
    image: metacubex/mihomo:latest
    container_name: sub2api-mihomo
    restart: unless-stopped
    volumes:
      - ./mihomo:/root/.config/mihomo
    command: ["-d", "/root/.config/mihomo"]
    networks:
      - sub2api-network
```

启动示例：

```bash
cd /opt/sub2api
sudo docker compose -f docker-compose.yml -f compose.my2g.yml -f compose.mihomo.yml up -d
```

如果服务器拉不到 `metacubex/mihomo:latest`，可以和应用镜像一样：本地 `docker pull` 后 `docker save`，上传到服务器再 `docker load`。

## 多节点自动切换

代理不稳定时，优先让 Mihomo 在内部做节点切换，而不是让 Sub2API 频繁切换账号代理。

推荐：`fallback` 节点组。

原因：OpenAI/ChatGPT 类账号更怕出口 IP 频繁变化。`fallback` 只有主节点不可用时才切换，通常比 `url-test` 或 `load-balance` 更稳。

不建议同一个 OpenAI/ChatGPT 账号使用 `load-balance` 随机换出口。

示例 Mihomo 配置片段：

```yaml
mixed-port: 7890
socks-port: 7891
allow-lan: true
bind-address: "*"
mode: rule
log-level: info

proxy-groups:
  - name: OpenAI
    type: fallback
    proxies:
      - 新加坡 SG-HY2
      - 日本 JP-HY2
      - 香港 HK-A-Gemini
      - 美国 USLA-A
    url: https://api.openai.com/v1/models
    interval: 300
    tolerance: 50

rules:
  - DOMAIN-SUFFIX,openai.com,OpenAI
  - DOMAIN-SUFFIX,chatgpt.com,OpenAI
  - DOMAIN-SUFFIX,oaistatic.com,OpenAI
  - DOMAIN-SUFFIX,oaiusercontent.com,OpenAI
  - MATCH,DIRECT
```

最小排障阶段也可以临时使用全局模式：

```yaml
mode: global

proxy-groups:
  - name: GLOBAL
    type: fallback
    proxies:
      - 新加坡 SG-HY2
      - 日本 JP-HY2
      - 香港 HK-A-Gemini
    url: https://api.openai.com/v1/models
    interval: 300
```

## Sub2API 后台配置

新增代理：

```text
名称: mihomo-openai
协议: http
主机: mihomo
端口: 7890
用户名/密码: 留空
状态: active
```

然后编辑 OpenAI 账号，绑定该代理的 `proxy_id`，再执行账号测试。

Sub2API 也支持代理的 `fallback_mode` 和 `backup_proxy_id`，但它更适合处理“代理到期后账号迁移”，不是处理节点偶发不稳定的首选机制。节点健康检查和自动切换优先交给 Mihomo。

## 验证命令

测试服务器直连是否可用：

```bash
ssh my2g 'curl -I --connect-timeout 8 https://api.openai.com/v1/models'
ssh my2g 'curl -I --connect-timeout 8 https://auth.openai.com'
```

测试 Sub2API 容器经 Mihomo 访问 OpenAI：

```bash
ssh my2g 'sudo docker exec sub2api sh -lc "curl -sS -x http://mihomo:7890 --connect-timeout 15 --max-time 35 -o /tmp/openai-proxy-test.out -w \"http_code=%{http_code} exit=%{exitcode} time=%{time_total}\n\" https://api.openai.com/v1/models; sed -n \"1,12p\" /tmp/openai-proxy-test.out"'
```

能返回 `401` 或 OpenAI JSON 错误，说明网络链路已通；超时、连接重置或 TLS 握手失败才说明代理链路仍有问题。

最终用户侧闭环：

```bash
curl https://portal.lizubin.online/openai/v1/chat/completions \
  -H 'Authorization: Bearer sk-your-sub2api-user-key' \
  -H 'Content-Type: application/json' \
  -d '{"model":"gpt-4.1-mini","messages":[{"role":"user","content":"ping"}]}'
```

## my2g 实施记录：2026-06-20

本次已按本机 ClashX Meta 当前配置，在 `my2g` 上完成 Sub2API 的 OpenAI 出站代理处理。

### 本机 ClashX Meta 依据

本机配置目录：

```text
~/.config/clash.meta/
```

当前 ClashX Meta 状态：

```text
selectConfigName: 魔戒.net
selectOutBoundMode: rule
proxyPort: 7890
restoreTunProxy: true
```

本次使用的本机配置文件：

```text
~/.config/clash.meta/魔戒.net.yaml
~/.config/clash.meta/綺夢雲.yaml
```

节点选择原则：

- 魔戒.net 作为主力机场。
- 綺夢雲作为备用机场。
- 只抽取新加坡相关节点。
- 不把订阅链接、节点密码、UUID、Reality 公钥等敏感配置写入 Git。

实际 fallback 候选顺序：

```text
MJ-新加坡SG-HY2
MJ-新加坡-优化-Gemini-GPT
MJ-新加坡-优化2-Gemini-GPT
MJ-新加坡-优化3-Gemini
QM-2X新加坡zf2（电信）
QM-1X新加坡3(电信)
```

### 远端部署结果

远端部署目录：

```text
/opt/sub2api/
```

新增或更新的远端文件：

```text
/opt/sub2api/compose.mihomo.yml
/opt/sub2api/mihomo/config.yaml
```

文件权限：

```text
/opt/sub2api/mihomo            root:root 700
/opt/sub2api/mihomo/config.yaml root:root 600
/opt/sub2api/compose.mihomo.yml root:root 640
```

运行容器：

```text
sub2api-mihomo
```

镜像：

```text
sub2api-mihomo:v1.19.27-my2g
```

由于 `my2g` 服务器直连 Docker Hub 超时，本机 Docker Hub 又触发匿名拉取限流，最终采用以下方式处理镜像：

1. 从 GitHub release 下载官方 `mihomo-linux-amd64-v1.19.27.gz`。
2. 确认二进制为 Linux amd64 静态链接 ELF。
3. 使用 `FROM scratch` 构建最小镜像。
4. 将本机 CA bundle 一起写入镜像，保证 HTTPS 健康检查和上游 TLS 可用。
5. `docker save` 后上传到 `my2g`，再 `docker load` 导入。

### Compose 覆盖配置

远端 `/opt/sub2api/compose.mihomo.yml`：

```yaml
services:
  mihomo:
    image: sub2api-mihomo:v1.19.27-my2g
    container_name: sub2api-mihomo
    restart: unless-stopped
    volumes:
      - ./mihomo/config.yaml:/config.yaml:ro
    command: ["-f", "/config.yaml"]
    networks:
      - sub2api-network
    healthcheck:
      test: ["CMD", "/mihomo", "-t", "-f", "/config.yaml"]
      interval: 60s
      timeout: 15s
      retries: 3
      start_period: 20s
```

启动命令：

```bash
ssh my2g 'cd /opt/sub2api && sudo docker compose -f docker-compose.yml -f compose.my2g.yml -f compose.mihomo.yml up -d mihomo'
```

完整服务状态检查：

```bash
ssh my2g 'cd /opt/sub2api && sudo docker compose -f docker-compose.yml -f compose.my2g.yml -f compose.mihomo.yml ps'
```

### Mihomo 运行配置摘要

远端 `/opt/sub2api/mihomo/config.yaml` 包含敏感节点凭据，不提交仓库，不公开粘贴。

非敏感摘要：

```yaml
mixed-port: 7890
socks-port: 7891
allow-lan: true
bind-address: "*"
mode: rule
log-level: info
external-controller: 127.0.0.1:9090

proxy-groups:
  - name: OpenAI-SG
    type: fallback
    url: https://chatgpt.com/backend-api/codex/responses
    interval: 300
    timeout: 8000
    lazy: false
    expected-status: 200-499

rules:
  - DOMAIN-SUFFIX,openai.com,OpenAI-SG
  - DOMAIN-SUFFIX,chatgpt.com,OpenAI-SG
  - DOMAIN-SUFFIX,oaistatic.com,OpenAI-SG
  - DOMAIN-SUFFIX,oaiusercontent.com,OpenAI-SG
  - DOMAIN-SUFFIX,auth0.com,OpenAI-SG
  - DOMAIN-SUFFIX,intercom.io,OpenAI-SG
  - DOMAIN-SUFFIX,intercomcdn.com,OpenAI-SG
  - MATCH,DIRECT
```

### 配置校验

远端配置检查已通过：

```bash
ssh my2g 'sudo docker run --rm -v /opt/sub2api/mihomo/config.yaml:/config.yaml:ro sub2api-mihomo:v1.19.27-my2g -t -f /config.yaml'
```

预期输出：

```text
configuration file /config.yaml test is successful
```

### Sub2API 后台代理记录

数据库中已创建或更新逻辑代理：

```text
proxy_id: 1
name: mihomo-openai
protocol: http
host: mihomo
port: 7890
status: active
fallback_mode: none
```

执行逻辑：

```sql
WITH existing AS (
  SELECT id FROM proxies
  WHERE name = 'mihomo-openai' AND deleted_at IS NULL
  LIMIT 1
),
updated AS (
  UPDATE proxies
  SET protocol = 'http',
      host = 'mihomo',
      port = 7890,
      username = '',
      password = '',
      status = 'active',
      fallback_mode = 'none',
      backup_proxy_id = NULL,
      updated_at = now()
  WHERE id IN (SELECT id FROM existing)
  RETURNING id
),
inserted AS (
  INSERT INTO proxies (
    name, protocol, host, port, username, password, status, fallback_mode, expiry_warn_days
  )
  SELECT 'mihomo-openai', 'http', 'mihomo', 7890, '', '', 'active', 'none', 7
  WHERE NOT EXISTS (SELECT 1 FROM existing)
  RETURNING id
)
SELECT 'proxy_id=' || id AS result FROM updated
UNION ALL
SELECT 'proxy_id=' || id FROM inserted;
```

### OpenAI 账号绑定

本次发现当前活跃账号：

```text
platform: openai
type: oauth
status: active
count: 3
```

这 3 个账号原先未绑定代理，本次已统一绑定到：

```text
proxy_id: 1
```

执行 SQL：

```sql
UPDATE accounts
SET proxy_id = 1,
    updated_at = now()
WHERE deleted_at IS NULL
  AND platform = 'openai'
  AND type = 'oauth'
  AND status = 'active';
```

绑定后统计：

```text
platform | type  | status | proxy_id | count
---------+-------+--------+----------+------
openai   | oauth | active | 1        | 3
```

### 节点连通性验证

逐节点在远端同 Docker 网络内验证过 6 个候选节点，均可经代理访问 `https://api.openai.com/v1/models` 并返回 `401`。

`401 Missing bearer authentication` 是无 OpenAI API key 请求时的预期结果，说明网络、代理、TLS 到 OpenAI 上游均已打通。

正式 sidecar 验证命令：

```bash
ssh my2g 'sudo docker exec sub2api sh -lc "curl -sS -x http://mihomo:7890 --connect-timeout 15 --max-time 35 -o /tmp/openai-proxy-test.out -w \"http_code=%{http_code} exit=%{exitcode} time=%{time_total}\n\" https://api.openai.com/v1/models; sed -n \"1,12p\" /tmp/openai-proxy-test.out"'
```

验证结果：

```text
http_code=401 exit=0
Missing bearer authentication in header
```

Mihomo 日志中可见请求命中：

```text
api.openai.com:443 match DomainSuffix(openai.com) using OpenAI-SG[...]
```

### 服务状态验证

最终远端容器状态：

```text
sub2api          healthy
sub2api-mihomo   healthy
sub2api-postgres healthy
sub2api-redis    healthy
```

公网健康检查：

```bash
curl -fsS https://portal.lizubin.online/health
```

结果：

```json
{"status":"ok"}
```

### 本地测试结果

后端单元测试：

```bash
make -C backend test-unit
```

结果：

```text
go test -tags=unit ./...
全部通过
```

前端关键单元测试：

```bash
make test-frontend-critical
```

结果：

```text
Test Files  6 passed (6)
Tests       85 passed (85)
```

测试期间出现既有 Vue `router-link` warning 和 Browserslist 数据过期提示，但测试结果为通过。

### 临时文件清理

本次已清理：

```text
本机 /tmp/sub2api-mihomo-config
本机 /tmp/sub2api-mihomo-node-tests
远端 /tmp/sub2api-mihomo-node-tests
远端 /tmp/config.yaml
远端 /tmp/compose.mihomo.yml
远端 /tmp/sub2api-mihomo-v1.19.27-my2g.tar.gz
```

保留的正式运行配置仅在：

```text
/opt/sub2api/mihomo/config.yaml
/opt/sub2api/compose.mihomo.yml
```

### 常用运维命令

查看 Mihomo 状态：

```bash
ssh my2g 'cd /opt/sub2api && sudo docker compose -f docker-compose.yml -f compose.my2g.yml -f compose.mihomo.yml ps mihomo'
```

查看 Mihomo 日志：

```bash
ssh my2g 'sudo docker logs --tail=120 sub2api-mihomo'
```

重启 Mihomo：

```bash
ssh my2g 'cd /opt/sub2api && sudo docker compose -f docker-compose.yml -f compose.my2g.yml -f compose.mihomo.yml restart mihomo'
```

重新校验配置：

```bash
ssh my2g 'sudo docker run --rm -v /opt/sub2api/mihomo/config.yaml:/config.yaml:ro sub2api-mihomo:v1.19.27-my2g -t -f /config.yaml'
```

检查 OpenAI 账号代理绑定：

```bash
ssh my2g 'sudo docker exec sub2api-postgres psql -U sub2api -d sub2api -c "SELECT platform, type, status, proxy_id, count(*) FROM accounts WHERE deleted_at IS NULL GROUP BY platform,type,status,proxy_id ORDER BY platform,type,status,proxy_id;"'
```

检查代理记录：

```bash
ssh my2g 'sudo docker exec sub2api-postgres psql -U sub2api -d sub2api -c "SELECT id,name,protocol,host,port,status,fallback_mode FROM proxies WHERE deleted_at IS NULL ORDER BY id;"'
```

## Codex Desktop `/responses` 断流排查：2026-06-20

### 问题总结

站点注册用户在 Codex Desktop 切换到当前站点后，基础请求报错：

```text
stream disconnected before completion: error sending request for url (https://portal.lizubin.online/responses)
```

最终判断：

- `/responses` 路由存在，且不带认证会返回 `401 API_KEY_REQUIRED`，说明请求可以到达 Nginx 和 Sub2API。
- 当前用户 API key、用户分组、OpenAI OAuth 账号绑定关系不是主要问题。
- Nginx 日志中历史请求出现 `502` 和 `499`，对应服务端上游失败和客户端提前断开。
- Sub2API 日志中出现 `chatgpt.com/backend-api/codex/responses` 连接超时，说明根因在 Sub2API 到 ChatGPT/Codex 上游的代理链路。
- 原 Mihomo fallback 第一节点对 `chatgpt.com` 不稳定，虽然 `api.openai.com` 健康检查可能通过，但不能代表 Codex Desktop 的 OAuth/Codex 请求链路健康。

已处理：

- 将 Mihomo `OpenAI-SG` fallback 第一节点改为 `MJ-新加坡SG-HY2`。
- 将 Mihomo fallback 健康检查 URL 改为 `https://chatgpt.com/backend-api/codex/responses`。
- 校验 Mihomo 配置通过后，重启 `sub2api-mihomo`。
- 确认 `sub2api` 和 `sub2api-mihomo` 均为 `healthy`。

验证结果：

- 使用站点注册用户的 Sub2API key 直接请求公网 `/responses`。
- 连续三次返回 `HTTP 200`。
- SSE 流中包含 `event: response.created` 和 `event: response.completed`。
- 未出现 `event: error`。

当前结论：

服务端链路已经恢复。若 Codex Desktop 客户端仍报同样错误，需要按新请求时间点继续判断：

- Nginx 无 `/responses` 访问日志：请求未到服务器，检查客户端网络、DNS、系统代理或 TUN。
- Nginx 返回 `401`：客户端未带 key 或 key 填错。
- Nginx 返回 `403`：用户、分组或模型权限限制。
- Nginx 返回 `499`：客户端提前断开。
- Nginx 返回 `502`：继续检查 Sub2API 和 Mihomo 到 `chatgpt.com` 的上游链路。

用户侧现象：

```text
stream disconnected before completion: error sending request for url (https://portal.lizubin.online/responses)
```

截图中的客户端为 Codex Desktop，切换到当前站点后，请求根路径：

```text
POST https://portal.lizubin.online/responses
```

### 路由判断

`/responses` 是 Codex Desktop 对接 Sub2API 时的正确根路径。当前 Nginx 能将该路径转发到 Sub2API：

- 不带认证访问 `/responses` 返回 `401 API_KEY_REQUIRED`，说明公网域名、Nginx 转发和应用路由均已命中。
- `/openai/v1/responses` 当前会落到前端 SPA，不是 Codex Desktop 应使用的路径。

因此本次问题不是 `/responses` 路由缺失。

### 日志现象

Nginx 历史访问日志中，Windows Codex Desktop 客户端访问 `/responses` 曾出现：

```text
502
499
```

含义：

- `502`：Sub2API 调 OpenAI/ChatGPT 上游失败后返回网关错误。
- `499`：客户端在服务端完成响应前主动断开，常见于桌面端重试、超时或用户侧网络中断。

Sub2API 应用日志中可见当时上游请求失败：

```text
Post "https://chatgpt.com/backend-api/codex/responses": dial tcp ... timeout
```

这说明失败点在 Sub2API 到 `chatgpt.com` 的 OAuth/Codex 上游链路，不在站点注册用户、API key、Nginx 或 `/responses` 路由本身。

### 修复动作

远端已备份并更新：

```text
/opt/sub2api/mihomo/config.yaml
```

调整内容：

1. 将更稳定的 `MJ-新加坡SG-HY2` 放到 `OpenAI-SG` fallback 组第一位。
2. 将 fallback 健康检查 URL 从 `https://api.openai.com/v1/models` 改为 `https://chatgpt.com/backend-api/codex/responses`。
3. 校验 Mihomo 配置通过后，重启 `sub2api-mihomo`。

原因：

- Codex OAuth 账号实际调用的是 `chatgpt.com/backend-api/codex/responses`。
- 原第一节点曾对 `chatgpt.com` 出现超时，而 `api.openai.com` 连通并不能完全代表 Codex Desktop 请求链路健康。

当前非敏感配置摘要：

```yaml
proxy-groups:
  - name: OpenAI-SG
    type: fallback
    proxies:
      - MJ-新加坡SG-HY2
      - MJ-新加坡-优化-Gemini-GPT
      - MJ-新加坡-优化2-Gemini-GPT
      - MJ-新加坡-优化3-Gemini
      - QM-2X新加坡zf2（电信）
      - QM-1X新加坡3(电信)
    url: https://chatgpt.com/backend-api/codex/responses
    interval: 300
    timeout: 8000
    lazy: false
    expected-status: 200-499
```

### 验证结果

使用站点已注册用户的 Sub2API key，从 `my2g` 直接请求公网域名：

```bash
POST https://portal.lizubin.online/responses
Content-Type: application/json
Accept: text/event-stream

{"model":"gpt-5.4","input":"ping","stream":true}
```

连续三次验证均成功：

```text
HTTP 200
Content-Type: text/event-stream
包含 event: response.created
包含 event: response.completed
```

耗时约：

```text
第 1 次：4s
第 2 次：1s
第 3 次：10s
```

Mihomo 日志确认请求命中新第一节点：

```text
chatgpt.com:443 match DomainSuffix(chatgpt.com) using OpenAI-SG[MJ-新加坡SG-HY2]
```

Sub2API 日志确认 `/responses` 正常完成：

```text
method=POST path=/responses status_code=200 platform=openai
```

### 当前结论

当前服务端链路已恢复：

```text
Codex Desktop
  -> https://portal.lizubin.online/responses
  -> Nginx
  -> Sub2API
  -> account proxy_id=1
  -> sub2api-mihomo:7890
  -> OpenAI-SG[MJ-新加坡SG-HY2]
  -> chatgpt.com/backend-api/codex/responses
```

如果客户端再次报同样错误，按新请求时间点排查：

1. Nginx 无对应 `/responses` 访问日志：请求没有到服务器，检查客户端网络、DNS、系统代理或 TUN。
2. Nginx 返回 `401`：客户端未带 API key 或 key 填错。
3. Nginx 返回 `403`：用户、分组或模型权限限制。
4. Nginx 返回 `499`：客户端先断开，检查 Codex Desktop 超时、代理或本地网络。
5. Nginx 返回 `502`：继续看 Sub2API 应用日志和 Mihomo 日志，重点确认 `chatgpt.com` 是否仍被当前节点超时。

## Codex Desktop 本地直连断流二次排查：2026-06-20

### 新结论

本次继续排查后，确认当前剩余问题不在 Sub2API 后端、Nginx `/responses` 路由、用户 API key、OpenAI 账号绑定或 Mihomo 出站链路，而在客户端到阿里云大陆服务器的公网入口链路：

```text
Codex Desktop / curl
  -> portal.lizubin.online
  -> 39.102.86.235:443
  -> 阿里云未备案域名接入拦截
  -> TLS 握手被断开
```

表现为 Codex Desktop 报：

```text
stream disconnected before completion: error sending request for url (https://portal.lizubin.online/responses)
```

这个报错发生在请求还没有进入 Nginx/应用层之前，和服务端 `/responses` 流式处理不是同一类问题。

### 关键证据

从 `my2g` 服务器本机使用站点 API key 请求公网 `/responses` 成功：

```text
POST https://portal.lizubin.online/responses
HTTP 200
Content-Type: text/event-stream
包含 response.created / response.completed
```

Nginx access log 对应成功记录：

```text
"POST /responses HTTP/1.1" 200 ... "Codex Desktop/0.142.0-alpha.1 ..."
```

数据库状态正常：

```text
api_key_id: 1
group: Codex
group platform: openai
OpenAI OAuth accounts: 3 active/schedulable
proxy_id: 1 (mihomo-openai)
```

但当时从本机直连旧入口域名时，DNS 被解析到异常地址：

```bash
dig +short <旧入口域名> A
```

结果：

```text
28.0.0.34
```

强制连接真实服务器 IP 后，TLS 仍在握手阶段被断开：

```bash
curl -vI --connect-to <旧入口域名>:443:39.102.86.235:443 \
  https://<旧入口域名>/health
```

结果：

```text
Connected to 39.102.86.235:443
TLS handshake Client hello
LibreSSL SSL_connect: SSL_ERROR_SYSCALL
```

直接用 HTTP 带 Host 访问真实 IP，阿里云返回未备案拦截页：

```bash
curl -sS --connect-timeout 5 http://39.102.86.235/health \
  -H 'Host: <旧入口域名>' -D -
```

结果摘要：

```text
HTTP/1.1 403 Forbidden
Server: Beaver
<title>Non-compliance ICP Filing</title>
http://www.aliyun.com/beian/beian-block
```

这说明大陆直连入口被阿里云备案策略拦截；HTTPS 场景下通常表现为 TLS 握手直接断开，客户端只能看到 `error sending request` / `stream disconnected`。

### 为什么浏览器可能还能打开后台

本机 macOS 当前启用了系统代理：

```text
HTTPProxy: 127.0.0.1:7890
HTTPSProxy: 127.0.0.1:7890
SOCKSProxy: 127.0.0.1:7890
```

浏览器可能走系统代理、TUN 或使用不同 TLS 指纹；而 Codex Desktop / curl / reqwest 这类客户端可能走直连，或在 Clash 规则中被 `GEOIP,CN,DIRECT` 判定为直连。于是浏览器能加载后台，不代表 Codex Desktop 的 `/responses` 请求一定能到达服务器。

### 立即可用的临时方案

在 ClashX Meta / Mihomo 规则里把旧入口链路强制走代理节点，不要 DIRECT：

```yaml
rules:
  - DOMAIN,portal.lizubin.online,PROXY
  - DOMAIN-SUFFIX,lizubin.online,PROXY
  - IP-CIDR,39.102.86.235/32,PROXY,no-resolve
```

`PROXY` 需要替换成实际策略组名称，例如 `OpenAI`、`Proxy`、`GLOBAL` 或机场配置里的节点组名。规则必须放在 `GEOIP,CN,DIRECT` 和 `MATCH,DIRECT` 前面。

如果 Codex Desktop 不遵循系统代理，优先打开 ClashX Meta 的 TUN 模式；Windows 客户端可用 Clash Verge/Mihomo TUN 或 Proxifier，把 Codex Desktop 进程强制走代理。

验证命令：

```bash
curl -vI --proxy http://127.0.0.1:7890 https://portal.lizubin.online/health
```

如果仍然握手失败，说明本机代理规则仍把目标域名或 `39.102.86.235` 走了 DIRECT，需要继续调整规则或切到全局代理验证。

### 长期修复方案

大陆云服务器对外提供 HTTPS 站点，域名需要完成 ICP 备案并接入到对应云厂商；否则不同客户端、不同网络下会出现 DNS 污染、HTTP 备案拦截页、HTTPS 握手中断等不稳定现象。

可选长期方案：

1. 给 `lizubin.online` 或实际 API 域名完成 ICP 备案，并按阿里云要求接入备案。
2. 将 Sub2API 迁移到香港、新加坡、日本、美国等非大陆服务器。
3. 使用已备案域名/CDN/反向代理作为公网入口，再回源到当前服务。

在备案或迁移完成前，Codex Desktop 用户侧必须确保请求走可用代理/TUN，否则会继续在进入 Nginx 前断开。

## Codex Desktop 本地入口链路三次排查：2026-06-21

### 新增确认

这次在远端和本机同时复查后，继续确认问题不在 Sub2API 应用、Nginx `/responses` 路由、OpenAI 账号测试接口或 Mihomo 出站链路，而在本机到 `portal.lizubin.online` 的入口链路。

服务器侧状态：

```text
sub2api-mihomo Up healthy
sub2api        Up healthy
sub2api-postgres Up healthy
sub2api-redis    Up healthy
```

远端服务器本机访问公网域名健康检查正常：

```bash
ssh my2g 'curl -sS -D - --connect-timeout 8 --max-time 20 https://portal.lizubin.online/health'
```

结果：

```text
HTTP/1.1 200 OK
{"status":"ok"}
```

远端 Nginx 最近的 `/responses` 记录里，成功请求仍是 `HTTP 200`；新出现的 `bruno-runtime` 请求是 `301`，且没有进入 Sub2API 应用日志，不属于 Codex Desktop 的流式失败：

```text
[20/Jun/2026:23:58:35 +0800] "POST /responses HTTP/1.1" 200 ... "Codex Desktop/0.142.0-alpha.1 ..."
[20/Jun/2026:23:58:36 +0800] "POST /responses HTTP/1.1" 200 ... "Codex Desktop/0.142.0-alpha.1 ..."
[21/Jun/2026:00:13:30 +0800] "POST /responses HTTP/1.1" 301 ... "bruno-runtime/3.4.2"
```

本机直连失败仍可复现：

```bash
curl -sS -D - --connect-timeout 8 --max-time 20 https://portal.lizubin.online/health
```

结果：

```text
LibreSSL SSL_connect: SSL_ERROR_SYSCALL in connection to portal.lizubin.online:443
```

当时本机 DNS 仍解析到异常地址：

```bash
dig +short <旧入口域名> A
```

结果：

```text
28.0.0.34
```

即使强制连真实服务器 IP，HTTPS 也在 TLS ClientHello 后被断开：

```bash
curl -vI --connect-to <旧入口域名>:443:39.102.86.235:443 \
  https://<旧入口域名>/health
```

结果摘要：

```text
Connected to 39.102.86.235:443
TLS handshake Client hello
LibreSSL SSL_connect: SSL_ERROR_SYSCALL
```

HTTP 明文带 Host 访问真实 IP，会被阿里云备案拦截：

```bash
curl -sS --connect-timeout 8 --max-time 20 -D - \
  http://39.102.86.235/health \
  -H 'Host: portal.lizubin.online'
```

结果摘要：

```text
HTTP/1.1 403 Forbidden
Server: Beaver
<title>Non-compliance ICP Filing</title>
```

### 本机 ClashX Meta 现状

本机 macOS 系统代理已打开：

```text
HTTPProxy:  127.0.0.1:7890
HTTPSProxy: 127.0.0.1:7890
SOCKSProxy: 127.0.0.1:7890
```

代理端口本身是可用的。通过同一个代理访问 OpenAI 可以返回正常的未认证响应：

```bash
curl -I --proxy http://127.0.0.1:7890 https://api.openai.com/v1/models
```

结果：

```text
HTTP/2 401
server: cloudflare
```

但通过同一个代理访问 `portal.lizubin.online` 仍然 TLS 断开：

```bash
curl -vI --proxy http://127.0.0.1:7890 https://portal.lizubin.online/health
```

结果：

```text
HTTP/1.1 200 Connection established
LibreSSL SSL_connect: SSL_ERROR_SYSCALL
```

当前 ClashX Meta 选中配置：

```text
selectConfigName: 魔戒.net
selectOutBoundMode: rule
proxyPort: 7890
restoreTunProxy: true
```

该配置里已有 `ChatGPT` 策略组，并且覆盖了 `openai.com`、`chatgpt.com` 等域名；但没有覆盖当前 Sub2API 入口域名：

```text
portal.lizubin.online
lizubin.online
39.102.86.235
```

因此会出现：

```text
api.openai.com -> 命中 ChatGPT 代理 -> 正常
portal.lizubin.online -> 命中 GEOIP,CN/DIRECT 或污染解析 -> TLS/备案拦截失败
```

### 当前结论

管理员账号测试接口成功：

```text
POST /api/v1/admin/accounts/1/test
```

只证明这条链路正常：

```text
本机/远端 curl
  -> portal.lizubin.online
  -> Sub2API 管理接口
  -> account proxy_id=1
  -> Mihomo
  -> chatgpt.com/backend-api/codex/responses
```

Codex Desktop 基础请求失败的是另一条链路：

```text
Codex Desktop
  -> https://portal.lizubin.online/responses
  -> 本机 DNS/Clash 规则/大陆入口
  -> 未进入 Nginx 或在 TLS 阶段断开
```

所以两者不矛盾：账号上游测试成功，不代表客户端到站点入口可达。

### 最小修复建议

在当前 ClashX Meta 选中配置 `魔戒.net` 的 `rules:` 顶部，放到 `GEOIP,CN,DIRECT` 和 `MATCH,DIRECT` 前，添加：

```yaml
rules:
  - DOMAIN,portal.lizubin.online,ChatGPT
  - DOMAIN-SUFFIX,lizubin.online,ChatGPT
  - IP-CIDR,39.102.86.235/32,ChatGPT,no-resolve
```

如果 `ChatGPT` 组当前节点不可用，也可以先改成：

```yaml
rules:
  - DOMAIN,portal.lizubin.online,节点选择
  - DOMAIN-SUFFIX,lizubin.online,节点选择
  - IP-CIDR,39.102.86.235/32,节点选择,no-resolve
```

然后在 ClashX Meta 中重新加载配置，或临时切到全局模式验证。

验证标准：

```bash
curl -vI --proxy http://127.0.0.1:7890 https://portal.lizubin.online/health
```

期望结果：

```text
HTTP/1.1 200 Connection established
HTTP/1.1 200 OK
{"status":"ok"}
```

如果这个命令仍然 TLS 断开，说明当前代理出口访问阿里云大陆入口也被拦截或仍然被 DIRECT，需要：

1. 切换 ClashX Meta 到全局模式验证。
2. 手动选择香港/新加坡/日本等境外节点验证。
3. 开启 TUN，让 Codex Desktop 进程也被接管。
4. 长期上把站点迁到非大陆服务器，或给域名完成 ICP 备案。

## 面向用户交付的较小闭环结论：2026-06-21

### 核心结论

不要把“指导用户修改 ClashX Meta / Clash Verge / Mihomo 规则”作为正式交付方案。

原因：

- 普通用户不应被要求理解 `GEOIP,CN,DIRECT`、TUN、策略组名、规则顺序等代理细节。
- 不同用户使用的代理客户端、规则集、节点质量都不同，客服成本不可控。
- 只要当前入口仍是“未备案域名 + 阿里云大陆 ECS”，不同客户端和网络环境下仍可能出现 DNS 污染、HTTP 备案拦截、HTTPS TLS 握手断开。
- 用户即使“开着翻墙”，规则模式也可能因为 `39.102.86.235` 是中国 IP 而直连；因此“开代理”不等于“这个域名实际走代理”。

当前正式问题不是 Sub2API 后端问题，而是公网入口问题：

```text
用户 Codex Desktop
  -> https://portal.lizubin.online/responses
  -> 阿里云大陆未备案域名入口
  -> TLS 握手阶段断开 / HTTP 备案拦截
  -> 请求没有进入 Nginx/Sub2API
```

服务端出站代理是另一段链路：

```text
Sub2API
  -> account proxy_id
  -> Mihomo
  -> chatgpt.com / api.openai.com
```

这两段互不替代。管理员账号测试接口成功，只能证明服务端出站链路可用，不能证明用户到站点入口可用。

### Ping 与 HTTP/HTTPS 判断

`ping <旧入口域名>` 成功不代表 Codex Desktop 可用。

`ping` 只验证 ICMP 到 IP 可达：

```text
<旧入口域名> -> 39.102.86.235
ICMP echo reply OK
```

Codex Desktop 实际需要的是：

```text
TCP 443
TLS SNI: portal.lizubin.online
HTTP POST /responses
Authorization: Bearer sk-...
SSE stream
```

本次 Windows 客户端已经确认：

```text
curl.exe -vI https://portal.lizubin.online/health
schannel: failed to receive handshake, SSL/TLS connection failed
```

这说明失败发生在 TLS 握手阶段，HTTP 请求头、API key 和 `/responses` 请求体都还没有发出去。

### 不建议改成 HTTP

不要通过去掉 HTTPS、改用 HTTP 来规避。

原因：

- HTTP 明文带 Host 访问真实 IP 已经返回阿里云备案拦截：

```text
HTTP/1.1 403 Forbidden
Server: Beaver
<title>Non-compliance ICP Filing</title>
```

- HTTP 会明文传输用户 API key：

```text
Authorization: Bearer sk-...
```

- 现代客户端或后续安全策略可能拒绝非 HTTPS API base URL。
- HTTP 只会把“TLS 握手断开”变成“备案拦截页”，不能解决正式访问问题。

### 推荐最小闭环

优先做一个海外 HTTPS 入口，不要求用户改代理。

推荐链路：

```text
用户 Codex Desktop
  -> https://codex-hk.lizubin.online
  -> 海外 VPS / 海外反代入口
  -> Sub2API
  -> Mihomo 出站代理
  -> OpenAI/ChatGPT
```

更稳的最小闭环是完整迁移：

```text
香港/新加坡/日本 VPS
  -> Nginx/Caddy
  -> Sub2API
  -> PostgreSQL
  -> Redis
  -> Mihomo
```

这样用户只需要配置一个正常的 HTTPS base URL，不需要关心代理软件规则。

### 可选落地路径

方案 A：新增海外 VPS 并完整迁移 Sub2API。

```text
codex-hk.lizubin.online
  -> overseas VPS public IP
  -> Nginx/Caddy TLS
  -> sub2api:8080
```

优点：

- 链路最短。
- 用户侧最稳定。
- 不再依赖大陆入口备案状态。
- 不需要客户改代理。

方案 B：海外 VPS 只做 HTTPS 反代入口，回源到当前 my2g。

```text
codex-hk.lizubin.online
  -> overseas VPS Nginx/Caddy
  -> WireGuard/SSH tunnel/private tunnel
  -> my2g sub2api:8080
```

优点：

- 迁移量较小。
- 可以先验证用户入口是否恢复。

缺点：

- 链路更长。
- 多一个隧道组件。
- 仍需要维护 my2g 与海外入口之间的稳定性。

方案 C：继续使用阿里云大陆入口，但完成 ICP 备案并接入备案。

优点：

- 可以保留当前大陆 ECS。

缺点：

- 周期较长。
- 需要满足备案主体、域名、服务器接入等要求。
- 不适合作为立即恢复 Codex Desktop 用户访问的方案。

### 临时排障方案

临时自测或内部用户可以要求代理规则强制走代理：

```yaml
- DOMAIN,portal.lizubin.online,PROXY
- DOMAIN-SUFFIX,lizubin.online,PROXY
- IP-CIDR,39.102.86.235/32,PROXY,no-resolve
```

规则必须放在：

```text
GEOIP,CN,DIRECT
MATCH,DIRECT
```

之前。

但这只适合内部排障，不适合面向注册用户交付。

### 最终建议

当前较小闭环最佳实践：

```text
1. 保留 my2g 作为现有服务和管理环境。
2. 新增海外 HTTPS 入口域名，例如 codex-hk.lizubin.online。
3. 优先完整迁移 Sub2API 到海外 VPS；若要更快验证，则先用海外 VPS 反代回 my2g。
4. 用户 Codex Desktop 统一切换到海外入口。
5. 大陆入口后续再决定是否备案或废弃。
```

正式用户不应依赖修改 ClashX Meta 配置来使用服务。
