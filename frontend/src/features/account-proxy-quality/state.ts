import { onScopeDispose, ref } from 'vue'
import { loadSettings, saveSettings, type Settings } from './api'

export function useProxyQuality() {
  const settings = ref<Settings | null>(null)
  const enabled = ref(false)
  const ids = ref<number[]>([])
  const busy = ref(false)
  const saving = ref(false)
  const error = ref(false)
  const saved = ref(false)
  let controller: AbortController | undefined
  let epoch = 0
  let accountID = 0
  function close() {
    epoch++
    controller?.abort()
    controller = undefined
    accountID = 0
    settings.value = null
    enabled.value = false
    ids.value = []
    busy.value = saving.value = error.value = saved.value = false
  }
  async function load(id: number) {
    controller?.abort()
    const token = ++epoch
    controller = new AbortController()
    accountID = id
    busy.value = true
    error.value = saved.value = false
    try {
      const result = await loadSettings(id, controller.signal)
      if (token !== epoch) return
      settings.value = result
      enabled.value = result.enabled
      ids.value = [...result.proxy_ids]
    } catch {
      if (token === epoch) error.value = true
    } finally {
      if (token === epoch) busy.value = false
    }
  }
  async function save() {
    if (!settings.value || busy.value || saving.value || !accountID) return false
    const id = accountID
    const token = ++epoch
    controller?.abort()
    controller = new AbortController()
    const signal = controller.signal
    saving.value = true
    error.value = saved.value = false
    try {
      await saveSettings(id, { enabled: enabled.value, proxy_ids: [...ids.value], version: settings.value.version }, signal)
      if (token !== epoch) return false
      const result = await loadSettings(id, signal)
      if (token !== epoch) return false
      settings.value = result
      enabled.value = result.enabled
      ids.value = [...result.proxy_ids]
      saved.value = true
      return true
    } catch {
      if (token === epoch) error.value = true
      return false
    } finally {
      if (token === epoch) saving.value = false
    }
  }
  onScopeDispose(close)
  return { settings, enabled, ids, busy, saving, error, saved, close, load, save }
}
