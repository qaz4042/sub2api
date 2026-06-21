# `Non-compliance ICP Filing` 拦截问题分析与处理

更新日期：2026-06-21

适用对象：旧入口域名、阿里云中国内地 ECS、Sub2API、Codex Desktop

## 1. 结论

故障发生时，旧入口域名指向阿里云中国内地公网 IP `39.102.86.235`，但该域名尚未完成与中国内地接入商相匹配的 ICP 备案或接入备案。

本次已经从客户端侧观测到：

```html
<title>Non-compliance ICP Filing</title>
```

并且响应包含：

```text
HTTP/1.1 403 Forbidden
Server: Beaver
```

结合以下事实，可以将当前用户访问失败定位为阿里云中国内地入口的备案检查或拦截，而不是 Sub2API 应用故障：

- Windows 客户端能够解析并 `ping` 通 `39.102.86.235`。
- Windows `curl.exe` 在访问 HTTPS 时于 TLS 握手阶段失败。
- 使用 HTTP 携带正确 `Host` 访问时，返回 `Non-compliance ICP Filing`。
- 失败请求没有进入服务器 Nginx 和 Sub2API 日志。
- 服务器内部和已能到达入口的环境中，`/health`、`/responses` 以及管理员账号测试均可正常完成。

因此，正式修复必须处理公网入口：

1. 将用户入口迁移到中国香港或其他境外服务器，并继续使用域名和 HTTPS；或
2. 保留阿里云中国内地入口，为域名完成 ICP 备案及阿里云接入备案。

修改 Sub2API、API key、模型、SSE 超时或服务器出站 Mihomo，均不能解决这个入口拦截。

## 2. 官方规则与平台行为

《非经营性互联网信息服务备案管理办法》规定，在中华人民共和国境内提供非经营性互联网信息服务，应履行备案手续。

阿里云官方文档进一步说明：

- 域名解析到中国内地服务器并对外提供 Web 服务时，需要完成 ICP 备案。
- 未备案域名直接解析到阿里云中国内地服务器时，可能被阿里云监测系统识别并阻断访问。
- 域名解析到非中国内地服务器，例如中国香港服务器时，不要求办理工信部 ICP 备案。
- 即使域名已有工信部备案，改用阿里云中国内地服务器时，仍可能需要在阿里云办理新增接入。

这里需要区分两个问题：

```text
访问层问题：未备案域名被中国内地云厂商阻断
业务合规问题：当前业务是否只需备案，还是还涉及许可、公安联网备案等要求
```

完成 ICP 备案可以解决当前中国内地云入口的基础备案检查，但不应自动理解为业务的全部合规事项已经完成。Sub2API 如果面向公众注册、收费或提供经营性服务，应根据实际运营主体、收费模式和服务内容，向接入商备案顾问或专业合规人员确认是否还涉及其他许可或备案要求。

## 3. 本次故障链路

Codex Desktop 正常请求链路应为：

```text
Codex Desktop
  -> DNS 解析 portal.lizubin.online
  -> TCP 连接 39.102.86.235:443
  -> TLS 握手，SNI=portal.lizubin.online
  -> POST /responses
  -> Nginx
  -> Sub2API
  -> 账号绑定的 Mihomo 出站代理
  -> OpenAI/ChatGPT 上游
```

本次实际链路为：

```text
Codex Desktop
  -> DNS 解析成功
  -> TCP 443 开始连接
  -> 阿里云入口执行域名备案检查
  -> TLS 握手被中止
  -> HTTP 请求尚未发送
  -> Nginx/Sub2API 无请求日志
```

客户端最终只能报告较上层的网络错误：

```text
stream disconnected before completion:
error sending request for url (https://portal.lizubin.online/responses)
```

该错误不是 SSE 流传输到一半才断开。根据本次 `curl.exe` 结果，请求实际上在 TLS 阶段就失败了。

## 4. 为什么其他测试正常

### 4.1 `ping` 正常不代表 HTTPS 正常

