export interface KeyModel {
  value: string
  label: string
}

export async function fetchKeyModels(
  baseUrl: string,
  apiKey: string,
  platform?: string,
  signal?: AbortSignal
): Promise<KeyModel[]> {
  const root = (baseUrl || window.location.origin).trim().replace(/\/+$/, '').replace(/\/(?:v1|v1beta)$/i, '')
  const prefix = platform === 'antigravity' && !root.endsWith('/antigravity') ? '/antigravity' : ''
  const response = await fetch(`${root}${prefix}/v1/models`, {
    headers: { Accept: 'application/json', Authorization: `Bearer ${apiKey}` },
    cache: 'no-store',
    signal
  })
  if (!response.ok) throw new Error(`Models request failed: ${response.status}`)
  const payload = await response.json()
  if (!Array.isArray(payload?.data)) throw new Error('Invalid models response')
  const models = new Map<string, KeyModel>()
  for (const model of payload.data) {
    if (typeof model?.id !== 'string' || !model.id.trim()) continue
    const id = model.id.trim()
    models.set(id, {
      value: id,
      label: typeof model.display_name === 'string' && model.display_name.trim() ? model.display_name : id
    })
  }
  return [...models.values()]
}
