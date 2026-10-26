import { ref, watch } from 'vue'
import { adminAPI } from '@/api/admin'
import type { AccountStatsGranularity, AccountUsageStatsResponse } from '@/types'

export function useAccountStats(show: () => boolean, accountId: () => number | undefined) {
  const days = ref(30)
  const granularity = ref<AccountStatsGranularity>('day')
  const includeTests = ref(true)
  const stats = ref<AccountUsageStatsResponse | null>(null)
  const loading = ref(false)
  const error = ref(false)

  watch([show, accountId, days, granularity, includeTests], async ([visible, id, range, resolution, tests], _, onCleanup) => {
    const controller = new AbortController()
    onCleanup(() => controller.abort())
    stats.value = null
    error.value = false
    loading.value = false
    if (!visible || id == null) return
    loading.value = true
    try {
      const result = await adminAPI.accounts.getStats(id, range, {
        granularity: resolution, includeTests: tests, signal: controller.signal
      })
      if (!controller.signal.aborted) stats.value = result
    } catch {
      if (!controller.signal.aborted) error.value = true
    } finally {
      if (!controller.signal.aborted) loading.value = false
    }
  }, { immediate: true })

  return { days, granularity, includeTests, stats, loading, error }
}