`ping` 使用 ICMP，只能证明 IP 基本可达：

```text
域名解析 -> IP -> ICMP Echo Reply
```

HTTPS 还需要完成：

```text
TCP 443 -> TLS/SNI -> 证书校验 -> HTTP -> SSE
```

备案拦截可以在 HTTP Host 或 TLS SNI 层发生，所以完全可能出现“能 ping，不能 HTTPS”。

### 4.2 管理员账号测试成功不代表用户入口正常

管理员测试：

```text
POST /api/v1/admin/accounts/1/test
```

成功主要证明：

```text
Sub2API
  -> 账号配置
  -> Mihomo
  -> OpenAI/ChatGPT
```

这属于服务器出站链路。

Codex Desktop 失败发生在：

```text
用户客户端
  -> portal.lizubin.online
  -> 阿里云中国内地公网入口
```

这属于服务器入站链路。两条链路相互独立，所以测试结果并不矛盾。

### 4.3 服务器本机访问正常不代表公网用户正常

云服务器本机访问自己的域名可能经过不同的 DNS、路由、回环或云内路径，未必触发与公网访问完全相同的检查。是否对用户可用，应以服务器外部客户端的实际 TLS 和 HTTP 结果为准。

### 4.4 开着代理仍可能失败

Clash/Mihomo 的规则模式可能把中国 IP 命中为：

```text
GEOIP,CN,DIRECT
```

所以“代理客户端已启动”不等于 `portal.lizubin.online` 一定通过境外节点。浏览器、`curl.exe` 和 Codex Desktop 也可能分别采用不同的系统代理、TUN 或直连路径。

要求每个注册用户修改代理规则只能作为排障手段，不能作为正式服务方案。

## 5. 为什么常见规避方式不成立

### 5.1 改成 HTTP

不可作为正式方案。

- 本次 HTTP 已明确返回备案拦截页。
- `Authorization: Bearer ...` 和请求内容会以明文传输。
- 中间网络可以读取或篡改 API key、模型请求和响应。
- Codex Desktop 或其他客户端可能限制非 HTTPS API 地址。

HTTP 只会把 HTTPS 的握手失败变成可见的备案拦截页，不会解除拦截。

### 5.2 直接使用公网 IP

不可作为正式方案。

- HTTPS 证书通常签发给域名，而不是 `39.102.86.235`，会发生证书名称不匹配。
- TLS SNI、Nginx 虚拟主机和上游路由均依赖域名。
- HTTP 使用 IP 时仍可能根据 `Host`、IP 所属云资源或平台检查被拦截。
- IP 变更会直接导致所有客户端配置失效。

不要关闭证书校验来迁就 IP 地址，这会破坏 HTTPS 的身份认证。

### 5.3 用户关闭代理

通常不会解决，反而会确保请求直连中国内地入口，继续触发当前问题。

只有完成备案、迁移入口或确认某条网络路径不受该拦截影响后，关闭代理才可能正常。它不是可重复、可交付的修复。

### 5.4 用户二次翻墙

下面两段代理不是“二次代理冲突”：

```text
用户代理：用户 -> Sub2API 公网入口
服务端 Mihomo：Sub2API -> OpenAI/ChatGPT
```

它们处理不同网络段，可以同时存在。当前问题是用户侧到中国内地入口的路由和备案拦截，不是服务端 Mihomo 多代理了一次。

### 5.5 更换证书或调整 Nginx TLS

当前证书和 Nginx 在能够到达服务器的环境中已经工作。备案拦截发生在请求进入 Nginx 之前，因此重新签发相同域名证书、调整 TLS 版本或增加 Nginx 超时不会消除平台入口检查。

## 6. 推荐方案

### 方案 A：完整迁移到境外 VPS

推荐优先级：最高。

```text
用户
  -> HTTPS 域名
  -> 香港/新加坡/日本 VPS
  -> Nginx/Caddy
  -> Sub2API
  -> PostgreSQL/Redis
  -> OpenAI/ChatGPT
```

