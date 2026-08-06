import { beforeEach, describe, expect, it } from 'vitest'
import type { RouteLocationNormalized } from 'vue-router'
import {
  DEFAULT_SEO_DESCRIPTION,
  applyRouteSeo,
  canonicalPath,
  resolveRouteDescription,
  routeShouldBeIndexed,
} from '@/router/seo'

function makeRoute(overrides: Partial<RouteLocationNormalized> = {}): RouteLocationNormalized {
  return {
    path: '/home',
    fullPath: '/home',
    name: 'Home',
    params: {},
    query: {},
    hash: '',
    meta: {},
    matched: [],
    redirectedFrom: undefined,
    ...overrides,
  } as RouteLocationNormalized
}

function metaContent(attribute: 'name' | 'property', key: string): string | null {
  return document.head.querySelector<HTMLMetaElement>(`meta[${attribute}="${key}"]`)?.getAttribute('content') ?? null
}

beforeEach(() => {
  document.title = ''
  document.documentElement.setAttribute('lang', 'zh-CN')
  document.head
    .querySelectorAll('meta[name="description"], meta[name="robots"], meta[property^="og:"], link[rel="canonical"]')
    .forEach((element) => element.remove())
})

describe('canonicalPath', () => {
  it('去掉非根路径的尾部斜杠', () => {
    expect(canonicalPath('/legal/terms/')).toBe('/legal/terms')
  })

  it('保留根路径', () => {
    expect(canonicalPath('/')).toBe('/')
  })
})

describe('routeShouldBeIndexed', () => {
  it('显式 indexable 的页面允许收录', () => {
    expect(routeShouldBeIndexed(makeRoute({ meta: { indexable: true } }))).toBe(true)
  })

  it('未显式标记的页面默认不收录（fail-closed）', () => {
    expect(routeShouldBeIndexed(makeRoute({ meta: {} }))).toBe(false)
    expect(routeShouldBeIndexed(makeRoute({ meta: { requiresAuth: false } }))).toBe(false)
  })

  it('indexable 但显式 noindex 的页面不收录', () => {
    expect(routeShouldBeIndexed(makeRoute({ meta: { indexable: true, noindex: true } }))).toBe(false)
  })
})

describe('resolveRouteDescription', () => {
  it('i18n 翻译缺失时回退到静态 description', () => {
    const route = makeRoute({ meta: { description: '静态描述' } })
    expect(resolveRouteDescription(route)).toBe('静态描述')
  })

  it('无任何描述时使用默认描述', () => {
    expect(resolveRouteDescription(makeRoute({ meta: {} }))).toBe(DEFAULT_SEO_DESCRIPTION)
  })
})

describe('applyRouteSeo', () => {
  it('indexable 路由同步标题、描述、canonical 和 Open Graph', () => {
    applyRouteSeo(
      makeRoute({
        path: '/model-plaza/',
        meta: { indexable: true, title: 'Model Plaza', description: '模型广场描述' },
      }),
      'Kaka Codex',
    )

    expect(document.title).toBe('Model Plaza - Kaka Codex')
    expect(metaContent('name', 'description')).toBe('模型广场描述')
    expect(metaContent('name', 'robots')).toBe('index, follow')

    const origin = window.location.origin
    expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe(`${origin}/model-plaza`)
    expect(metaContent('property', 'og:url')).toBe(`${origin}/model-plaza`)
    expect(metaContent('property', 'og:image')).toBe(`${origin}/logo.svg`)
    expect(metaContent('property', 'og:title')).toBe('Model Plaza - Kaka Codex')
    expect(metaContent('property', 'og:description')).toBe('模型广场描述')
    expect(metaContent('property', 'og:site_name')).toBe('Kaka Codex')
    expect(metaContent('property', 'og:locale')).toBe('zh_CN')
  })

  it('未标记的页面默认 noindex', () => {
    applyRouteSeo(makeRoute({ path: '/login', meta: { title: 'Login' } }), 'Kaka Codex')

    expect(metaContent('name', 'robots')).toBe('noindex, follow')
    expect(document.title).toBe('Login - Kaka Codex')
  })

  it('indexable 但显式 noindex 的页面强制不收录', () => {
    applyRouteSeo(
      makeRoute({ path: '/legal/terms', meta: { indexable: true, noindex: true, title: 'Legal' } }),
      'Sub2API',
    )

    expect(metaContent('name', 'robots')).toBe('noindex, follow')
  })

  it('og:locale 跟随 html lang', () => {
    document.documentElement.setAttribute('lang', 'en')
    applyRouteSeo(makeRoute({ meta: { indexable: true, title: 'Home' } }), 'Sub2API')

    expect(metaContent('property', 'og:locale')).toBe('en_US')
  })

  it('重复导航复用已有 meta 节点，不产生重复标签', () => {
    applyRouteSeo(makeRoute({ meta: { indexable: true, title: 'Home' } }), 'Sub2API')
    applyRouteSeo(makeRoute({ path: '/model-plaza', meta: { indexable: true, title: 'Model Plaza' } }), 'Sub2API')

    expect(document.head.querySelectorAll('meta[name="robots"]')).toHaveLength(1)
    expect(document.head.querySelectorAll('link[rel="canonical"]')).toHaveLength(1)
  })
})
