import { apiClient } from '../client'

export interface PlatformConfig {
  key: string
  label: string
  description: string
  enabled: boolean
  core: boolean
  sort_order: number
  created_at?: string
  updated_at?: string
}

export interface CreatePlatformConfigRequest {
  key: string
  label: string
  description?: string
  enabled: boolean
  sort_order?: number
}

export interface UpdatePlatformConfigRequest {
  label?: string
  description?: string
  enabled?: boolean
  sort_order?: number
}

export async function list(): Promise<PlatformConfig[]> {
  const { data } = await apiClient.get<PlatformConfig[]>('/admin/platforms')
  return data
}

export async function create(payload: CreatePlatformConfigRequest): Promise<PlatformConfig> {
  const { data } = await apiClient.post<PlatformConfig>('/admin/platforms', payload)
  return data
}

export async function update(key: string, payload: UpdatePlatformConfigRequest): Promise<PlatformConfig> {
  const { data } = await apiClient.put<PlatformConfig>(`/admin/platforms/${encodeURIComponent(key)}`, payload)
  return data
}

export async function deletePlatform(key: string): Promise<{ deleted: boolean }> {
  const { data } = await apiClient.delete<{ deleted: boolean }>(`/admin/platforms/${encodeURIComponent(key)}`)
  return data
}

export default {
  list,
  create,
  update,
  delete: deletePlatform,
}
