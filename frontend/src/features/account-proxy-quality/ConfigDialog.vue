<template>
  <BaseDialog :show="!!account" :title="text.title" width="wide" :show-close-button="!saving" @close="close">
    <div class="space-y-4 text-sm">
      <p class="break-all font-medium">{{ account?.name }}</p>
      <p class="text-gray-500">{{ text.description }}</p>
      <p v-if="busy" role="status">{{ text.loading }}</p>
      <p v-if="error" role="alert" class="text-red-600">{{ text.error }}</p>
      <p v-if="saved" role="status" class="text-green-600">{{ text.saved }}</p>
      <template v-if="settings">
        <label class="flex items-center gap-2 font-medium"><input v-model="enabled" type="checkbox" :disabled="busy || saving" />{{ text.enabled }}</label>
        <dl class="grid gap-2 rounded-lg bg-gray-50 p-3 dark:bg-dark-800 sm:grid-cols-2">
          <div><dt class="text-xs text-gray-500">{{ text.current }}</dt><dd class="break-words">{{ proxyName(settings.current_proxy_id) }}</dd></div>
          <div><dt class="text-xs text-gray-500">{{ text.status }}</dt><dd>{{ reason(settings.reason) }}</dd></div>
          <div class="sm:col-span-2"><dt class="text-xs text-gray-500">{{ text.evaluated }}</dt><dd>{{ date(settings.last_evaluated_at) }}</dd></div>
        </dl>
        <fieldset :disabled="busy || saving" class="min-w-0 space-y-2">
          <legend class="mb-2 font-medium">{{ text.pool }} · {{ ids.length }}/10</legend>
          <input v-model="search" class="input" :placeholder="text.search" :aria-label="text.search" />
          <div class="max-h-52 space-y-1 overflow-y-auto rounded-lg border border-gray-200 p-2 dark:border-dark-700">
            <p v-if="!choices.length" class="p-2 text-gray-500">{{ text.empty }}</p>
            <label v-for="p in choices" :key="p.id" class="flex items-start gap-2 rounded p-2 hover:bg-gray-50 dark:hover:bg-dark-700">
              <input v-model="ids" type="checkbox" :value="p.id" class="mt-0.5" :disabled="!ids.includes(p.id) && ids.length >= 10" />
              <span class="min-w-0 break-words">{{ p.name }} <span class="text-xs text-gray-500">#{{ p.id }}{{ p.available ? '' : ` · ${text.unavailable}` }}</span>
                <span v-if="ids.includes(p.id)" class="mt-1 block text-xs text-gray-500">
                  <span v-if="!settings.proxy_ids.includes(p.id)">{{ text.saveForSamples }}</span>
                  <template v-else-if="samplesFor(p.id).length">
                    <span class="block">{{ text.sampleScope }}</span>
                    <span v-for="sample in samplesFor(p.id)" :key="`${sample.model}:${sample.transport}`" class="block break-all">{{ sample.model }} · {{ sample.transport || text.unknownTransport }} · {{ text.sampleCounts(sample.successes, sample.total, sample.first_tokens, sample.http_5xx) }}</span>
                  </template>
                  <span v-else>{{ text.noSamples }}</span>
                </span>
              </span>
            </label>
          </div>
        </fieldset>
        <p v-if="invalid" role="alert" class="text-amber-600">{{ text.invalid }}</p>
        <p class="text-xs text-gray-500">{{ text.hint }}</p>
        <details class="text-xs text-gray-500"><summary class="cursor-pointer">{{ text.rule }}</summary><p class="mt-2">{{ text.trigger }}</p><p class="mt-2">{{ text.metric }}</p></details>
        <div class="border-t border-gray-100 pt-3 dark:border-dark-700">
          <h4 class="mb-2 font-medium">{{ text.history }}</h4>
          <p v-if="!settings.events.length" class="text-xs text-gray-500">{{ text.noHistory }}</p>
          <ol v-else class="max-h-40 space-y-2 overflow-y-auto text-xs">
            <li v-for="(event, index) in settings.events" :key="index" class="break-words">{{ date(event.created_at) }} · {{ proxyName(event.from_proxy_id) }} → {{ proxyName(event.to_proxy_id) }}<span class="block text-gray-500">{{ reason(event.reason) }}</span></li>
          </ol>
        </div>
      </template>
    </div>
    <template #footer>
      <button class="btn btn-secondary" :disabled="busy || saving" @click="reload">{{ text.refresh }}</button>
      <button class="btn btn-primary" :disabled="!settings || busy || saving || invalid" @click="save">{{ saving ? text.saving : text.save }}</button>
    </template>
  </BaseDialog>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import type { Account } from '@/types'
import { useProxyQuality } from './state'
import { copy } from './text'
const props = defineProps<{ account: Account | null }>()
const emit = defineEmits<{ close: []; updated: [] }>()
const { locale } = useI18n()
const text = computed(() => locale.value.startsWith('zh') ? copy.zh : copy.en)
const state = useProxyQuality()
const { settings, enabled, ids, busy, saving, error, saved } = state
const search = ref('')
const invalid = computed(() => enabled.value && (ids.value.length < 2 || ids.value.length > 10 || !settings.value?.current_proxy_id || !ids.value.includes(settings.value.current_proxy_id)))
const choices = computed(() => {
  const all = new Map((settings.value?.choices || []).map(p => [p.id, { ...p, available: true }]))
  for (const id of ids.value) if (!all.has(id)) all.set(id, { id, name: `#${id}`, available: false })
  return [...all.values()].filter(p => `${p.name} ${p.id}`.toLowerCase().includes(search.value.toLowerCase()))
})
const proxyName = (id: number | null) => id == null ? '—' : settings.value?.choices.find(p => p.id === id)?.name || `#${id}`
const samplesFor = (id: number) => (settings.value?.samples || []).filter(sample => sample.proxy_id === id)
const reason = (value: string) => text.value.states[value as keyof typeof text.value.states] || value
const date = (value: string | null) => value ? new Date(value).toLocaleString(locale.value) : '—'
const reload = () => { if (props.account) void state.load(props.account.id) }
const close = () => { if (!saving.value) emit('close') }
const save = async () => { if (!invalid.value && await state.save()) emit('updated') }
watch(() => props.account?.id, () => { state.close(); search.value = ''; reload() }, { immediate: true })
</script>
