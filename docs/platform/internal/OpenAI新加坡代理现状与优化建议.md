# OpenAI 新加坡代理现状与优化建议

> 状态快照：2026-07-29。本文仅记录私有部署现状，不包含订阅密码、UUID 等敏感信息。

## 目标

1. OpenAI/Codex 流量尽可能保持新加坡出口。
2. 正常情况下优先选择低延迟线路。
3. 主线路异常时自动切换，减少 503、超时和流式中断。

## 故障结论

- 客户端出现 `503 Service temporarily unavailable` 时，仅凭响应文案无法直接判断是 sub2api 内部还是 OpenAI 上游返回，需结合 request ID 检查网关和上游日志。
- 本次排查未发现 sub2api、nginx、mihomo 持续异常；切换代理后 OpenAI `/responses` 连续返回 HTTP 200。期间出现过一次 HTTP/2 流中断，随后持续恢复，整体更符合临时代理链路或上游异常。
- 代理切换后已验证服务健康，未修改账号调度状态或业务核心逻辑。

## 地区判断纠正

- `7890` 是规则代理：只有 OpenAI 相关域名进入 `OpenAI-SG`，普通 IP 查询域名可能走 `DIRECT`。
- 因此，通过 `7890` 查询到香港 IP，不能据此判断 OpenAI 实际出口在香港。
- `新加坡SG-HY2` 经全局模式验证，Cloudflare 显示 `loc=SG`、`colo=SIN`，实际出口为新加坡。
- `新加坡SG-A-Gemini` 的入口服务器定位在香港，但最终出口为新加坡。其链路可能为“香港入口 -> 新加坡出口”，所以地区符合要求，但延迟高于 HY2。

## 当前代理配置

| 后台名称 | 端口 | 对应节点 | 状态 | 说明 |
|---|---:|---|---|---|
| `mihomo-openai` | 7890 | 规则模式 `OpenAI-SG` | active | 普通探测可能直连，不适合作为出口地区判断依据 |
| `openai-sg-mj-01-hy2` | 7891 | `新加坡SG-HY2` | active | 当前主线路 |
| `openai-sg-mj-02-anytls` | 7892 | `新加坡SG-A-Gemini` | active | 同国家备用线路 |
| `openai-sg-mj-03-vmess` | 7893 | 新加坡 VMess 1 | active | 高延迟、偶发超时 |
| `openai-sg-mj-04-vmess` | 7894 | 新加坡 VMess 2 | active | 高延迟、偶发超时 |
| `openai-sg-mj-05-vmess` | 7895 | 新加坡 VMess 3 | inactive | 冷备，当前不建议启用 |

当前 OpenAI 账号记录中，实际可调度账号仍绑定 `7891`；`7892` 虽已添加并启用，但目前不构成自动热备。

数据库中的代理 `fallback_mode` 用于代理到期后的迁移，不是运行时网络故障转移，不能依靠它处理临时断线。

## 实测结果

测试从 `my4g` 发起，主要观察 OpenAI 端点可达率和首包时间。HTTP 405 表示目标已成功响应，只是不接受测试所用的 HEAD 方法。

| 排名 | 节点 | 出口 | 典型表现 | 建议用途 |
|---:|---|---|---|---|
| 1 | `新加坡SG-HY2` | SG / SIN | IP 探测约 41-53ms；OpenAI 约 0.16-0.35s | 主节点 |
| 2 | `新加坡SG-A-Gemini` | SG / SIN | IP 探测热连接约 97-101ms；OpenAI 约 0.53-0.72s | 同国家热备 |
| 3 | `日本JP-HY2` | JP / NRT | 3/3 成功，约 0.16-0.27s | 人工跨地区灾备 |
| 4 | `日本JP-A` | JP / NRT | 3/3 成功，约 0.26-0.28s | 人工跨地区灾备 |
| 5 | `英国-优化-GPT` | GB / LHR | 3/3 成功，中位约 1s | 末级人工灾备 |

其他新加坡 VMess 节点多次出现约 3-10s 延迟或超时；当前 `OpenAI-SG` 组中的两条 QM 新加坡节点，OpenAI 健康测试均未通过。因此，不应仅根据节点名称中的“优化”或“GPT”判断质量。

## 推荐的较小闭环

最高性价比方案是不新增复杂服务，先用 mihomo 原生 `fallback` 形成固定的新加坡主备：

1. 新建固定入口 `openai-sg-auto`，例如端口 `7896`，直接绑定 `OpenAI-SG` 组。
2. `OpenAI-SG` 只保留并按顺序使用：`新加坡SG-HY2`、`新加坡SG-A-Gemini`。
3. 将健康检查周期由 300 秒缩短为 60 秒，超时设为约 5 秒。
4. 将实际生产账号绑定到 `openai-sg-auto`。
5. 保留 `7891`、`7892` 固定入口，作为诊断和人工回滚通道。
6. 日本节点不加入自动组，避免 OpenAI 会话自动改变国家；仅在两条新加坡线路都不可用时人工启用。

建议配置形态：

```yaml
listeners:
  - name: openai-sg-auto
    type: mixed
    port: 7896
    listen: 0.0.0.0
    proxy: OpenAI-SG

proxy-groups:
  - name: OpenAI-SG
    type: fallback
    proxies:
      - MJ-新加坡SG-HY2
      - MJ-新加坡SG-A-Gemini
    url: https://chatgpt.com/backend-api/codex/responses
    expected-status: 200-499
    interval: 60
    timeout: 5000
    lazy: false
```

该方案的预期行为：正常时固定走 HY2；HY2 检测失败后，新请求自动走 AnyTLS；两条线路出口均保持新加坡。已经建立的流式连接无法无缝迁移，仍可能失败一次，随后由重试请求使用备用线路。

## 后续增强

- 先观察上述主备方案 24-72 小时的成功率、超时率和首包延迟，再决定是否增加节点。
- 如需进一步降低单一服务商风险，优先购买一条不同服务商、不同 ASN、固定新加坡出口的线路，而不是继续增加同服务商的 VMess。
- 新节点进入自动组前，应至少验证 OpenAI 实际出口、连续可达率、长流稳定性和高峰期延迟。

## 运维信息

现有回滚文件：

- `/opt/sub2api/mihomo/config.yaml.codex-backup-20260728-151652`
- `/opt/sub2api/compose.binary-deps.yml.codex-backup-20260728-151907`

本文中的自动主备方案截至 2026-07-29 尚未部署。
