import { i18n } from '@/i18n'
import type { RouteLocationNormalized } from 'vue-router'
import type { CustomMenuItem } from '@/types'
import { DEFAULT_SITE_NAME } from '@/constants/site'
import { resolveRouteDocumentTitle } from './title'

/**
 * SEO 路由同步：统一维护 document.title、description、canonical、
 * Open Graph 和 robots，避免各页面各自写入产生覆盖冲突。
 *
 * 收录策略 fail-closed：只有路由 meta 显式声明 indexable 的页面允许
 * index，其余（登录、注册、回调、后台、404 等）一律 noindex。
 */

export const DEFAULT_SEO_DESCRIPTION =
  'Kaka Codex 是以 Codex 为主的 AI 开发工具站与交流社区：Codex 接入教程、CC Switch 配置、API Key 与 Base URL 管理、模型广场与开发者经验交流。'

type SEOAwareRoute = Pick<RouteLocationNormalized, 'path' | 'name' | 'params' | 'meta'>

function setMeta(attribute: 'name' | 'property', key: string, content: string): void {
  const selectorKey = key.replace(/"/g, '\\"')
  let meta = document.head.querySelector<HTMLMetaElement>(`meta[${attribute}="${selectorKey}"]`)
  if (!meta) {
    meta = document.createElement('meta')
    meta.setAttribute(attribute, key)
    document.head.appendChild(meta)
  }
  meta.setAttribute('content', content)
}

function setCanonical(url: string): void {
  let link = document.head.querySelector<HTMLLinkElement>('link[rel="canonical"]')
  if (!link) {
    link = document.createElement('link')
    link.setAttribute('rel', 'canonical')
    document.head.appendChild(link)
  }
  link.setAttribute('href', url)
}

/**
 * 规范化 canonical 路径：除根路径外去掉尾部斜杠，避免重复收录。
 */
export function canonicalPath(path: string): string {
  return path.length > 1 && path.endsWith('/') ? path.slice(0, -1) : path
}

/**
 * 解析页面描述：优先 i18n 翻译，其次静态 description，最后默认描述。
 */
export function resolveRouteDescription(route: Pick<SEOAwareRoute, 'meta'>): string {
  const descriptionKey = route.meta.descriptionKey
  if (typeof descriptionKey === 'string' && descriptionKey.trim()) {
    const translated = i18n.global.t(descriptionKey)
    if (translated && translated !== descriptionKey) {
      return translated
    }
  }

  if (typeof route.meta.description === 'string' && route.meta.description.trim()) {
    return route.meta.description.trim()
  }

  return DEFAULT_SEO_DESCRIPTION
}

/**
 * 是否允许搜索引擎收录：显式 indexable 且未强制 noindex。
 */
export function routeShouldBeIndexed(route: Pick<SEOAwareRoute, 'meta'>): boolean {
  return route.meta.indexable === true && route.meta.noindex !== true
}

/**
 * 路由变化后同步页面 SEO 元数据。
 */
export function applyRouteSeo(
  route: SEOAwareRoute,
  siteName: string | undefined,
  customMenuItems: CustomMenuItem[] = [],
): void {
  const title = resolveRouteDocumentTitle(route, siteName, customMenuItems)
  const description = resolveRouteDescription(route)
  const shouldIndex = routeShouldBeIndexed(route)
  const normalizedSiteName = typeof siteName === 'string' && siteName.trim() ? siteName.trim() : DEFAULT_SITE_NAME

  document.title = title
  setMeta('name', 'description', description)
  setMeta('name', 'robots', shouldIndex ? 'index, follow' : 'noindex, follow')
  setMeta('property', 'og:title', title)
  setMeta('property', 'og:description', description)
  setMeta('property', 'og:type', 'website')
  setMeta('property', 'og:site_name', normalizedSiteName)

  try {
    const parsedOrigin = new URL(window.location.origin)
    if (parsedOrigin.protocol === 'http:' || parsedOrigin.protocol === 'https:') {
      const canonical = `${window.location.origin}${canonicalPath(route.path)}`
      setCanonical(canonical)
      setMeta('property', 'og:url', canonical)
      setMeta('property', 'og:image', `${window.location.origin}/logo.svg`)
    }
  } catch {
    // 非 http(s) 环境（file://、about:blank 等）跳过绝对 URL 元数据。
  }

  const lang = document.documentElement.getAttribute('lang') || 'zh-CN'
  setMeta('property', 'og:locale', lang === 'en' ? 'en_US' : 'zh_CN')
}

/**
 * 同步当前路由的 SEO 元数据：读取当前路由、站点设置与自定义菜单后应用。
 * 路由导航（router.afterEach）与语言切换（setLocale）统一走此入口，
 * 避免各自重复“取路由 + 取门店 + 收集菜单”的逻辑。
 */
export async function syncCurrentRouteSeo(): Promise<void> {
  const [{ default: router }, { useAppStore, useAuthStore, useAdminSettingsStore }] = await Promise.all([
    import('@/router'),
    import('@/stores'),
  ])

  const appStore = useAppStore()
  const authStore = useAuthStore()
  const adminSettingsStore = useAdminSettingsStore()
  const customMenuItems = [
    ...(appStore.cachedPublicSettings?.custom_menu_items ?? []),
    ...(authStore.isAdmin ? adminSettingsStore.customMenuItems : []),
  ]

  applyRouteSeo(router.currentRoute.value, appStore.siteName, customMenuItems)
}