优点：

- 用户无需修改代理软件。
- 链路最短，故障边界清晰。
- 不再依赖中国内地云入口的 ICP 检查。
- HTTPS、SSE 和 Codex Desktop 使用方式保持标准。

注意：

- 境外入口到中国内地用户的网络质量需要实际测试。
- 迁移数据库前应停止写入或设计可验证的数据迁移窗口。
- 应保留回滚方案和旧入口，但不要让两个实例同时写同一份未同步数据。

### 方案 B：境外 VPS 做 HTTPS 入口，私网回源 my2g

推荐优先级：适合作为较快验证闭环。

```text
用户
  -> 境外 HTTPS 入口
  -> WireGuard/其他受控隧道
  -> my2g 内部 Sub2API
```

优点：

- 无需立即迁移数据库和应用状态。
- 可以快速验证 Codex Desktop 是否恢复。

缺点：

- 比完整迁移多一段跨境回源。
- SSE 长连接同时依赖境外入口、隧道和 my2g。
- 隧道断开时需要监控和自动恢复。

不要把 my2g 的 `8080` 直接暴露到公网作为回源。应使用 WireGuard、受限安全组或双向认证等方式限制入口。

### 方案 C：保留阿里云中国内地入口并完成备案

推荐优先级：适合必须长期保留中国内地入口的情况。

大致步骤：

1. 确认域名实名信息与备案主体一致。
2. 确认当前阿里云 ECS 满足备案服务条件。
3. 在阿里云 ICP 备案系统提交首次备案或新增互联网信息服务。
4. 按要求完成负责人核验、短信核验和管局审核。
5. 如果域名已在其他接入商备案，确认是否需要阿里云新增接入。
6. 备案通过后，将域名解析和实际服务保持在备案信息对应的接入环境。
7. 按实际业务继续确认公安联网备案及其他可能适用的许可要求。

该方案不是即时修复。审核周期、材料和具体规则以备案主体所在地通信管理局及阿里云备案系统当时要求为准。

## 7. 推荐的较小闭环

若目标是尽快恢复注册用户的 Codex Desktop：

```text
第一阶段
  1. 新建境外 VPS。
  2. 使用新子域名，例如 codex-hk.lizubin.online。
  3. 配置正常的域名证书和 HTTPS。
  4. 先通过私网隧道反代回 my2g。
  5. 用真实用户 API key 验证 /health 和 /responses SSE。

第二阶段
  1. 将 Sub2API、PostgreSQL、Redis 和必要配置完整迁移到境外。
  2. 切换正式域名或稳定保留新的境外域名。
  3. 移除对用户本地 Clash 规则的依赖。

后续
  1. 根据是否保留中国内地入口，决定办理备案或下线该入口。
```

正式交付标准应是：

```text
用户只配置 HTTPS base URL 和 API key
不要求关闭代理
不要求开启代理
不要求修改 Clash/Mihomo 规则
```

## 8. 验证与证据采集

### 8.1 Windows 外部客户端

检查 DNS：

```bat
nslookup portal.lizubin.online
```

检查 TLS 和健康接口：

```bat
curl.exe -vk https://portal.lizubin.online/health
```

检查流式接口：

```bat
curl.exe -vN https://portal.lizubin.online/responses ^
  -H "Authorization: Bearer sk-your-user-key" ^
  -H "Content-Type: application/json" ^
  -H "Accept: text/event-stream" ^
  --data "{\"model\":\"gpt-5.4-mini\",\"input\":\"ping\",\"stream\":true}"
```

安全注意：排障输出对外共享前，必须删除完整 API key、Cookie 和 Authorization 头。

### 8.2 HTTP 备案拦截证据

仅用于诊断，不用于传输真实 API key：

```bash
curl -v http://39.102.86.235/ \
  -H 'Host: portal.lizubin.online'
```

