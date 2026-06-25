import DOMPurify from 'dompurify'

export function sanitizeHtml(html: string): string {
  if (!html) return ''
  return DOMPurify.sanitize(html, {
    ADD_ATTR: ['target'],
    FORBID_TAGS: ['script', 'iframe', 'object', 'embed', 'base'],
  })
}

export function sanitizeSvg(svg: string): string {
  if (!svg) return ''
  const sanitized = DOMPurify.sanitize(svg, { USE_PROFILES: { svg: true, svgFilters: true } })
  if (sanitized) return sanitized

  const wrapped = DOMPurify.sanitize(`<svg>${svg}</svg>`, { USE_PROFILES: { svg: true, svgFilters: true } })
  const container = document.createElement('div')
  container.innerHTML = wrapped
  return container.querySelector('svg')?.innerHTML ?? ''
}
