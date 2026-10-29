import { computed, onMounted, onUnmounted, ref, watch, type Ref } from 'vue'
import { loadTrends, type Overview, type Trends } from './api'

export function useOverviewTrends(overview: Ref<Overview | null>) {
  const selected = ref<number[]>([])
  const data = ref<Trends | null>(null)
  const busy = ref(false)
  const error = ref(false)
  const rows = computed(() => overview.value?.rows || [])
  let controller: AbortController | undefined
  let revision = 0
  let rowsKey = ''
  function cancel() { revision++; controller?.abort(); busy.value = false }
  async function load(reset = false) {
    cancel()
    if (reset) data.value = null
    error.value = false
    const source = overview.value
    if (!source || !selected.value.length) { data.value = null; return }
    if (document.hidden) return
    const request = new AbortController()
    controller = request
    const current = revision
    busy.value = true
    error.value = false
    const hours = Math.round((Date.parse(source.to) - Date.parse(source.from)) / 3600000)
    try {
      const result = await loadTrends([...selected.value], hours, source.to, request.signal)
      if (current === revision && !request.signal.aborted) data.value = result
    } catch {
      if (current === revision && !request.signal.aborted) error.value = true
    } finally {
      if (current === revision) busy.value = false
    }
  }
  function toggle(id: number) {
    if (selected.value.includes(id)) selected.value = selected.value.filter(value => value !== id)
    else if (selected.value.length < 10) selected.value = [...selected.value, id]
    else return
    void load(true)
  }
  watch(overview, source => {
    if (!source) { cancel(); data.value = null; return }
    const key = source.rows.map(row => row.id).sort((a, b) => a - b).join(',')
    const sameRows = key === rowsKey
    rowsKey = key
    if (!sameRows) {
      selected.value = source.rows.filter(row => row.config.enabled && row.eligible).slice(0, 10).map(row => row.id)
      if (!selected.value.length) selected.value = source.rows.filter(row => row.summary.count > 0).slice(0, 10).map(row => row.id)
    }
    void load(!sameRows)
  }, { immediate: true })
  function visibility() { if (document.hidden) cancel() }
  onMounted(() => document.addEventListener('visibilitychange', visibility))
  onUnmounted(() => { cancel(); document.removeEventListener('visibilitychange', visibility) })
  return { selected, data, busy, error, rows, toggle, load }
}
