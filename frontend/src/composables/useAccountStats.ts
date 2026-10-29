import { computed, ref, watch } from 'vue'
import { adminAPI } from '@/api/admin'
import type { AccountStatsGranularity, AccountUsageStatsResponse } from '@/types'
import { formatLocalDate } from '@/utils/usageDateRange'
import { isValidAccountStatsDateRange } from '@/utils/accountStatsDateRange'

export function useAccountStats(show: () => boolean, accountId: () => number | undefined) {
  const days = ref(30)
  const dateRange = ref<{ startDate: string; endDate: string } | null>(null)
  const rollingRange = (count: number) => {
    const end = new Date()
    const start = new Date(end)
    start.setDate(start.getDate() - count + 1)
    return { startDate: formatLocalDate(start), endDate: formatLocalDate(end) }
  }
  // Keep labels and requests on the same snapshot; refresh rolling dates on reload.
  const activeRange = ref(rollingRange(days.value))
  const startDate = computed(() => activeRange.value.startDate)
  const endDate = computed(() => activeRange.value.endDate)
  const invalidRange = computed(() => !isValidAccountStatsDateRange(startDate.value, endDate.value))
  const setDateRange = (range: { startDate: string; endDate: string }) => {
    dateRange.value = { startDate: range.startDate, endDate: range.endDate }
  }
  const granularity = ref<AccountStatsGranularity>('day')
  const includeTests = ref(true)
  const stats = ref<AccountUsageStatsResponse | null>(null)
  const loading = ref(false)
  const error = ref(false)

  watch([show, accountId, days, granularity, includeTests, dateRange], async ([visible, id, range, resolution, tests, dates], _, onCleanup) => {
    const controller = new AbortController()
    onCleanup(() => controller.abort())
    stats.value = null
    error.value = false
    loading.value = false
    if (!visible || id == null) return
    activeRange.value = dates ?? rollingRange(range)
    if (invalidRange.value) return
    loading.value = true
    try {
      const result = await adminAPI.accounts.getStats(id, range, {
        granularity: resolution, includeTests: tests, signal: controller.signal,
        startDate: startDate.value, endDate: endDate.value
      })
      if (!controller.signal.aborted) stats.value = result
    } catch {
      if (!controller.signal.aborted) error.value = true
    } finally {
      if (!controller.signal.aborted) loading.value = false
    }
  }, { immediate: true })

  return { days, startDate, endDate, setDateRange, invalidRange, granularity, includeTests, stats, loading, error }
}
