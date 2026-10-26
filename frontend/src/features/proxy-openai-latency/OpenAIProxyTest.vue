<template>
  <div class="flex flex-col items-start gap-1 whitespace-nowrap">
    <span
      class="badge"
      :class="result ? (result.success ? 'badge-success' : 'badge-danger') : 'badge-gray'"
      :title="result ? `${result.message} · ${result.checked_at}` : description"
      role="status"
    >
      {{ busy ? testing : result ? (result.success ? `${result.latency_ms}ms` : failed) : '—' }}
    </span>
    <button
      type="button"
      class="text-xs text-primary-600 hover:underline disabled:cursor-not-allowed disabled:opacity-50 dark:text-primary-400"
      :disabled="busy || loading || state.batchRunning.value"
      :title="description"
      @click="runTest"
    >
      {{ busy ? testing : label }}
    </button>
    <span v-if="error" class="max-w-40 whitespace-normal text-xs text-red-600" role="alert">{{ error }}</span>
  </div>
</template>

<script setup lang="ts">
import { computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Proxy } from '@/types'
import { useOpenAIProxyTests } from './state'

const props = defineProps<{ proxy: Proxy }>()
const { locale } = useI18n()
const zh = computed(() => locale.value.startsWith('zh'))
const label = computed(() => zh.value ? '测试 OpenAI' : 'Test OpenAI')
const testing = computed(() => zh.value ? '测试中…' : 'Testing…')
const failed = computed(() => zh.value ? '连接失败' : 'Failed')
const description = computed(() => zh.value
  ? '仅测试 OpenAI API 连接延迟，不代表模型响应速度；结果保留 24 小时'
  : 'OpenAI API connectivity only, not model response time; cached for 24 hours')
const state = useOpenAIProxyTests()
const current = computed(() => state.entry(props.proxy.id))
const result = computed(() => current.value.result)
const busy = computed(() => current.value.busy)
const loading = computed(() => current.value.loading)
const error = computed(() => {
  if (!current.value.error) return ''
  return zh.value
    ? (current.value.error === 'test' ? '测试请求失败，请重试' : '历史结果加载失败，请重新测试')
    : (current.value.error === 'test' ? 'Test request failed. Retry.' : 'Could not load history. Run a new test.')
})
const runTest = () => state.test(props.proxy.id)
watch(() => [props.proxy.id, props.proxy.protocol, props.proxy.host, props.proxy.port,
  props.proxy.username, props.proxy.password], () => {
  void state.load(props.proxy)
}, { immediate: true })
</script>
