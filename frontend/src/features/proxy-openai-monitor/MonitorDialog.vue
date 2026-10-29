<template>
  <button type="button" class="text-xs text-primary-600 hover:underline dark:text-primary-400" @click="open = true">
    {{ text.monitor }}
  </button>
  <BaseDialog :show="open" :title="`${text.title} · ${proxy.name}`" width="extra-wide" @close="open = false">
    <div class="space-y-4">
      <p class="text-xs text-gray-500">{{ text.description }}</p>
      <div v-if="error || configError" class="text-sm text-red-600" role="alert">{{ text.error }}</div>
      <div v-if="saved" class="text-sm text-green-600" role="status">{{ text.saved }}</div>
      <div class="flex flex-wrap items-center gap-3">
        <label class="flex items-center gap-2 text-sm">
          <input v-model="config.enabled" type="checkbox" :disabled="!settings || saving" />{{ text.enabled }}
        </label>
        <label class="flex items-center gap-2 text-sm">
          {{ text.interval }}
          <select v-model="config.interval_seconds" class="input w-auto" :disabled="!settings || saving">
            <option v-for="n in [10, 30, 60]" :key="n" :value="n">{{ n }}s</option>
          </select>
        </label>
        <button class="btn btn-primary btn-sm" :disabled="!settings || saving || (config.enabled && !settings.eligible)" @click="state.save()">
          {{ saving ? text.saving : text.save }}
        </button>
        <span v-if="settings" class="text-xs text-gray-500">{{ text.status }}: {{ statusLabel }}</span>
      </div>
      <div class="flex flex-wrap items-center gap-3">
        <select v-model="hours" class="input w-auto" :aria-label="text.range" :disabled="saving">
          <option :value="1">{{ text.hour }}</option><option :value="24">{{ text.day }}</option><option :value="168">{{ text.week }}</option>
        </select>
        <button class="btn btn-secondary btn-sm" :disabled="busy || saving" @click="refresh">{{ text.refresh }}</button>
        <span class="text-xs text-gray-500">{{ text.autoRefresh }}</span>
      </div>
      <p v-if="busy" class="text-sm text-gray-500" role="status">{{ text.loading }}</p>
      <template v-if="report">
        <p class="text-sm" :class="report.stale ? 'text-amber-600' : 'text-gray-500'">
          {{ !report.latest ? text.noCurrent : report.stale ? text.stale : text.fresh }}
          <span v-if="report.latest">· {{ report.latest.success ? ms(report.latest.latency_ms) : report.latest.error_class }}</span>
          · {{ text.last }}: {{ report.latest ? date(report.latest.started_at) : '—' }}
        </p>
        <div class="grid grid-cols-2 gap-3 text-sm sm:grid-cols-5">
          <div>{{ text.samples }}<strong class="block">{{ report.summary.count }}</strong></div>
          <div>{{ text.rate }}<strong class="block">{{ report.summary.success_rate === null ? '—' : `${(report.summary.success_rate * 100).toFixed(1)}%` }}</strong></div>
          <div>{{ text.average }}<strong class="block">{{ ms(report.summary.average_ms) }}</strong></div>
          <div>P95<strong class="block">{{ ms(report.summary.p95_ms) }}</strong></div>
          <div>{{ text.failures }}<strong class="block">{{ report.summary.failures }}</strong></div>
        </div>
        <p class="text-xs text-gray-500">
          {{ date(report.from) }} – {{ date(report.to) }} · {{ text.coverage }}:
          {{ report.first_sample_at ? date(report.first_sample_at) : '—' }} – {{ report.last_sample_at ? date(report.last_sample_at) : '—' }}
        </p>
        <p v-if="!report.summary.count" class="py-4 text-center text-sm text-gray-500">{{ text.empty }}</p>
        <template v-else>
          <p v-if="!report.summary.successes" class="text-sm text-amber-600">{{ text.allFailed }}</p>
          <p class="text-xs text-gray-500">{{ text.granularity }}: {{ report.bucket_seconds === 60 ? text.minute : text.hourly }}</p>
          <div class="h-52"><Line :data="chartData" :options="chartOptions" /></div>
          <div class="overflow-x-auto">
            <table class="w-full whitespace-nowrap text-left text-xs">
              <thead><tr><th class="py-2">{{ text.time }}</th><th>{{ text.result }}</th><th>{{ text.latency }}</th><th>HTTP</th></tr></thead>
              <tbody><tr v-for="sample in report.records" :key="sample.id" class="border-t border-gray-100 dark:border-dark-700">
                <td class="py-2 pr-3">{{ date(sample.started_at) }}</td><td class="pr-3">{{ sample.success ? text.success : sample.error_class }}</td>
                <td class="pr-3">{{ ms(sample.latency_ms) }}</td><td>{{ sample.http_status || '—' }}</td>
              </tr></tbody>
            </table>
          </div>
          <div class="flex items-center justify-end gap-3 text-sm">
            <button class="btn btn-secondary btn-sm" :disabled="page === 1 || busy || saving" @click="page--">{{ text.previous }}</button>
            <span>{{ page }}</span>
            <button class="btn btn-secondary btn-sm" :disabled="!report.truncated || busy || saving" @click="page++">{{ text.next }}</button>
          </div>
        </template>
        <p class="text-xs text-gray-500">{{ text.staleRule.replace('{seconds}', String(report.stale_seconds)) }}</p>
      </template>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Chart as ChartJS, CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend } from 'chart.js'
