import { onMounted, onUnmounted, reactive, ref } from 'vue'
import { loadOverview, type Overview, type OverviewRow, type OverviewQuery } from './api'

export function monitorStatus(row: OverviewRow) {
  if (!row.eligible) return 'unavailable'
  if (!row.config.enabled) return 'paused'
  if (!row.latest) return 'empty'
  if (row.stale) return 'stale'
  return row.latest.success ? 'success' : 'failed'
}

export function useOverview() {
  const query = reactive<OverviewQuery>({ hours: 24, page: 1, search: '', mode: 'all', sort: 'latency' })
  const data = ref<Overview | null>(null)
  const busy = ref(false)
  const error = ref(false)
  let controller: AbortController | undefined
  let timer: ReturnType<typeof setTimeout> | undefined
  let revision = 0
  let disposed = false

  function cancel() {
    revision++
    controller?.abort()
    clearTimeout(timer)
    busy.value = false
  }
  async function load(reset = false) {
    cancel()
    if (disposed || document.hidden) return
    if (reset) data.value = null
    const current = revision
    const request = new AbortController()
    controller = request
    busy.value = true
    error.value = false
    try {
      const result = await loadOverview({ ...query }, request.signal)
      if (current !== revision || request.signal.aborted) return
      // A proxy may have been deleted while viewing the last page.
      const lastPage = Math.max(1, Math.ceil(result.total / result.page_size))
      if (query.page > lastPage) {
        query.page = lastPage
        void load(true)
        return
      }
      data.value = result
    } catch {
      if (current === revision && !request.signal.aborted) error.value = true
    } finally {
      if (current === revision) {
        busy.value = false
        timer = setTimeout(() => void load(), 15000)
      }
    }
  }
  function reset() { query.page = 1; void load(true) }
  function page(value: number) { query.page = value; void load(true) }
  function visibility() { if (document.hidden) cancel(); else void load() }
  onMounted(() => { document.addEventListener('visibilitychange', visibility); void load() })
  onUnmounted(() => { disposed = true; cancel(); document.removeEventListener('visibilitychange', visibility) })
  return { query, data, busy, error, load, reset, page }
}
