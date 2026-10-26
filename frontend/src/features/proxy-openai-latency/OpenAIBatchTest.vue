<template>
  <div class="flex flex-wrap items-center gap-2">
    <button
      type="button"
      class="btn btn-secondary"
      :disabled="disabled || state.batchRunning.value || proxyIds.length === 0"
      :title="scopeLabel"
      @click="state.runBatch(proxyIds)"
    >
      <Icon name="play" size="md" class="mr-2" :class="state.batchRunning.value ? 'animate-pulse' : ''" />
      {{ zh ? '批量测试 OpenAI' : 'Batch test OpenAI' }}
      <span class="ml-1">({{ state.batchRunning.value ? `${state.completed.value}/${state.total.value}` : proxyIds.length }})</span>
    </button>
    <span v-if="state.total.value > 0" role="status" class="text-xs text-gray-500 dark:text-gray-400">
      {{ zh ? `成功 ${state.succeeded.value}，失败 ${state.completed.value - state.succeeded.value}`
        : `Passed ${state.succeeded.value}, failed ${state.completed.value - state.succeeded.value}` }}
    </span>
    <span v-else class="text-xs text-gray-500 dark:text-gray-400">{{ scopeLabel }}</span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useOpenAIProxyTests } from './state'

const props = defineProps<{ proxyIds: number[]; selected: boolean; disabled?: boolean }>()
const { locale } = useI18n()
const zh = computed(() => locale.value.startsWith('zh'))
const state = useOpenAIProxyTests()
const scopeLabel = computed(() => zh.value
  ? (props.selected ? '测试勾选的代理，最多 3 个并发' : '未勾选时测试当前页，最多 3 个并发')
  : (props.selected ? 'Test selected proxies; up to 3 at once' : 'Test current page when none selected; up to 3 at once'))
</script>