若出现以下内容，说明请求被入口层返回备案拦截页：

```text
HTTP/1.1 403 Forbidden
Server: Beaver
<title>Non-compliance ICP Filing</title>
```

### 8.3 服务器日志关联

在发起客户端请求的同时检查：

```bash
ssh my2g 'sudo tail -f /var/log/nginx/access.log /var/log/nginx/error.log'
```

判断方法：

- 客户端失败且 Nginx 完全没有记录：优先排查 DNS、TCP、TLS、云平台入口和客户端代理。
- Nginx 有 `4xx/5xx`：请求已到服务器，再排查 Nginx 或认证。
- Nginx 返回 `200`，应用流中断：再排查 Sub2API、SSE、上游和超时。

## 9. 迁移或备案后的验收标准

至少从两个不同网络进行验证，例如中国电信家庭宽带和手机热点：

```text
[ ] DNS 稳定解析到预期入口
[ ] TLS 证书链有效，域名匹配
[ ] GET /health 返回 HTTP 200
[ ] POST /responses 能建立 SSE
[ ] 流中包含完成事件，而非中途断开
[ ] 请求进入 Nginx 和 Sub2API 日志
[ ] Codex Desktop 不依赖特定 Clash 规则
[ ] 连续请求和长响应均正常
[ ] API key 未出现在公开日志或排障文档
```

建议同时保留入口监控：

```text
DNS 解析
TCP 443
TLS 证书有效期
GET /health
POST /responses 的合成流式测试
Nginx 5xx 和上游超时
```

## 10. 决策表

| 方案 | 能否解决当前拦截 | 用户需改代理 | 安全性 | 实施速度 | 长期维护 |
| --- | --- | --- | --- | --- | --- |
| HTTP + IP | 否，且已出现拦截页 | 不确定 | 不可接受 | 快 | 不可用 |
| HTTPS + IP 并关闭校验 | 不可靠 | 不确定 | 不可接受 | 快 | 不可用 |
| 要求用户强制走境外代理 | 可能临时绕过 | 是 | 取决于用户环境 | 快 | 成本高 |
| 境外 HTTPS 入口回源 my2g | 是 | 否 | 可做好 | 较快 | 中等 |
| 完整迁移境外 VPS | 是 | 否 | 可做好 | 中等 | 最清晰 |
| 阿里云中国内地完成备案 | 是 | 否 | 正常 | 较慢 | 适合保留内地入口 |

## 11. 官方参考

- [工信部 ICP/IP 地址/域名信息备案管理系统](https://beian.miit.gov.cn/)
- [非经营性互联网信息服务备案管理办法](https://www.cac.gov.cn/2005-02/09/c_1112147171.htm)
- [阿里云：什么是 ICP 备案](https://help.aliyun.com/zh/icp-filing/basic-icp-service/)
- [阿里云：未备案域名解析至不同地区是否可以访问](https://help.aliyun.com/zh/icp-filing/not-for-the-record-dns-can-access-to-different-areas)
- [阿里云：ICP 备案流程](https://help.aliyun.com/zh/icp-filing/basic-icp-service/user-guide/icp-filing-application-overview)
- [阿里云：不同场景下的 ICP 备案说明](https://help.aliyun.com/zh/icp-filing/basic-icp-service/product-overview/faq-about-icp-filing-applications-in-different-scenarios)

## 12. 最终判断

`Non-compliance ICP Filing` 是本次故障的关键证据。它说明当前“未备案域名指向阿里云中国内地服务器并对外提供 Web/API 服务”的入口形态不可作为稳定的正式用户入口。

当前最实用的处理顺序是：

```text
短期：境外 HTTPS 入口回源 my2g
中期：完整迁移 Sub2API 到境外 VPS
长期：若继续使用中国内地入口，则完成对应备案和接入手续
```

无论选择哪条路径，都应继续使用“域名 + 有效 HTTPS 证书”，不要通过 HTTP、裸 IP 或关闭证书校验规避。
