import { inject, onScopeDispose, provide, reactive, ref, type InjectionKey } from 'vue'
import type { Proxy } from '@/types'
import { loadOpenAILatency, testOpenAILatency, type OpenAIProxyLatency } from './api'

interface Entry {
  result: OpenAIProxyLatency | null
  error: 'load' | 'test' | null
  busy: boolean
  loading: boolean
  fingerprint?: string
}

export function createOpenAIProxyTests() {
  const entries = reactive(new Map<number, Entry>())
  const requests = new Map<number, AbortController>()
  const pendingTests = new Map<number, Promise<boolean>>()
  const batchRunning = ref(false)
  const completed = ref(0)
  const total = ref(0)
  const succeeded = ref(0)
  let disposed = false

  function entry(id: number): Entry {
    if (!entries.has(id)) entries.set(id, { result: null, error: null, busy: false, loading: false })
    return entries.get(id)!
  }

  async function load(proxy: Proxy) {
    const current = entry(proxy.id)
    const fingerprint = JSON.stringify([proxy.protocol, proxy.host, proxy.port, proxy.username, proxy.password])
    if (current.busy) {
      if (current.fingerprint === fingerprint) return
      current.fingerprint = fingerprint
      current.result = null
      await pendingTests.get(proxy.id)
      if (disposed || current.fingerprint !== fingerprint) return
    }
    if (current.loading && current.fingerprint === fingerprint) return
    current.fingerprint = fingerprint
    requests.get(proxy.id)?.abort()
    const request = new AbortController()
    requests.set(proxy.id, request)
    current.result = null
    current.error = null
    current.loading = true
    try {
      const result = await loadOpenAILatency(proxy.id, request.signal)
      if (!request.signal.aborted) current.result = result
    } catch {
      if (!request.signal.aborted) current.error = 'load'
    } finally {
      if (!request.signal.aborted) current.loading = false
    }
  }

  function test(id: number): Promise<boolean> {
    const pending = pendingTests.get(id)
    if (pending) return pending
    if (disposed) return Promise.resolve(false)
    const current = entry(id)
    const fingerprint = current.fingerprint
    requests.get(id)?.abort()
    const request = new AbortController()
    requests.set(id, request)
    current.busy = true
    current.loading = false
    current.error = null
    const promise = (async () => {
      try {
        const result = await testOpenAILatency(id, request.signal)
        if (request.signal.aborted) return false
        if (current.fingerprint === fingerprint) current.result = result
        return result.success
      } catch {
        if (!request.signal.aborted && current.fingerprint === fingerprint) {
          current.result = null
          current.error = 'test'
        }
        return false
      } finally {
        current.busy = false
        pendingTests.delete(id)
      }
    })()
    pendingTests.set(id, promise)
    return promise
  }

  async function runBatch(ids: number[]) {
    if (batchRunning.value || disposed) return
    const snapshot = [...new Set(ids)]
    batchRunning.value = true
    total.value = snapshot.length
    completed.value = 0
    succeeded.value = 0
    let next = 0
    async function worker() {
      while (!disposed && next < snapshot.length) {
        const id = snapshot[next++]!
        if (await test(id)) succeeded.value++
        completed.value++
      }
    }
    try {
      await Promise.all(Array.from({ length: Math.min(3, snapshot.length) }, worker))
    } finally {
      batchRunning.value = false
    }
  }

  function dispose() {
    disposed = true
    requests.forEach(request => request.abort())
  }

  return { entry, load, test, runBatch, dispose, batchRunning, completed, total, succeeded }
}

const key: InjectionKey<ReturnType<typeof createOpenAIProxyTests>> = Symbol('openai-proxy-tests')

export function provideOpenAIProxyTests() {
  const state = createOpenAIProxyTests()
  provide(key, state)
  onScopeDispose(state.dispose)
  return state
}

export function useOpenAIProxyTests() {
  const shared = inject(key, null)
  if (shared) return shared
  const local = createOpenAIProxyTests()
  onScopeDispose(local.dispose)
  return local
}
