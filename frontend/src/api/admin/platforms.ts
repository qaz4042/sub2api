import { apiClient } from '../client'
import type { PlatformConfig } from '@/types'

export async function list(): Promise<PlatformConfig[]> {
  const { data } = await apiClient.get<PlatformConfig[]>('/admin/platforms')
  return data
}

export async function update(key: string, enabled: boolean): Promise<PlatformConfig> {
  const { data } = await apiClient.put<PlatformConfig>(
    `/admin/platforms/${encodeURIComponent(key)}`,
    { enabled },
  )
  return data
}

export default { list, update }
