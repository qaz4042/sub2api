import type { ContactMethod } from '@/types'
import { sanitizeUrl } from '@/utils/url'

export type ContactMethodIcon = 'chat' | 'mail' | 'link'

export interface DisplayContactMethod {
  key: string
  label: string
  value: string
  href?: string
  external: boolean
  icon: ContactMethodIcon
  sort?: number
}

const emailPattern = /^[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}$/i

export function resolveContactMethods(
  configured: ContactMethod[] | undefined,
  legacyContactInfo: string,
  supportLabel: string,
): DisplayContactMethod[] {
  const hasStructuredMethods = Array.isArray(configured) && configured.length > 0
  const configuredMethods = hasStructuredMethods
    ? configured
        .filter((method) => method && method.enabled !== false)
        .map((method) => mapConfiguredContactMethod(method, supportLabel))
        .filter((method): method is DisplayContactMethod => method !== null)
        .sort((a, b) => (a.sort ?? 0) - (b.sort ?? 0))
    : []

  return hasStructuredMethods
    ? configuredMethods
    : parseLegacyContactInfo(legacyContactInfo, supportLabel)
}

function parseLegacyContactInfo(raw: string, supportLabel: string): DisplayContactMethod[] {
  const chunks = raw
    .split(/[\n|；;]+/)
    .map((part) => part.trim())
    .filter(Boolean)
  const seen = new Set<string>()

  return chunks.flatMap((chunk) => parseLegacyContactChunk(chunk, supportLabel)).filter((method) => {
    const dedupeKey = method.href || `${method.label}:${method.value}`
    if (seen.has(dedupeKey)) return false
    seen.add(dedupeKey)
    return true
  })
}

function parseLegacyContactChunk(chunk: string, supportLabel: string): DisplayContactMethod[] {
  const labelMatch = chunk.match(/^([^:：]{1,24})[:：]\s*(.+)$/)
  const labelHint = labelMatch?.[1]?.trim()
  const value = (labelMatch?.[2] || chunk).trim()
  const methods: DisplayContactMethod[] = []
  const telegram = parseTelegram(value)

  if (telegram) {
    methods.push({
      key: `telegram:${telegram.href}`,
      label: normalizeContactLabel(labelHint, 'Telegram'),
      value: telegram.display,
      href: telegram.href,
      external: true,
      icon: 'chat',
    })
  }

  const email = value.match(/[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}/i)?.[0]
  if (email) {
    methods.push({
      key: `email:${email}`,
      label: normalizeContactLabel(labelHint, 'Email'),
      value: email,
      href: `mailto:${email}`,
      external: false,
      icon: 'mail',
    })
  }

  if (methods.length > 0) return methods

  const url = value.match(/https?:\/\/[^\s]+/i)?.[0]
  if (url) {
    const cleanUrl = sanitizeUrl(url.replace(/[，。,.;；|)）]+$/g, ''))
    if (cleanUrl) {
      return [{
        key: `url:${cleanUrl}`,
        label: normalizeContactLabel(labelHint, getUrlHost(cleanUrl)),
        value: cleanUrl,
        href: cleanUrl,
        external: true,
        icon: 'link',
      }]
    }
  }

  return [{
    key: `text:${chunk}`,
    label: normalizeContactLabel(labelHint, supportLabel),
    value,
    external: false,
    icon: 'chat',
  }]
}

function mapConfiguredContactMethod(
  method: ContactMethod,
  supportLabel: string,
): DisplayContactMethod | null {
  const type = method.type?.trim().toLowerCase() || 'text'
  const value = method.value?.trim() || method.url?.trim() || ''
  const url = method.url?.trim() || ''
  const label = method.label?.trim() || defaultContactLabel(type, supportLabel)
  const sort = Number.isFinite(method.sort) ? Number(method.sort) : 0
  if (!label || (!value && !url)) return null

  if (type === 'email') {
    const email = (value || url).replace(/^mailto:/i, '')
    return {
      key: `email:${email}`,
      label,
      value: email,
      href: emailPattern.test(email) ? `mailto:${email}` : undefined,
      external: false,
      icon: 'mail',
      sort,
    }
  }

  if (type === 'telegram') {
    const telegram = parseTelegram(url || value)
    const href = telegram?.href || sanitizeUrl(url)
    return {
      key: `telegram:${href || value}`,
      label,
      value: telegram?.display || value,
      href: href || undefined,
      external: Boolean(href),
      icon: 'chat',
      sort,
    }
  }

  const href = type === 'link' ? sanitizeUrl(url || value) : undefined
  return {
    key: `${type}:${href || value}`,
    label,
    value,
    href: href || undefined,
    external: Boolean(href),
    icon: type === 'text' ? 'chat' : 'link',
    sort,
  }
}

function parseTelegram(value: string): { href: string; display: string } | null {
  const urlMatch = value.match(/(?:https?:\/\/)?(?:t\.me|telegram\.me)\/([A-Za-z0-9_]{5,32})/i)
  if (urlMatch?.[1]) {
    return { href: `https://t.me/${urlMatch[1]}`, display: `@${urlMatch[1]}` }
  }
  const handleMatch = value.match(/(?:^|\s)@([A-Za-z0-9_]{5,32})(?:\s|$)/)
  if (handleMatch?.[1]) {
    return { href: `https://t.me/${handleMatch[1]}`, display: `@${handleMatch[1]}` }
  }
  return null
}

function defaultContactLabel(type: string, supportLabel: string): string {
  if (type === 'telegram') return 'Telegram'
  if (type === 'email') return 'Email'
  if (type === 'link') return 'Link'
  return supportLabel
}

function normalizeContactLabel(label: string | undefined, fallback: string): string {
  const normalized = label?.trim()
  if (!normalized) return fallback
  if (/^(tg|telegram)$/i.test(normalized)) return 'Telegram'
  if (/^(email|mail|e-mail|邮箱|郵箱)$/i.test(normalized)) return 'Email'
  return normalized
}

function getUrlHost(url: string): string {
  try {
    return new URL(url).hostname.replace(/^www\./, '')
  } catch {
    return 'Link'
  }
}
