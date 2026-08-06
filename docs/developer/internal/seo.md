# 本站 SEO 最小闭环

## 目标与范围

当前站点是 Go 内嵌的 Vue SPA。本闭环解决三类问题：

1. `index.html` 缺少 description、robots、Open Graph 基线；
2. 路由级标题已有，但 description/canonical/robots 未随路由同步，且 `/robots.txt`、`/sitemap.xml` 会命中 SPA 回退返回 HTML，对爬虫反而有害；
3. 未登录页面（登录、注册、OAuth 回调、后台、404）缺少 noindex 保护。

本站品牌与定位：**Kaka Codex，以 Codex 为主的 AI 开发工具站与交流社区**。
默认标题、description、Open Graph 与首页文案均按此口径维护；站点名/副标题仍由后台设置注入，代码默认值仅作无配置时的回退。

收录策略为 fail-closed：**只有路由显式标记 `indexable` 的页面允许收录**，其余一律 `noindex, follow`。

当前可收录页面：

| 路径 | 说明 |
| --- | --- |
| `/home` | 首页（`/` 重定向到 `/home`） |
| `/model-plaza` | 模型广场 |
| `/legal/:documentId` | 法律文档（terms、usage-policy 等） |

## 机制

### 前端：路由级 SEO 同步

- `frontend/index.html`：静态基线 description、robots、Open Graph、twitter card；站点名/Logo 仍由后端按设置注入。
- `frontend/src/router/seo.ts`：`applyRouteSeo` 统一维护 `document.title`、`meta[name=description]`、`meta[name=robots]`、`link[rel=canonical]`、`og:*`；`syncCurrentRouteSeo` 是唯一同步入口，`router.afterEach` 与语言切换 `setLocale` 都只调用它。
- `frontend/src/constants/site.ts`：品牌名与标题后缀的单一来源；`frontend/index.html` 静态标题与后端 `injectSiteTitle` 为跨语言副本，改动时需同步。
- 路由 meta（`frontend/src/router/meta.d.ts`）：`indexable`（允许收录）、`noindex`（强制不收录）、`descriptionKey`/`description`（描述来源，优先 i18n 翻译）。

### 后端：按请求 Origin 动态生成 robots/sitemap

- `backend/internal/web/seo.go`：`robots.txt` 声明公开内容并排除登录/注册/回调/后台/API 路径；`sitemap.xml` 列出 `publicIndexablePaths`，URL 使用请求的 `scheme://host`，多域名部署各 Origin 自动正确。
- `backend/internal/web/embed_on.go`：`FrontendServer.Middleware()` 与 legacy `ServeEmbeddedFrontend()` 在 SPA 回退前处理这两个路径；`data/public` 下的同名覆盖文件优先级更高。

## 新增可收录页面

两处同步，缺一不可：

1. 前端路由 meta 加 `indexable: true`（首页/广场/法律文档已标记）；
2. `backend/internal/web/seo.go` 的 `publicIndexablePaths` 追加对应路径。

## 运维说明

- 无需配置域名；robots/sitemap 按请求 Host 生成。
- 如需自定义 robots/sitemap（例如只想收录某个子域名），在 `data/public/` 放置同名文件即可覆盖。

## 验证

```bash
go test -tags=embed ./internal/web -count=1
pnpm --dir frontend run typecheck
pnpm --dir frontend exec vitest run src/router/__tests__/seo.spec.ts src/router/__tests__/title.spec.ts
git diff --check
```

## 已知限制与后续

- 无 SSR：不支持 JS 的爬虫只能看到 `index.html` 基线，路由级 description/canonical 依赖搜索引擎执行 JS（Google/Bing 支持，百度部分支持）。
- 法律文档页依赖管理端配置的文档 ID，暂不进 sitemap，避免死链；后续可改为按设置生成。
- 结构化数据（JSON-LD）、hreflang、逐页 OG 图暂未纳入，需要时按同一接入点扩展。