import { Line } from 'vue-chartjs'
import BaseDialog from '@/components/common/BaseDialog.vue'
import type { Proxy } from '@/types'
import { useMonitor } from './state'
import { copy } from './text'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend)
const props = defineProps<{ proxy: Proxy }>()
const { locale } = useI18n()
const text = computed(() => locale.value.startsWith('zh') ? copy.zh : copy.en)
const open = ref(false)
const hours = ref(24)
const page = ref(1)
const state = useMonitor()
const { report, settings, config, busy, saving, error, configError, saved } = state
const date = (value: string) => new Date(value).toLocaleString(locale.value)
const ms = (value: number | null) => value === null ? '—' : `${value.toFixed(0)}ms`
const refresh = () => void state.load(props.proxy.id, hours.value, page.value)
const statusLabel = computed(() => {
  const value = settings.value?.config.runtime_state || 'paused'
  return text.value.states[value as keyof typeof text.value.states] || value
})
watch(() => [open.value, props.proxy.id], () => {
  state.close()
  if (open.value) { page.value = 1; void state.load(props.proxy.id, hours.value, 1, true) }
})
watch(hours, () => { page.value = 1; if (open.value) refresh() })
watch(page, () => { if (open.value) refresh() })
const chartData = computed(() => ({
  labels: report.value?.trend.map(b => new Date(b.at).toLocaleString(locale.value, {
    ...(report.value?.bucket_seconds === 3600 ? { month: 'numeric', day: 'numeric' } as const : {}),
    hour: '2-digit', minute: '2-digit'
  })) || [],
  datasets: [
    { label: text.value.average, data: report.value?.trend.map(b => b.average_ms) || [], borderColor: '#3b82f6', spanGaps: false, pointRadius: 2, yAxisID: 'y' },
    { label: text.value.failures, data: report.value?.trend.map(b => b.count ? b.failures : null) || [], borderColor: '#ef4444', spanGaps: false, pointRadius: 2, yAxisID: 'failures' }
  ]
}))
const chartOptions = computed(() => ({
  responsive: true, maintainAspectRatio: false,
  interaction: { mode: 'index' as const, intersect: false },
  scales: {
    x: { ticks: { maxTicksLimit: 6 } },
    y: { beginAtZero: true, title: { display: true, text: 'ms' } },
    failures: { position: 'right' as const, beginAtZero: true, ticks: { precision: 0 }, grid: { drawOnChartArea: false }, title: { display: true, text: text.value.failures } }
  }
}))
</script>
