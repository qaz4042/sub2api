# OpenAI 代理延迟

独立展示管理员代理到 `https://api.openai.com/v1/models` 的连接延迟，不使用 API Key、不发起模型生成请求。HTTP 200、401 判为可达；403、429、5xx 等均为失败。复用现有代理目标探测器，超时沿用其 10 秒设置；本功能总探测上下文上限 12 秒。

## 公共接入点

- `ProxiesView.vue`：导入本 feature 的行组件、批量按钮与页面级 provider，添加列与单元格 slot。
- 中英文 `admin/resources.ts`：各添加一个列名。
- 后端 `server/routes/admin.go`：在既有管理员代理路由组调用 `RegisterOpenAILatency`。

其余实现集中在本目录和后端各层的 `proxy_openai_latency.go`，通过可选 service/cache contract 接入，不扩大 `AdminService`、`ProxyLatencyCache` 核心接口，不修改原来的测试、质量检测、代理 DTO 或历史 migration。

## 批量测试

页面顶部“批量测试 OpenAI”优先测试勾选的代理，未勾选时测试当前页；按钮数量与提示明确展示范围，不会自动扫描所有页。开始后固定本次 ID 列表，去重并以最多 3 个 worker 复用既有 POST 接口；单项异常不会中断其余测试。

每行仅展示延迟／失败／测试中状态，不提供测试按钮；统一使用顶部批量测试入口。批量按钮与行组件通过 feature 内页面级 provider 共享状态。结果逐条更新，进度与成功／失败数同步变化；进行中的单条请求会复用，历史读取不会覆盖较新的测试结果。翻页不会取消整个批次，离开页面后取消在途请求并停止发出剩余请求。没有增加后端批量路由或修改核心代理 API。

## API 与缓存

- `GET /api/v1/admin/proxies/:id/openai-latency`：只读取历史结果，无结果返回 `data: null`。
- `POST /api/v1/admin/proxies/:id/openai-latency`：触发测速并缓存成功或失败结果。
- 两个接口继承既有管理员鉴权，先校验代理存在。
- Redis 独立前缀 `proxy:openai-latency:`，TTL 24 小时；完整连接配置的 SHA-256 指纹用于避免地址或凭证变更后复用旧结果。不保存代理明文凭证，不覆盖通用延迟缓存。
- 页面按行读取历史结果，测试后即时更新本列；代理切换、配置变化或组件卸载时取消旧请求并忽略过期响应。

## 后续合并

优先保留上游代理管理、测速与质量检测实现，只重新适配上述接入点。若上游增加等价能力，可整体删除本 feature 与独立后端文件，无需回滚数据库。功能按单一意图提交，避免混入账号统计等无关改动。
