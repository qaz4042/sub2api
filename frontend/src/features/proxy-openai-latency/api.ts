import { apiClient } from '@/api/client'

export interface OpenAIProxyLatency {
  success: boolean
  latency_ms?: number
  http_status?: number
  message: string
  checked_at: string
}

export async function loadOpenAILatency(id: number, signal: AbortSignal) {
  const { data } = await apiClient.get<OpenAIProxyLatency | null>(
    `/admin/proxies/${id}/openai-latency`, { signal }
  )
  return data
}

export async function testOpenAILatency(id: number, signal: AbortSignal) {
  const { data } = await apiClient.post<OpenAIProxyLatency>(
    `/admin/proxies/${id}/openai-latency`, null, { signal }
  )
  return data
}
