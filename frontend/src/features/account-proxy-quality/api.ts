import { apiClient } from '@/api/client'

export interface Config { enabled: boolean; all_eligible: boolean; proxy_ids: number[]; version: number }
export interface Settings extends Config {
  candidate_count: number
  current_proxy_id: number | null
  reason: string
  last_evaluated_at: string | null
  last_switch_at: string | null
  choices: { id: number; name: string }[]
  events: { from_proxy_id: number; to_proxy_id: number; reason: string; created_at: string }[]
  samples: { proxy_id: number; model: string; transport: string; total: number; successes: number; first_tokens: number; http_5xx: number }[]
}
export async function loadSettings(id: number, signal: AbortSignal) {
  return (await apiClient.get<Settings>(`/admin/proxies/account-auto-switch/${id}`, { signal })).data
}
export async function saveSettings(id: number, config: Config, signal: AbortSignal) {
  await apiClient.put(`/admin/proxies/account-auto-switch/${id}`, config, { signal })
}
