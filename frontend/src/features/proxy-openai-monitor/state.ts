import { onUnmounted, reactive, ref } from 'vue'
import { loadReport, loadSettings, saveConfig, type Report, type MonitorSettings } from './api'

export function useMonitor() {
  const report = ref<Report | null>(null)
  const settings = ref<MonitorSettings | null>(null)
  const config = reactive({ enabled: false, interval_seconds: 30 })
  const busy = ref(false)
  const saving = ref(false)
  const error = ref(false)
  const configError = ref(false)
  const saved = ref(false)
  let controller: AbortController | undefined
  let saveController: AbortController | undefined
  let sequence = 0
  let timer: ReturnType<typeof setTimeout> | undefined
  let configuredId: number | undefined
  let active = false
  let currentId = 0
  let currentHours = 24
  let currentPage = 1
  let until: string | undefined

  function close() {
    active = false
    configuredId = undefined
    until = undefined
    sequence++
    controller?.abort()
    saveController?.abort()
    clearTimeout(timer)
    busy.value = false
    saving.value = false
    report.value = null
    settings.value = null
    configError.value = false
    saved.value = false
  }
  async function load(id: number, hours: number, page: number, resetConfig = false) {
    if (id !== currentId || hours !== currentHours || page === 1) until = undefined
    if (id !== currentId) settings.value = null
    active = true
    currentId = id
    currentHours = hours
    currentPage = page
    controller?.abort()
    clearTimeout(timer)
    const request = new AbortController()
    controller = request
    const revision = ++sequence
    const isCurrent = () => revision === sequence && !request.signal.aborted
    busy.value = true
    error.value = false
    configError.value = false
    report.value = null
    // Configuration stays usable as soon as this lightweight request finishes,
    // even if aggregation is still running or fails.
    const configuration = loadSettings(id, request.signal).then(result => {
      if (!isCurrent()) return
      settings.value = result
      if (resetConfig || configuredId !== id) {
        configuredId = id
        Object.assign(config, { enabled: result.config.enabled, interval_seconds: result.config.interval_seconds })
        saved.value = false
      }
    }).catch(() => { if (isCurrent()) configError.value = true })
    const history = loadReport(id, hours, page, request.signal, until).then(result => {
      if (!isCurrent()) return
      report.value = result
      until = result.to
    }).catch(() => { if (isCurrent()) error.value = true })
    await Promise.allSettled([configuration, history])
    if (isCurrent()) {
      busy.value = false
      if (active) timer = setTimeout(() => void load(currentId, currentHours, currentPage), 15000)
    }
  }
  async function save() {
    if (saving.value || !active || !settings.value) return
    const id = currentId
    const request = new AbortController()
    saveController = request
    // A pending history request must not delay or overwrite a configuration save.
    sequence++
    controller?.abort()
    clearTimeout(timer)
    busy.value = false
    saving.value = true
    configError.value = false
    saved.value = false
    try {
      await saveConfig(id, { ...config }, request.signal)
      if (!active || id !== currentId || request.signal.aborted) return
      saved.value = true
      await load(id, currentHours, currentPage)
    } catch {
      if (active && !request.signal.aborted) configError.value = true
    } finally {
      if (saveController === request) saving.value = false
    }
  }
  onUnmounted(close)
  return { report, settings, config, busy, saving, error, configError, saved, load, save, close }
}
