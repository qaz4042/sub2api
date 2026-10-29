import { apiClient } from '@/api/client'

export interface MonitorConfig {
  enabled: boolean
  interval_seconds: number
  runtime_state?: string
}
export interface MonitorSettings {
 config: MonitorConfig
 eligible: boolean
}
export async function loadSettings(id: number, signal: AbortSignal) {
 const { data } = await apiClient.get<MonitorSettings>(`/admin/proxies/${id}/openai-monitor/config`, { signal })
 return data
}
export interface Metrics {
  count: number
  successes: number
  failures: number
  success_rate: number | null
  average_ms: number | null
  p95_ms: number | null
}
export interface Sample {
  id: number
  started_at: string
  finished_at: string
  success: boolean
  latency_ms: number | null
  http_status: number
  error_class: string
}
export interface Report {
  config: MonitorConfig
  eligible: boolean
  summary: Metrics
  latest: Sample | null
  stale: boolean
  stale_seconds: number
  from: string
  to: string
  first_sample_at: string | null
  last_sample_at: string | null
  bucket_seconds: number
  trend: (Metrics & { at: string })[]
  records: Sample[]
  truncated: boolean
  page: number
  page_size: number
}
export async function loadReport(id: number, hours: number, page: number, signal: AbortSignal, until?: string) {
  const { data } = await apiClient.get<Report>(`/admin/proxies/${id}/openai-monitor`, {
    params: { hours, page, until }, signal
  })
  return data
}
export async function saveConfig(id: number, config: MonitorConfig, signal: AbortSignal) {
  await apiClient.put(`/admin/proxies/${id}/openai-monitor`, config, { signal })
}

export type OverviewSort = 'latency' | 'failures' | 'p95' | 'success_rate' | 'name'
export interface OverviewQuery {
  hours: number
  page: number
  search: string
  mode: 'all' | 'enabled' | 'paused'
  sort: OverviewSort
}
export interface OverviewRow extends MonitorSettings {
  id: number
  name: string
  proxy_status: string
  latest: Sample | null
  stale: boolean
  summary: Metrics
}
export interface Overview {
  rows: OverviewRow[]
  total: number
  page: number
  page_size: number
  from: string
  to: string
}
export async function loadOverview(query: OverviewQuery, signal: AbortSignal) {
  const { data } = await apiClient.get<Overview>('/admin/proxies/openai-monitor/overview', {
    params: query, signal
  })
  return data
}

export interface TrendPoint extends Metrics { at: string }
export interface Trends {
  from: string
  to: string
  bucket_seconds: number
  series: { id: number; name: string; points: TrendPoint[] }[]
}
export async function loadTrends(ids: number[], hours: number, until: string, signal: AbortSignal) {
  const { data } = await apiClient.get<Trends>('/admin/proxies/openai-monitor/trends', {
    params: { ids: ids.join(','), hours, until }, signal
  })
  return data
}
