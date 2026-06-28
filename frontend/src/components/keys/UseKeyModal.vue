<template>
  <BaseDialog
    :show="show"
    :title="t('keys.useKeyModal.title')"
    width="wide"
    @close="emit('close')"
  >
    <div class="space-y-4">
      <div v-if="apiKeyOptions.length > 0" class="space-y-1.5">
        <label class="input-label">{{ t('keys.useKeyModal.keySelectorLabel') }}</label>
        <Select
          :model-value="selectedKeyId"
          :options="apiKeyOptions"
          :searchable="true"
          :search-placeholder="t('keys.useKeyModal.keySelectorSearchPlaceholder')"
          @update:model-value="updateSelectedKey"
        >
          <template #selected="{ option }">
            <span v-if="option" class="flex min-w-0 items-center gap-2">
              <span class="truncate font-medium">{{ option.label }}</span>
              <span class="shrink-0 text-xs text-gray-400 dark:text-gray-500">{{ option.maskedKey }}</span>
            </span>
            <span v-else class="text-gray-400">{{ t('keys.useKeyModal.keySelectorPlaceholder') }}</span>
          </template>
          <template #option="{ option, selected }">
            <div class="flex min-w-0 flex-1 items-center justify-between gap-3">
              <div class="min-w-0">
                <div class="truncate text-sm font-medium text-gray-900 dark:text-white">
                  {{ option.label }}
                </div>
                <div class="mt-0.5 flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                  <span class="font-mono">{{ option.maskedKey }}</span>
                  <span v-if="option.groupName" class="truncate">{{ option.groupName }}</span>
                </div>
              </div>
              <Icon
                v-if="selected"
                name="check"
                size="sm"
                class="shrink-0 text-primary-500"
                :stroke-width="2"
              />
            </div>
          </template>
        </Select>
      </div>

      <!-- No Group Assigned Warning -->
      <div v-if="!platform" class="flex items-start gap-3 p-4 rounded-lg bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800">
        <svg class="w-5 h-5 text-yellow-500 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
        </svg>
        <div>
          <p class="text-sm font-medium text-yellow-800 dark:text-yellow-200">
            {{ t('keys.useKeyModal.noGroupTitle') }}
          </p>
          <p class="text-sm text-yellow-700 dark:text-yellow-300 mt-1">
            {{ t('keys.useKeyModal.noGroupDescription') }}
          </p>
        </div>
      </div>

      <!-- Platform-specific content -->
      <template v-else>
        <!-- Description -->
        <p class="text-sm text-gray-600 dark:text-gray-400">
          {{ platformDescription }}
        </p>

        <!-- Client Tabs -->
        <div v-if="clientTabs.length" class="border-b border-gray-200 dark:border-dark-700">
          <nav class="-mb-px flex space-x-6" aria-label="Client">
            <button
              v-for="tab in clientTabs"
              :key="tab.id"
              @click="activeClientTab = tab.id"
              :class="[
                'whitespace-nowrap py-2.5 px-1 border-b-2 font-medium text-sm transition-colors',
                activeClientTab === tab.id
                  ? 'border-primary-500 text-primary-600 dark:text-primary-400'
                  : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-300'
              ]"
            >
              <span class="flex items-center gap-2">
                <component :is="tab.icon" class="w-4 h-4" />
                {{ tab.label }}
              </span>
            </button>
          </nav>
        </div>

        <!-- OS/Shell Tabs -->
        <div v-if="showShellTabs" class="border-b border-gray-200 dark:border-dark-700">
          <nav class="-mb-px flex space-x-4" aria-label="Tabs">
            <button
              v-for="tab in currentTabs"
              :key="tab.id"
              @click="activeTab = tab.id"
              :class="[
                'whitespace-nowrap py-2.5 px-1 border-b-2 font-medium text-sm transition-colors',
                activeTab === tab.id
                  ? 'border-primary-500 text-primary-600 dark:text-primary-400'
                  : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-300'
              ]"
            >
              <span class="flex items-center gap-2">
                <component :is="tab.icon" class="w-4 h-4" />
                {{ tab.label }}
              </span>
            </button>
          </nav>
        </div>

        <!-- API Integration Test -->
        <div
          v-if="activeClientTab === 'api-test'"
          class="space-y-4 rounded-xl border border-gray-200 bg-gray-50 p-4 dark:border-dark-600 dark:bg-dark-800/70"
        >
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div>
              <h4 class="text-sm font-semibold text-gray-900 dark:text-white">
                {{ t('keys.useKeyModal.apiTest.quickTestTitle') }}
              </h4>
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                {{ t('keys.useKeyModal.apiTest.quickTestDescription') }}
              </p>
            </div>
            <span
              :class="[
                'rounded-full px-2.5 py-1 text-xs font-semibold',
                apiTestStatus === 'success'
                  ? 'bg-green-100 text-green-700 dark:bg-green-500/20 dark:text-green-400'
                  : apiTestStatus === 'error'
                    ? 'bg-red-100 text-red-700 dark:bg-red-500/20 dark:text-red-400'
                    : apiTestStatus === 'connecting'
                      ? 'bg-yellow-100 text-yellow-700 dark:bg-yellow-500/20 dark:text-yellow-400'
                      : 'bg-gray-100 text-gray-600 dark:bg-dark-600 dark:text-gray-300'
              ]"
            >
              {{ apiTestStatusLabel }}
            </span>
          </div>

          <div class="grid gap-3 md:grid-cols-2">
            <div v-if="apiTestProtocolOptions.length > 1" class="space-y-1.5">
              <label class="input-label">{{ t('keys.useKeyModal.apiTest.protocolLabel') }}</label>
              <Select
                v-model="apiTestProtocol"
                :options="apiTestProtocolOptions"
                :disabled="apiTestStatus === 'connecting'"
              />
            </div>
            <div class="space-y-1.5">
              <label class="input-label">{{ t('keys.useKeyModal.apiTest.modelLabel') }}</label>
              <Select
                v-model="apiTestModel"
                :options="apiTestModelOptions"
                :disabled="apiTestStatus === 'connecting'"
              />
            </div>
          </div>

          <div class="space-y-1.5">
            <label class="input-label">{{ t('keys.useKeyModal.apiTest.promptLabel') }}</label>
            <textarea
              v-model="apiTestPrompt"
              rows="2"
              :disabled="apiTestStatus === 'connecting'"
              class="input min-h-[72px] resize-y"
              :placeholder="t('keys.useKeyModal.apiTest.promptPlaceholder')"
            />
          </div>

          <div class="group relative">
            <div
              ref="apiTestTerminalRef"
              class="max-h-[220px] min-h-[118px] overflow-y-auto rounded-xl border border-gray-700 bg-gray-900 p-4 font-mono text-sm dark:border-gray-800 dark:bg-black"
            >
              <div v-if="apiTestStatus === 'idle' && apiTestOutputLines.length === 0" class="flex items-center gap-2 text-gray-500">
                <Icon name="play" size="sm" :stroke-width="2" />
                <span>{{ t('keys.useKeyModal.apiTest.ready') }}</span>
              </div>
              <div v-else-if="apiTestStatus === 'connecting'" class="flex items-center gap-2 text-yellow-400">
                <Icon name="refresh" size="sm" class="animate-spin" :stroke-width="2" />
                <span>{{ t('keys.useKeyModal.apiTest.testing') }}</span>
              </div>

              <div v-for="(line, index) in apiTestOutputLines" :key="index" :class="line.class">
                {{ line.text }}
              </div>

              <div v-if="apiTestStreamingContent" class="whitespace-pre-wrap text-green-400">
                {{ apiTestStreamingContent }}
              </div>

              <div
                v-if="apiTestStatus === 'success'"
                class="mt-3 flex items-center gap-2 border-t border-gray-700 pt-3 text-green-400"
              >
                <Icon name="check" size="sm" :stroke-width="2" />
                <span>{{ t('keys.useKeyModal.apiTest.completed') }}</span>
              </div>
              <div
                v-else-if="apiTestStatus === 'error'"
                class="mt-3 flex items-center gap-2 border-t border-gray-700 pt-3 text-red-400"
              >
                <Icon name="x" size="sm" :stroke-width="2" />
                <span>{{ apiTestErrorMessage }}</span>
              </div>
            </div>

            <button
              v-if="apiTestOutputLines.length > 0 || apiTestStreamingContent"
              @click="copyApiTestOutput"
              class="absolute right-2 top-2 rounded-lg bg-gray-800/80 p-1.5 text-gray-400 opacity-0 transition-all hover:bg-gray-700 hover:text-white group-hover:opacity-100"
              :title="t('keys.useKeyModal.apiTest.copyOutput')"
            >
              <Icon name="link" size="sm" :stroke-width="2" />
            </button>
          </div>

          <div class="flex flex-wrap justify-end gap-3">
            <button
              type="button"
              class="btn btn-secondary"
              :disabled="apiTestStatus === 'connecting'"
              @click="resetApiTestState"
            >
              {{ t('keys.useKeyModal.apiTest.clear') }}
            </button>
            <button
              type="button"
              :disabled="apiTestStatus === 'connecting' || !apiTestModel"
              :class="[
                'btn',
                apiTestStatus === 'success'
                  ? 'bg-green-500 text-white hover:bg-green-600'
                  : apiTestStatus === 'error'
                    ? 'bg-orange-500 text-white hover:bg-orange-600'
                    : 'btn-primary',
                (apiTestStatus === 'connecting' || !apiTestModel) && 'cursor-not-allowed opacity-70'
              ]"
              @click="startApiTest"
            >
              <Icon
                v-if="apiTestStatus === 'connecting'"
                name="refresh"
                size="sm"
                class="mr-2 animate-spin"
                :stroke-width="2"
              />
              <Icon
                v-else
                name="play"
                size="sm"
                class="mr-2"
                :stroke-width="2"
              />
              {{
                apiTestStatus === 'connecting'
                  ? t('keys.useKeyModal.apiTest.testing')
                  : apiTestStatus === 'idle'
                    ? t('keys.useKeyModal.apiTest.start')
                    : t('keys.useKeyModal.apiTest.retry')
              }}
            </button>
          </div>
        </div>

        <!-- Code Blocks (Stacked for multi-file platforms) -->
        <div class="space-y-4">
          <div
            v-for="(file, index) in currentFiles"
            :key="index"
            class="relative"
          >
            <!-- File Hint (if exists) -->
            <p v-if="file.hint" class="text-xs text-amber-600 dark:text-amber-400 mb-1.5 flex items-center gap-1">
              <Icon name="exclamationCircle" size="sm" class="flex-shrink-0" />
              {{ file.hint }}
            </p>
            <div class="bg-gray-900 dark:bg-dark-900 rounded-xl overflow-hidden">
              <!-- Code Header -->
              <div class="flex items-center justify-between px-4 py-2 bg-gray-800 dark:bg-dark-800 border-b border-gray-700 dark:border-dark-700">
                <span class="text-xs text-gray-400 font-mono">{{ file.path }}</span>
                <button
                  @click="copyContent(file.content, index)"
                  class="flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-lg transition-colors"
                  :class="copiedIndex === index
                    ? 'bg-green-500/20 text-green-400'
                    : 'bg-gray-700 hover:bg-gray-600 text-gray-300 hover:text-white'"
                >
                  <svg v-if="copiedIndex === index" class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                  </svg>
                  <svg v-else class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M15.666 3.888A2.25 2.25 0 0013.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 01-.75.75H9a.75.75 0 01-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 01-2.25 2.25H6.75A2.25 2.25 0 014.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 011.927-.184" />
                  </svg>
                  {{ copiedIndex === index ? t('keys.useKeyModal.copied') : t('keys.useKeyModal.copy') }}
                </button>
              </div>
              <!-- Code Content -->
              <pre class="p-4 text-sm font-mono text-gray-100 overflow-x-auto"><code v-if="file.highlighted" v-html="sanitizeHighlightedHtml(file.highlighted)"></code><code v-else v-text="file.content"></code></pre>
            </div>
          </div>
        </div>

        <!-- Usage Note -->
        <div v-if="showPlatformNote" class="flex items-start gap-3 p-3 rounded-lg bg-blue-50 dark:bg-blue-900/20 border border-blue-100 dark:border-blue-800">
          <Icon name="infoCircle" size="md" class="text-blue-500 flex-shrink-0 mt-0.5" />
          <p class="text-sm text-blue-700 dark:text-blue-300">
            {{ platformNote }}
          </p>
        </div>
      </template>
    </div>

    <template #footer>
      <div class="flex justify-end">
        <button
          @click="emit('close')"
          class="btn btn-secondary"
        >
          {{ t('common.close') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, computed, h, watch, nextTick, onBeforeUnmount, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'
import { sanitizeHtml } from '@/utils/sanitize'
import type { GroupPlatform } from '@/types'

interface UseKeyOption {
  id: number
  name: string
  key: string
  status: string
  platform: GroupPlatform | null
  groupName: string | null
}

interface Props {
  show: boolean
  apiKey: string
  apiKeys?: UseKeyOption[]
  selectedKeyId?: number | null
  baseUrl: string
  gatewayBaseUrl?: string
  platform: GroupPlatform | null
  allowMessagesDispatch?: boolean
}

interface Emits {
  (e: 'close'): void
  (e: 'update:selectedKeyId', value: number | null): void
}

interface TabConfig {
  id: string
  label: string
  icon: Component
}

interface FileConfig {
  path: string
  content: string
  hint?: string  // Optional hint message for this file
  highlighted?: string
}

interface ApiTestOutputLine {
  text: string
  class: string
}

type ApiTestProtocol = 'openai' | 'anthropic' | 'gemini'

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const { t } = useI18n()
const { copyToClipboard: clipboardCopy } = useClipboard()

const copiedIndex = ref<number | null>(null)
const activeTab = ref<string>('unix')
const activeClientTab = ref<string>('claude')
const apiTestTerminalRef = ref<HTMLElement | null>(null)
const apiTestStatus = ref<'idle' | 'connecting' | 'success' | 'error'>('idle')
const apiTestOutputLines = ref<ApiTestOutputLine[]>([])
const apiTestStreamingContent = ref('')
const apiTestErrorMessage = ref('')
const apiTestPrompt = ref('')
const apiTestProtocol = ref<ApiTestProtocol>('openai')
const apiTestModel = ref('')
let apiTestAbortController: AbortController | null = null

const maskKey = (value: string) => {
  if (!value) return ''
  if (value.length <= 12) return value
  return `${value.slice(0, 6)}...${value.slice(-4)}`
}

const apiKeyOptions = computed(() =>
  (props.apiKeys ?? []).map((key) => ({
    value: key.id,
    label: key.name,
    description: [key.key, key.groupName, key.platform].filter(Boolean).join(' '),
    maskedKey: maskKey(key.key),
    groupName: key.groupName,
    status: key.status
  }))
)

const updateSelectedKey = (value: string | number | boolean | null) => {
  emit('update:selectedKeyId', typeof value === 'number' ? value : null)
}

const defaultApiTestPrompt = computed(() => t('keys.useKeyModal.apiTest.defaultPrompt'))

const apiTestProtocolOptions = computed(() => {
  switch (props.platform) {
    case 'openai':
      return [{ value: 'openai', label: 'OpenAI' }]
    case 'gemini':
      return [{ value: 'gemini', label: 'Gemini' }]
    case 'antigravity':
      return [
        { value: 'anthropic', label: 'Claude' },
        { value: 'gemini', label: 'Gemini' }
      ]
    default:
      return [{ value: 'anthropic', label: 'Claude' }]
  }
})

const apiTestModelOptions = computed(() => {
  switch (apiTestProtocol.value) {
    case 'openai':
      return [
        { value: 'gpt-5.5', label: 'gpt-5.5' },
        { value: 'gpt-5.4', label: 'gpt-5.4' },
        { value: 'gpt-5.4-mini', label: 'gpt-5.4-mini' }
      ]
    case 'gemini':
      return [
        { value: 'gemini-2.0-flash', label: 'gemini-2.0-flash' },
        { value: 'gemini-2.5-flash', label: 'gemini-2.5-flash' },
        { value: 'gemini-2.5-pro', label: 'gemini-2.5-pro' },
        { value: 'gemini-3-pro-preview', label: 'gemini-3-pro-preview' }
      ]
    default:
      return [
        { value: props.platform === 'antigravity' ? 'claude-fable-5' : 'claude-sonnet-4-6', label: props.platform === 'antigravity' ? 'claude-fable-5' : 'claude-sonnet-4-6' },
        { value: 'claude-opus-4-6-thinking', label: 'claude-opus-4-6-thinking' },
        { value: 'claude-sonnet-4-6', label: 'claude-sonnet-4-6' }
      ]
  }
})

const apiTestStatusLabel = computed(() => {
  switch (apiTestStatus.value) {
    case 'connecting':
      return t('keys.useKeyModal.apiTest.statusConnecting')
    case 'success':
      return t('keys.useKeyModal.apiTest.statusSuccess')
    case 'error':
      return t('keys.useKeyModal.apiTest.statusError')
    default:
      return t('keys.useKeyModal.apiTest.statusIdle')
  }
})

// Reset tabs when platform changes
const defaultClientTab = computed(() => {
  switch (props.platform) {
    case 'openai':
      return 'codex'
    case 'gemini':
      return 'gemini'
    case 'antigravity':
      return 'claude'
    default:
      return 'claude'
  }
})

watch(() => props.platform, () => {
  activeTab.value = 'unix'
  activeClientTab.value = defaultClientTab.value
}, { immediate: true })

watch(
  () => props.platform,
  () => {
    apiTestProtocol.value = (apiTestProtocolOptions.value[0]?.value as ApiTestProtocol | undefined) ?? 'openai'
  },
  { immediate: true }
)

watch(apiTestProtocol, () => {
  apiTestModel.value = String(apiTestModelOptions.value[0]?.value ?? '')
  resetApiTestState()
}, { immediate: true })

watch(activeClientTab, (tab) => {
  if (tab === 'api-test' && !apiTestPrompt.value.trim()) {
    apiTestPrompt.value = defaultApiTestPrompt.value
  }
})

watch(() => props.apiKey, () => {
  resetApiTestState()
})

watch(() => props.show, (show) => {
  if (!show) {
    abortApiTest()
  }
})

// Reset shell tab when client changes
watch(activeClientTab, () => {
  activeTab.value = 'unix'
})

// Icon components
const AppleIcon = {
  render() {
    return h('svg', {
      fill: 'currentColor',
      viewBox: '0 0 24 24',
      class: 'w-4 h-4'
    }, [
      h('path', { d: 'M18.71 19.5c-.83 1.24-1.71 2.45-3.05 2.47-1.34.03-1.77-.79-3.29-.79-1.53 0-2 .77-3.27.82-1.31.05-2.3-1.32-3.14-2.53C4.25 17 2.94 12.45 4.7 9.39c.87-1.52 2.43-2.48 4.12-2.51 1.28-.02 2.5.87 3.29.87.78 0 2.26-1.07 3.81-.91.65.03 2.47.26 3.64 1.98-.09.06-2.17 1.28-2.15 3.81.03 3.02 2.65 4.03 2.68 4.04-.03.07-.42 1.44-1.38 2.83M13 3.5c.73-.83 1.94-1.46 2.94-1.5.13 1.17-.34 2.35-1.04 3.19-.69.85-1.83 1.51-2.95 1.42-.15-1.15.41-2.35 1.05-3.11z' })
    ])
  }
}

const WindowsIcon = {
  render() {
    return h('svg', {
      fill: 'currentColor',
      viewBox: '0 0 24 24',
      class: 'w-4 h-4'
    }, [
      h('path', { d: 'M3 12V6.75l6-1.32v6.48L3 12zm17-9v8.75l-10 .15V5.21L20 3zM3 13l6 .09v6.81l-6-1.15V13zm7 .25l10 .15V21l-10-1.91v-5.84z' })
    ])
  }
}

// Terminal icon for Claude Code
const TerminalIcon = {
  render() {
    return h('svg', {
      fill: 'none',
      stroke: 'currentColor',
      viewBox: '0 0 24 24',
      'stroke-width': '1.5',
      class: 'w-4 h-4'
    }, [
      h('path', {
        'stroke-linecap': 'round',
        'stroke-linejoin': 'round',
        d: 'm6.75 7.5 3 2.25-3 2.25m4.5 0h3m-9 8.25h13.5A2.25 2.25 0 0 0 21 17.25V6.75A2.25 2.25 0 0 0 18.75 4.5H5.25A2.25 2.25 0 0 0 3 6.75v10.5A2.25 2.25 0 0 0 5.25 20.25Z'
      })
    ])
  }
}

// Sparkle icon for Gemini
const SparkleIcon = {
  render() {
    return h('svg', {
      fill: 'none',
      stroke: 'currentColor',
      viewBox: '0 0 24 24',
      'stroke-width': '1.5',
      class: 'w-4 h-4'
    }, [
      h('path', {
        'stroke-linecap': 'round',
        'stroke-linejoin': 'round',
        d: 'M9.813 15.904 9 18.75l-.813-2.846a4.5 4.5 0 0 0-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 0 0 3.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 0 0 3.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 0 0-3.09 3.09ZM18.259 8.715 18 9.75l-.259-1.035a3.375 3.375 0 0 0-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 0 0 2.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 0 0 2.456 2.456L21.75 6l-1.035.259a3.375 3.375 0 0 0-2.456 2.456ZM16.894 20.567 16.5 21.75l-.394-1.183a2.25 2.25 0 0 0-1.423-1.423L13.5 18.75l1.183-.394a2.25 2.25 0 0 0 1.423-1.423l.394-1.183.394 1.183a2.25 2.25 0 0 0 1.423 1.423l1.183.394-1.183.394a2.25 2.25 0 0 0-1.423 1.423Z'
      })
    ])
  }
}

const clientTabs = computed((): TabConfig[] => {
  if (!props.platform) return []
  switch (props.platform) {
    case 'openai': {
      const tabs: TabConfig[] = [
        { id: 'codex', label: t('keys.useKeyModal.cliTabs.codexCli'), icon: TerminalIcon },
        { id: 'codex-ws', label: t('keys.useKeyModal.cliTabs.codexCliWs'), icon: TerminalIcon },
      ]
      if (props.allowMessagesDispatch) {
        tabs.push({ id: 'claude', label: t('keys.useKeyModal.cliTabs.claudeCode'), icon: TerminalIcon })
      }
      tabs.push(
        { id: 'opencode', label: t('keys.useKeyModal.cliTabs.opencode'), icon: TerminalIcon },
        { id: 'api-test', label: t('keys.useKeyModal.cliTabs.apiTest'), icon: TerminalIcon }
      )
      return tabs
    }
    case 'gemini':
      return [
        { id: 'gemini', label: t('keys.useKeyModal.cliTabs.geminiCli'), icon: SparkleIcon },
        { id: 'opencode', label: t('keys.useKeyModal.cliTabs.opencode'), icon: TerminalIcon },
        { id: 'api-test', label: t('keys.useKeyModal.cliTabs.apiTest'), icon: TerminalIcon }
      ]
    case 'antigravity':
      return [
        { id: 'claude', label: t('keys.useKeyModal.cliTabs.claudeCode'), icon: TerminalIcon },
        { id: 'gemini', label: t('keys.useKeyModal.cliTabs.geminiCli'), icon: SparkleIcon },
        { id: 'opencode', label: t('keys.useKeyModal.cliTabs.opencode'), icon: TerminalIcon },
        { id: 'api-test', label: t('keys.useKeyModal.cliTabs.apiTest'), icon: TerminalIcon }
      ]
    default:
      return [
        { id: 'claude', label: t('keys.useKeyModal.cliTabs.claudeCode'), icon: TerminalIcon },
        { id: 'opencode', label: t('keys.useKeyModal.cliTabs.opencode'), icon: TerminalIcon },
        { id: 'api-test', label: t('keys.useKeyModal.cliTabs.apiTest'), icon: TerminalIcon }
      ]
  }
})

// Shell tabs (3 types for environment variable based configs)
const shellTabs: TabConfig[] = [
  { id: 'unix', label: 'macOS / Linux', icon: AppleIcon },
  { id: 'cmd', label: 'Windows CMD', icon: WindowsIcon },
  { id: 'powershell', label: 'PowerShell', icon: WindowsIcon }
]

// OpenAI tabs (2 OS types)
const openaiTabs: TabConfig[] = [
  { id: 'unix', label: 'macOS / Linux', icon: AppleIcon },
  { id: 'windows', label: 'Windows', icon: WindowsIcon }
]

const noShellTabClients = new Set(['opencode', 'api-test'])

const showShellTabs = computed(() => !noShellTabClients.has(activeClientTab.value))

const currentTabs = computed(() => {
  if (!showShellTabs.value) return []
  if (activeClientTab.value === 'codex' || activeClientTab.value === 'codex-ws') {
    return openaiTabs
  }
  return shellTabs
})

const platformDescription = computed(() => {
  switch (props.platform) {
    case 'openai':
      if (activeClientTab.value === 'api-test') {
        return t('keys.useKeyModal.apiTest.openaiDescription')
      }
      if (activeClientTab.value === 'claude') {
        return t('keys.useKeyModal.description')
      }
      return t('keys.useKeyModal.openai.description')
    case 'gemini':
      if (activeClientTab.value === 'api-test') {
        return t('keys.useKeyModal.apiTest.geminiDescription')
      }
      return t('keys.useKeyModal.gemini.description')
    case 'antigravity':
      if (activeClientTab.value === 'api-test') {
        return t('keys.useKeyModal.apiTest.antigravityDescription')
      }
      return t('keys.useKeyModal.antigravity.description')
    default:
      if (activeClientTab.value === 'api-test') {
        return t('keys.useKeyModal.apiTest.description')
      }
      return t('keys.useKeyModal.description')
  }
})

const platformNote = computed(() => {
  switch (props.platform) {
    case 'openai':
      if (activeClientTab.value === 'claude') {
        return t('keys.useKeyModal.note')
      }
      return activeTab.value === 'windows'
        ? t('keys.useKeyModal.openai.noteWindows')
        : t('keys.useKeyModal.openai.note')
    case 'gemini':
      return t('keys.useKeyModal.gemini.note')
    case 'antigravity':
      return activeClientTab.value === 'claude'
        ? t('keys.useKeyModal.antigravity.claudeNote')
        : t('keys.useKeyModal.antigravity.geminiNote')
    default:
      return t('keys.useKeyModal.note')
  }
})

const showPlatformNote = computed(() => !noShellTabClients.has(activeClientTab.value))

const escapeHtml = (value: string) => value
  .replace(/&/g, '&amp;')
  .replace(/</g, '&lt;')
  .replace(/>/g, '&gt;')
  .replace(/"/g, '&quot;')
  .replace(/'/g, '&#39;')

const wrapToken = (className: string, value: string) =>
  `<span class="${className}">${escapeHtml(value)}</span>`

const keyword = (value: string) => wrapToken('text-emerald-300', value)
const variable = (value: string) => wrapToken('text-sky-200', value)
const operator = (value: string) => wrapToken('text-slate-400', value)
const string = (value: string) => wrapToken('text-amber-200', value)
const comment = (value: string) => wrapToken('text-slate-500', value)

const sanitizeHighlightedHtml = (value: string) => sanitizeHtml(value)

// Syntax highlighting helpers
// Generate file configs based on platform and active tab
const currentFiles = computed((): FileConfig[] => {
  const baseUrl = props.baseUrl || window.location.origin
  const apiKey = props.apiKey
  const baseRoot = baseUrl.replace(/\/v1\/?$/, '').replace(/\/+$/, '')
  const gatewayRoot = (props.gatewayBaseUrl || baseUrl || window.location.origin)
    .replace(/\/v1\/?$/, '')
    .replace(/\/+$/, '')
  const ensureV1 = (value: string) => {
    const trimmed = value.replace(/\/+$/, '')
    return trimmed.endsWith('/v1') ? trimmed : `${trimmed}/v1`
  }
  const apiBase = ensureV1(baseRoot)
  const antigravityBase = ensureV1(`${baseRoot}/antigravity`)
  const antigravityGeminiBase = (() => {
    const trimmed = `${baseRoot}/antigravity`.replace(/\/+$/, '')
    return trimmed.endsWith('/v1beta') ? trimmed : `${trimmed}/v1beta`
  })()
  const geminiBase = (() => {
    const trimmed = baseRoot.replace(/\/+$/, '')
    return trimmed.endsWith('/v1beta') ? trimmed : `${trimmed}/v1beta`
  })()
  const apiTestApiBase = ensureV1(gatewayRoot)
  const apiTestAntigravityGeminiBase = (() => {
    const trimmed = `${gatewayRoot}/antigravity`.replace(/\/+$/, '')
    return trimmed.endsWith('/v1beta') ? trimmed : `${trimmed}/v1beta`
  })()
  const apiTestGeminiBase = (() => {
    const trimmed = gatewayRoot.replace(/\/+$/, '')
    return trimmed.endsWith('/v1beta') ? trimmed : `${trimmed}/v1beta`
  })()

  if (activeClientTab.value === 'api-test') {
    switch (props.platform) {
      case 'openai':
        return generateOpenAIApiTestFiles(apiTestApiBase, apiKey)
      case 'gemini':
        return [generateGeminiApiTestFile(apiTestGeminiBase, apiKey)]
      case 'antigravity':
        return [
          generateAnthropicApiTestFile(`${gatewayRoot}/antigravity`, apiKey, 'claude-fable-5', 'Claude'),
          generateGeminiApiTestFile(apiTestAntigravityGeminiBase, apiKey, 'gemini-2.5-flash', 'Gemini')
        ]
      default:
        return [generateAnthropicApiTestFile(gatewayRoot, apiKey, 'claude-sonnet-4-6')]
    }
  }

  if (activeClientTab.value === 'opencode') {
    switch (props.platform) {
      case 'anthropic':
        return [generateOpenCodeConfig('anthropic', apiBase, apiKey)]
      case 'openai':
        return [generateOpenCodeConfig('openai', apiBase, apiKey)]
      case 'gemini':
        return [generateOpenCodeConfig('gemini', geminiBase, apiKey)]
      case 'antigravity':
        return [
          generateOpenCodeConfig('antigravity-claude', antigravityBase, apiKey, 'opencode.json (Claude)'),
          generateOpenCodeConfig('antigravity-gemini', antigravityGeminiBase, apiKey, 'opencode.json (Gemini)')
        ]
      default:
        return [generateOpenCodeConfig('openai', apiBase, apiKey)]
    }
  }

  switch (props.platform) {
    case 'openai':
      if (activeClientTab.value === 'claude') {
        return generateAnthropicFiles(baseUrl, apiKey)
      }
      if (activeClientTab.value === 'codex-ws') {
        return generateOpenAIWsFiles(baseUrl, apiKey)
      }
      return generateOpenAIFiles(baseUrl, apiKey)
    case 'gemini':
      return [generateGeminiCliContent(baseUrl, apiKey)]
    case 'antigravity':
      if (activeClientTab.value === 'gemini') {
        return [generateGeminiCliContent(`${baseUrl}/antigravity`, apiKey)]
      }
      return generateAnthropicFiles(`${baseUrl}/antigravity`, apiKey)
    default:
      return generateAnthropicFiles(baseUrl, apiKey)
  }
})

function generateOpenAIApiTestFiles(baseUrl: string, apiKey: string): FileConfig[] {
  const listModels = `curl "${baseUrl}/models" \\
  -H "Authorization: Bearer ${apiKey}"`

  const chatCompletions = `curl "${baseUrl}/chat/completions" \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer ${apiKey}" \\
  -d '{
    "model": "gpt-5.5",
    "messages": [
      {
        "role": "user",
        "content": "Say hello in one short sentence."
      }
    ],
    "stream": false
  }'`

  return [
    {
      path: 'GET /v1/models',
      content: listModels,
      hint: t('keys.useKeyModal.apiTest.listModelsHint')
    },
    {
      path: 'POST /v1/chat/completions',
      content: chatCompletions,
      hint: t('keys.useKeyModal.apiTest.requestHint')
    }
  ]
}

function generateAnthropicApiTestFile(baseUrl: string, apiKey: string, model: string, label?: string): FileConfig {
  const endpointBase = baseUrl.replace(/\/+$/, '')
  return {
    path: label ? `POST /v1/messages (${label})` : 'POST /v1/messages',
    content: `curl "${endpointBase}/v1/messages" \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer ${apiKey}" \\
  -H "anthropic-version: 2023-06-01" \\
  -d '{
    "model": "${model}",
    "max_tokens": 128,
    "messages": [
      {
        "role": "user",
        "content": "Say hello in one short sentence."
      }
    ]
  }'`,
    hint: t('keys.useKeyModal.apiTest.requestHint')
  }
}

function generateGeminiApiTestFile(baseUrl: string, apiKey: string, model = 'gemini-2.0-flash', label?: string): FileConfig {
  const endpointBase = baseUrl.replace(/\/+$/, '')
  return {
    path: label ? `POST /v1beta/models:generateContent (${label})` : 'POST /v1beta/models:generateContent',
    content: `curl "${endpointBase}/models/${model}:generateContent" \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer ${apiKey}" \\
  -d '{
    "contents": [
      {
        "role": "user",
        "parts": [
          {
            "text": "Say hello in one short sentence."
          }
        ]
      }
    ]
  }'`,
    hint: t('keys.useKeyModal.apiTest.requestHint')
  }
}

const scrollApiTestToBottom = async () => {
  await nextTick()
  if (apiTestTerminalRef.value) {
    apiTestTerminalRef.value.scrollTop = apiTestTerminalRef.value.scrollHeight
  }
}

const addApiTestLine = (text: string, className = 'text-gray-300') => {
  apiTestOutputLines.value.push({ text, class: className })
  scrollApiTestToBottom()
}

function resetApiTestState() {
  apiTestStatus.value = 'idle'
  apiTestOutputLines.value = []
  apiTestStreamingContent.value = ''
  apiTestErrorMessage.value = ''
}

function abortApiTest() {
  if (apiTestAbortController) {
    apiTestAbortController.abort()
    apiTestAbortController = null
  }
}

const getApiTestBaseUrl = () => {
  const baseRoot = window.location.origin
    .replace(/\/v1\/?$/, '')
    .replace(/\/+$/, '')

  if (props.platform === 'antigravity') {
    if (apiTestProtocol.value === 'gemini') return `${baseRoot}/antigravity/v1beta`
    return `${baseRoot}/antigravity`
  }

  if (apiTestProtocol.value === 'gemini') return `${baseRoot}/v1beta`
  if (apiTestProtocol.value === 'openai') return `${baseRoot}/v1`
  return baseRoot
}

const getDisplayedApiTestUrl = () => {
  const files = currentFiles.value
  const requestFile = files.find((file) => file.path.startsWith('POST '))
  if (!requestFile) return ''
  const match = requestFile.content.match(/curl "([^"]+)"/)
  return match?.[1] || ''
}

const getApiTestUrl = () => {
  const baseUrl = getApiTestBaseUrl().replace(/\/+$/, '')
  switch (apiTestProtocol.value) {
    case 'openai':
      return `${baseUrl}/chat/completions`
    case 'gemini':
      return `${baseUrl}/models/${apiTestModel.value}:generateContent`
    default:
      return `${baseUrl}/v1/messages`
  }
}

const buildApiTestBody = () => {
  const prompt = apiTestPrompt.value.trim() || defaultApiTestPrompt.value
  switch (apiTestProtocol.value) {
    case 'openai':
      return {
        model: apiTestModel.value,
        messages: [{ role: 'user', content: prompt }],
        stream: false
      }
    case 'gemini':
      return {
        contents: [
          {
            role: 'user',
            parts: [{ text: prompt }]
          }
        ]
      }
    default:
      return {
        model: apiTestModel.value,
        max_tokens: 128,
        messages: [{ role: 'user', content: prompt }]
      }
  }
}

const extractApiTestText = (payload: unknown): string => {
  if (!payload || typeof payload !== 'object') return ''
  const data = payload as Record<string, any>

  if (Array.isArray(data.choices)) {
    return data.choices
      .map((choice) => choice?.message?.content ?? choice?.delta?.content ?? choice?.text ?? '')
      .filter(Boolean)
      .join('\n')
  }

  if (Array.isArray(data.content)) {
    return data.content
      .map((part) => typeof part === 'string' ? part : part?.text ?? '')
      .filter(Boolean)
      .join('\n')
  }

  if (Array.isArray(data.candidates)) {
    return data.candidates
      .flatMap((candidate) => candidate?.content?.parts ?? [])
      .map((part) => part?.text ?? '')
      .filter(Boolean)
      .join('\n')
  }

  if (typeof data.output_text === 'string') return data.output_text
  if (typeof data.text === 'string') return data.text
  return ''
}

const extractApiTestError = (payload: unknown) => {
  if (!payload || typeof payload !== 'object') return ''
  const data = payload as Record<string, any>
  if (typeof data.error === 'string') return data.error
  if (data.error?.message) return String(data.error.message)
  if (data.message) return String(data.message)
  return ''
}

const startApiTest = async () => {
  if (!props.apiKey || !apiTestModel.value) return

  abortApiTest()
  resetApiTestState()
  apiTestStatus.value = 'connecting'
  apiTestAbortController = new AbortController()

  const url = getApiTestUrl()
  const displayedUrl = getDisplayedApiTestUrl()
  addApiTestLine(t('keys.useKeyModal.apiTest.starting'), 'text-blue-400')
  addApiTestLine(`${apiTestProtocol.value.toUpperCase()} ${apiTestModel.value}`, 'text-cyan-400')
  if (displayedUrl && displayedUrl !== url) {
    addApiTestLine(t('keys.useKeyModal.apiTest.displayEndpoint', { url: displayedUrl }), 'text-gray-400')
  }
  addApiTestLine(t('keys.useKeyModal.apiTest.actualRequestEndpoint', { url }), 'text-gray-400')
  addApiTestLine('', 'text-gray-300')

  try {
    const response = await fetch(url, {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${props.apiKey}`,
        'Content-Type': 'application/json',
        ...(apiTestProtocol.value === 'anthropic' ? { 'anthropic-version': '2023-06-01' } : {})
      },
      body: JSON.stringify(buildApiTestBody()),
      signal: apiTestAbortController.signal
    })

    const rawText = await response.text()
    let payload: unknown = null
    if (rawText) {
      try {
        payload = JSON.parse(rawText)
      } catch {
        payload = rawText
      }
    }

    addApiTestLine(`HTTP ${response.status} ${response.statusText}`, response.ok ? 'text-green-400' : 'text-red-400')

    if (!response.ok) {
      const errorText = extractApiTestError(payload) || rawText || t('keys.useKeyModal.apiTest.requestFailed')
      apiTestStatus.value = 'error'
      apiTestErrorMessage.value = errorText
      addApiTestLine(errorText, 'text-red-400')
      return
    }

    const responseText = extractApiTestText(payload)
    if (responseText) {
      addApiTestLine(t('keys.useKeyModal.apiTest.response'), 'text-yellow-400')
      apiTestStreamingContent.value = responseText
    } else if (rawText) {
      addApiTestLine(t('keys.useKeyModal.apiTest.rawResponse'), 'text-yellow-400')
      apiTestStreamingContent.value = rawText.length > 1200 ? `${rawText.slice(0, 1200)}...` : rawText
    } else {
      addApiTestLine(t('keys.useKeyModal.apiTest.emptyResponse'), 'text-yellow-400')
    }

    apiTestStatus.value = 'success'
    scrollApiTestToBottom()
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') {
      apiTestStatus.value = 'idle'
      return
    }
    const message = error instanceof Error ? error.message : String(error)
    apiTestStatus.value = 'error'
    apiTestErrorMessage.value = message
    addApiTestLine(`Error: ${message}`, 'text-red-400')
  } finally {
    apiTestAbortController = null
  }
}

const copyApiTestOutput = async () => {
  const content = [
    ...apiTestOutputLines.value.map((line) => line.text),
    apiTestStreamingContent.value
  ].filter(Boolean).join('\n')
  await clipboardCopy(content, t('keys.copied'))
}

onBeforeUnmount(() => {
  abortApiTest()
})

function generateAnthropicFiles(baseUrl: string, apiKey: string): FileConfig[] {
  let path: string
  let content: string

  switch (activeTab.value) {
    case 'unix':
      path = 'Terminal'
      content = `export ANTHROPIC_BASE_URL="${baseUrl}"
export ANTHROPIC_AUTH_TOKEN="${apiKey}"
export CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1`
      break
    case 'cmd':
      path = 'Command Prompt'
      content = `set ANTHROPIC_BASE_URL=${baseUrl}
set ANTHROPIC_AUTH_TOKEN=${apiKey}
set CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1`
      break
    case 'powershell':
      path = 'PowerShell'
      content = `$env:ANTHROPIC_BASE_URL="${baseUrl}"
$env:ANTHROPIC_AUTH_TOKEN="${apiKey}"
$env:CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1`
      break
    default:
      path = 'Terminal'
      content = ''
  }

  const vscodeSettingsPath = activeTab.value === 'unix'
    ? '~/.claude/settings.json'
    : '%userprofile%\\.claude\\settings.json'

  const vscodeContent = `{
  "env": {
    "ANTHROPIC_BASE_URL": "${baseUrl}",
    "ANTHROPIC_AUTH_TOKEN": "${apiKey}",
    "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
    "CLAUDE_CODE_ATTRIBUTION_HEADER": "0"
  }
}`

  return [
    { path, content },
    { path: vscodeSettingsPath, content: vscodeContent, hint: 'VSCode Claude Code' }
  ]
}

function generateGeminiCliContent(baseUrl: string, apiKey: string): FileConfig {
  const model = 'gemini-2.0-flash'
  const modelComment = t('keys.useKeyModal.gemini.modelComment')
  let path: string
  let content: string
  let highlighted: string

  switch (activeTab.value) {
    case 'unix':
      path = 'Terminal'
      content = `export GOOGLE_GEMINI_BASE_URL="${baseUrl}"
export GEMINI_API_KEY="${apiKey}"
export GEMINI_MODEL="${model}"  # ${modelComment}`
      highlighted = `${keyword('export')} ${variable('GOOGLE_GEMINI_BASE_URL')}${operator('=')}${string(`"${baseUrl}"`)}
${keyword('export')} ${variable('GEMINI_API_KEY')}${operator('=')}${string(`"${apiKey}"`)}
${keyword('export')} ${variable('GEMINI_MODEL')}${operator('=')}${string(`"${model}"`)}  ${comment(`# ${modelComment}`)}`
      break
    case 'cmd':
      path = 'Command Prompt'
      content = `set GOOGLE_GEMINI_BASE_URL=${baseUrl}
set GEMINI_API_KEY=${apiKey}
set GEMINI_MODEL=${model}`
      highlighted = `${keyword('set')} ${variable('GOOGLE_GEMINI_BASE_URL')}${operator('=')}${string(baseUrl)}
${keyword('set')} ${variable('GEMINI_API_KEY')}${operator('=')}${string(apiKey)}
${keyword('set')} ${variable('GEMINI_MODEL')}${operator('=')}${string(model)}
${comment(`REM ${modelComment}`)}`
      break
    case 'powershell':
      path = 'PowerShell'
      content = `$env:GOOGLE_GEMINI_BASE_URL="${baseUrl}"
$env:GEMINI_API_KEY="${apiKey}"
$env:GEMINI_MODEL="${model}"  # ${modelComment}`
      highlighted = `${keyword('$env:')}${variable('GOOGLE_GEMINI_BASE_URL')}${operator('=')}${string(`"${baseUrl}"`)}
${keyword('$env:')}${variable('GEMINI_API_KEY')}${operator('=')}${string(`"${apiKey}"`)}
${keyword('$env:')}${variable('GEMINI_MODEL')}${operator('=')}${string(`"${model}"`)}  ${comment(`# ${modelComment}`)}`
      break
    default:
      path = 'Terminal'
      content = ''
      highlighted = ''
  }

  return { path, content, highlighted }
}

function generateOpenAIFiles(baseUrl: string, apiKey: string): FileConfig[] {
  const isWindows = activeTab.value === 'windows'
  const configDir = isWindows ? '%userprofile%\\.codex' : '~/.codex'

  // config.toml content
  const configContent = `model_provider = "OpenAI"
model = "gpt-5.5"
review_model = "gpt-5.5"
model_reasoning_effort = "xhigh"
disable_response_storage = true
network_access = "enabled"
windows_wsl_setup_acknowledged = true

[model_providers.OpenAI]
name = "OpenAI"
base_url = "${baseUrl}"
wire_api = "responses"
requires_openai_auth = true

[features]
goals = true`

  // auth.json content
  const authContent = `{
  "OPENAI_API_KEY": "${apiKey}"
}`

  return [
    {
      path: `${configDir}/config.toml`,
      content: configContent,
      hint: t('keys.useKeyModal.openai.configTomlHint')
    },
    {
      path: `${configDir}/auth.json`,
      content: authContent
    }
  ]
}

function generateOpenAIWsFiles(baseUrl: string, apiKey: string): FileConfig[] {
  const isWindows = activeTab.value === 'windows'
  const configDir = isWindows ? '%userprofile%\\.codex' : '~/.codex'

  // config.toml content with WebSocket v2
  const configContent = `model_provider = "OpenAI"
model = "gpt-5.5"
review_model = "gpt-5.5"
model_reasoning_effort = "xhigh"
disable_response_storage = true
network_access = "enabled"
windows_wsl_setup_acknowledged = true

[model_providers.OpenAI]
name = "OpenAI"
base_url = "${baseUrl}"
wire_api = "responses"
supports_websockets = true
requires_openai_auth = true

[features]
responses_websockets_v2 = true
goals = true`

  // auth.json content
  const authContent = `{
  "OPENAI_API_KEY": "${apiKey}"
}`

  return [
    {
      path: `${configDir}/config.toml`,
      content: configContent,
      hint: t('keys.useKeyModal.openai.configTomlHint')
    },
    {
      path: `${configDir}/auth.json`,
      content: authContent
    }
  ]
}

function generateOpenCodeConfig(platform: string, baseUrl: string, apiKey: string, pathLabel?: string): FileConfig {
  const provider: Record<string, any> = {
    [platform]: {
      options: {
        baseURL: baseUrl,
        apiKey
      }
    }
  }
  const openaiModels = {
    'gpt-5.5': {
      name: 'GPT-5.5',
      limit: {
        context: 1050000,
        output: 128000
      },
      options: {
        store: false
      },
      variants: {
        low: {},
        medium: {},
        high: {},
        xhigh: {}
      }
    },
    'gpt-5.4': {
      name: 'GPT-5.4',
      limit: {
        context: 1050000,
        output: 128000
      },
      options: {
        store: false
      },
      variants: {
        low: {},
        medium: {},
        high: {},
        xhigh: {}
      }
    },
    'gpt-5.4-mini': {
      name: 'GPT-5.4 Mini',
      limit: {
        context: 400000,
        output: 128000
      },
      options: {
        store: false
      },
      variants: {
        low: {},
        medium: {},
        high: {},
        xhigh: {}
      }
    }
  }
  const geminiModels = {
    'gemini-2.0-flash': {
      name: 'Gemini 2.0 Flash',
      limit: {
        context: 1048576,
        output: 65536
      },
      modalities: {
        input: ['text', 'image', 'pdf'],
        output: ['text']
      }
    },
    'gemini-2.5-flash': {
      name: 'Gemini 2.5 Flash',
      limit: {
        context: 1048576,
        output: 65536
      },
      modalities: {
        input: ['text', 'image', 'pdf'],
        output: ['text']
      }
    },
    'gemini-2.5-pro': {
      name: 'Gemini 2.5 Pro',
      limit: {
        context: 2097152,
        output: 65536
      },
      modalities: {
        input: ['text', 'image', 'pdf'],
        output: ['text']
      },
      options: {
        thinking: {
          budgetTokens: 24576,
          type: 'enabled'
        }
      }
    },
    'gemini-3.5-flash': {
      name: 'Gemini 3.5 Flash',
      limit: {
        context: 1048576,
        output: 65536
      },
      modalities: {
        input: ['text', 'image', 'pdf'],
        output: ['text']
      }
    },
    'gemini-3-flash-preview': {
      name: 'Gemini 3 Flash Preview',
      limit: {
        context: 1048576,
        output: 65536
      },
      modalities: {
        input: ['text', 'image', 'pdf'],
        output: ['text']
      }
    },
    'gemini-3-pro-preview': {
      name: 'Gemini 3 Pro Preview',
      limit: {
        context: 1048576,
        output: 65536
      },
      modalities: {
        input: ['text', 'image', 'pdf'],
        output: ['text']
      },
      options: {
        thinking: {
          budgetTokens: 24576,
          type: 'enabled'
        }
      }
    },
    'gemini-3.1-pro-preview': {
      name: 'Gemini 3.1 Pro Preview',
      limit: {
        context: 1048576,
        output: 65536
      },
      modalities: {
        input: ['text', 'image', 'pdf'],
        output: ['text']
      },
      options: {
        thinking: {
          budgetTokens: 24576,
          type: 'enabled'
        }
      }
    }
  }

  const antigravityGeminiModels = {
    'gemini-2.5-flash': {
      name: 'Gemini 2.5 Flash',
      limit: {
        context: 1048576,
        output: 65536
      },
      modalities: {
        input: ['text', 'image', 'pdf'],
        output: ['text']
      },
      options: {
        thinking: {
          budgetTokens: 24576,
          type: 'disable'
        }
      }
    },
    'gemini-2.5-flash-lite': {
      name: 'Gemini 2.5 Flash Lite',
      limit: {
        context: 1048576,
        output: 65536
      },
      modalities: {
        input: ['text', 'image', 'pdf'],
        output: ['text']
      },
      options: {
        thinking: {
          budgetTokens: 24576,
          type: 'enabled'
        }
      }
    },
    'gemini-2.5-flash-thinking': {
      name: 'Gemini 2.5 Flash (Thinking)',
      limit: {
        context: 1048576,
        output: 65536
      },
      modalities: {
        input: ['text', 'image', 'pdf'],
        output: ['text']
      },
      options: {
        thinking: {
          budgetTokens: 24576,
          type: 'enabled'
        }
      }
    },
    'gemini-3-flash': {
      name: 'Gemini 3 Flash',
      limit: {
        context: 1048576,
        output: 65536
      },
      modalities: {
        input: ['text', 'image', 'pdf'],
        output: ['text']
      },
      options: {
        thinking: {
          budgetTokens: 24576,
          type: 'enabled'
        }
      }
    },
    'gemini-3.1-pro-low': {
      name: 'Gemini 3.1 Pro Low',
      limit: {
        context: 1048576,
        output: 65536
      },
      modalities: {
        input: ['text', 'image', 'pdf'],
        output: ['text']
      },
      options: {
        thinking: {
          budgetTokens: 24576,
          type: 'enabled'
        }
      }
    },
    'gemini-3.1-pro-high': {
      name: 'Gemini 3.1 Pro High',
      limit: {
        context: 1048576,
        output: 65536
      },
      modalities: {
        input: ['text', 'image', 'pdf'],
        output: ['text']
      },
      options: {
        thinking: {
          budgetTokens: 24576,
          type: 'enabled'
        }
      }
    },
    'gemini-2.5-flash-image': {
      name: 'Gemini 2.5 Flash Image',
      limit: {
        context: 1048576,
        output: 65536
      },
      modalities: {
        input: ['text', 'image'],
        output: ['image']
      },
      options: {
        thinking: {
          budgetTokens: 24576,
          type: 'enabled'
        }
      }
    },
    'gemini-3.1-flash-image': {
      name: 'Gemini 3.1 Flash Image',
      limit: {
        context: 1048576,
        output: 65536
      },
      modalities: {
        input: ['text', 'image'],
        output: ['image']
      },
      options: {
        thinking: {
          budgetTokens: 24576,
          type: 'enabled'
        }
      }
    }
  }
  const claudeModels = {
    'claude-fable-5': {
      name: 'Claude Fable 5',
      limit: {
        context: 1048576,
        output: 128000
      },
      modalities: {
        input: ['text', 'image', 'pdf'],
        output: ['text']
      },
      options: {
        thinking: {
          type: 'adaptive'
        }
      }
    },
    'claude-opus-4-6-thinking': {
      name: 'Claude 4.6 Opus (Thinking)',
      limit: {
        context: 200000,
        output: 128000
      },
      modalities: {
        input: ['text', 'image', 'pdf'],
        output: ['text']
      },
      options: {
        thinking: {
          budgetTokens: 24576,
          type: 'enabled'
        }
      }
    },
    'claude-sonnet-4-6': {
      name: 'Claude 4.6 Sonnet',
      limit: {
        context: 200000,
        output: 64000
      },
      modalities: {
        input: ['text', 'image', 'pdf'],
        output: ['text']
      },
      options: {
        thinking: {
          budgetTokens: 24576,
          type: 'enabled'
        }
      }
    }
  }

  if (platform === 'gemini') {
    provider[platform].npm = '@ai-sdk/google'
    provider[platform].models = geminiModels
  } else if (platform === 'anthropic') {
    provider[platform].npm = '@ai-sdk/anthropic'
  } else if (platform === 'antigravity-claude') {
    provider[platform].npm = '@ai-sdk/anthropic'
    provider[platform].name = 'Antigravity (Claude)'
    provider[platform].models = claudeModels
  } else if (platform === 'antigravity-gemini') {
    provider[platform].npm = '@ai-sdk/google'
    provider[platform].name = 'Antigravity (Gemini)'
    provider[platform].models = antigravityGeminiModels
  } else if (platform === 'openai') {
    provider[platform].models = openaiModels
  }

  const agent =
    platform === 'openai'
      ? {
          build: {
            options: {
              store: false
            }
          },
          plan: {
            options: {
              store: false
            }
          }
        }
      : undefined

  const content = JSON.stringify(
    {
      provider,
      ...(agent ? { agent } : {}),
      $schema: 'https://opencode.ai/config.json'
    },
    null,
    2
  )

  return {
    path: pathLabel ?? 'opencode.json',
    content,
    hint: t('keys.useKeyModal.opencode.hint')
  }
}

const copyContent = async (content: string, index: number) => {
  const success = await clipboardCopy(content, t('keys.copied'))
  if (success) {
    copiedIndex.value = index
    setTimeout(() => {
      copiedIndex.value = null
    }, 2000)
  }
}
</script>
