<template>
  <div class="flex flex-col items-start gap-1 whitespace-nowrap">
    <span
      class="badge"
      :class="result ? (result.success ? 'badge-success' : 'badge-danger') : 'badge-gray'"
      :title="result ? `${result.message} · ${result.checked_at}` : description"
      role="status"
    >
      {{ result ? (result.success ? `${result.latency_ms}ms` : failed) : '—' }}
    </span>
    <button
      type="button"
      class="text-xs text-primary-600 hover:underline disabled:cursor-not-allowed disabled:opacity-50 dark:text-primary-400"
      :disabled="busy || loading"
      :title="description"
      @click="runTest"
    >
      {{ busy ? testing : label }}
    </button>
    <span v-if="error" class="max-w-40 whitespace-normal text-xs text-red-600" role="alert">{{ error }}</span>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Proxy } from '@/types'
import { loadOpenAILatency, testOpenAILatency, type OpenAIProxyLatency } from './api'

const props = defineProps<{ proxy: Proxy }>()
const { locale } = useI18n()
const zh = computed(() => locale.value.startsWith('zh'))
const label = computed(() => zh.value ? '测试 OpenAI' : 'Test OpenAI')
const testing = computed(() => zh.value ? '测试中…' : 'Testing…')
const failed = computed(() => zh.value ? '连接失败' : 'Failed')
const description = computed(() => zh.value
  ? '仅测试 OpenAI API 连接延迟，不代表模型响应速度；结果保留 24 小时'
  : 'OpenAI API connectivity only, not model response time; cached for 24 hours')
const result = ref<OpenAIProxyLatency | null>(null)
const busy = ref(false)
const loading = ref(false)
const error = ref('')
let request: AbortController | undefined

async function requestResult(test: boolean) {
  request?.abort()
  const current = new AbortController()
  request = current
  busy.value = test
  loading.value = !test
  error.value = ''
  try {
    const value = await (test ? testOpenAILatency : loadOpenAILatency)(props.proxy.id, current.signal)
    if (!current.signal.aborted) result.value = value
  } catch {
    if (!current.signal.aborted) {
      result.value = null
      error.value = zh.value
        ? (test ? '测试请求失败，请重试' : '历史结果加载失败，请重新测试')
        : (test ? 'Test request failed. Retry.' : 'Could not load history. Run a new test.')
    }
  } finally {
    if (!current.signal.aborted) {
      busy.value = false
      loading.value = false
    }
  }
}

const runTest = () => requestResult(true)
watch(() => [props.proxy.id, props.proxy.protocol, props.proxy.host, props.proxy.port,
  props.proxy.username, props.proxy.password], () => {
  result.value = null
  void requestResult(false)
}, { immediate: true })
onBeforeUnmount(() => request?.abort())
</script>
