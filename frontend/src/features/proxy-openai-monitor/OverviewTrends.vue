<template>
  <section class="card space-y-4 p-4 sm:p-5" :aria-label="text.title">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h3 class="font-semibold">{{ text.title }}</h3>
      <label class="flex items-center gap-2 text-sm">
        {{ text.metric }}
        <select v-model="metric" class="input w-auto">
          <option value="p95_ms">P95</option><option value="average_ms">{{ text.mean }}</option>
        </select>
      </label>
    </div>
    <p class="text-xs text-gray-500 dark:text-dark-400">{{ text.explanation }}</p>
    <details v-if="rows.length" class="text-sm">
      <summary class="cursor-pointer">{{ text.select }} ({{ selected.length }}/10) · {{ text.pageOnly }}</summary>
      <div class="mt-3 flex flex-wrap gap-2">
        <label v-for="row in rows" :key="row.id" class="flex max-w-full items-center gap-2 rounded-lg border border-gray-200 px-3 py-2 dark:border-dark-700" :title="row.name">
          <input type="checkbox" :checked="selected.includes(row.id)" :disabled="selected.length >= 10 && !selected.includes(row.id)" @change="state.toggle(row.id)" />
          <span class="max-w-64 truncate">{{ row.name }}</span>
        </label>
      </div>
    </details>
    <p v-if="error" role="alert" class="text-sm text-red-600">{{ text.error }} <button class="underline" @click="state.load()">{{ text.retry }}</button></p>
    <p v-if="busy" role="status" class="text-sm text-gray-500">{{ text.loading }}</p>
    <template v-if="data && data.series.length">
      <p class="text-xs text-gray-500 dark:text-dark-400">
        {{ date(data.from) }} – {{ date(data.to) }} · {{ text.bucket }} {{ data.bucket_seconds / 60 }} {{ text.minutes }}
      </p>
      <p v-if="!hasSamples" class="py-3 text-sm text-gray-500">{{ text.noSamples }}</p>
      <template v-else>
        <p v-if="!hasSuccesses" class="text-sm text-amber-600">{{ text.noSuccess }}</p>
        <h4 class="text-sm font-medium">{{ metric === 'p95_ms' ? text.p95 : text.average }}</h4>
        <div class="h-64 sm:h-72"><Line :data="latencyChart" :options="latencyOptions" :aria-label="metric === 'p95_ms' ? text.p95 : text.average" /></div>
        <h4 class="text-sm font-medium">{{ text.rate }}</h4>
        <div class="h-52 sm:h-60"><Line :data="rateChart" :options="rateOptions" :aria-label="text.rate" /></div>
      </template>
    </template>
    <p v-else-if="!busy && !error" class="py-6 text-center text-sm text-gray-500">{{ selected.length ? text.noSamples : text.choose }}</p>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, toRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { Chart as ChartJS, CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, type ChartOptions } from 'chart.js'
import { Line } from 'vue-chartjs'
import type { Overview } from './api'
import { useOverviewTrends } from './trend-state'
ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend)
const props = defineProps<{ overview: Overview | null }>()
const { locale } = useI18n()
const zh = computed(() => locale.value.startsWith('zh'))
const text = computed(() => zh.value ? {
  title: '线路稳定趋势', metric: '成功延迟', mean: '平均值',
  explanation: '延迟曲线越平稳，线路波动越小；P95 可帮助发现高延迟。成功率下降表示出现失败，断线表示缺测或没有成功延迟，不能当作 0。悬停可查看样本和失败数；点击图例可隐藏线路。',
  select: '选择对比线路', pageOnly: '当前页，最多 10 条', error: '趋势更新失败，已显示的数据为上次结果。', retry: '重试', loading: '加载趋势…',
  bucket: '每桶', minutes: '分钟', noSamples: '选定区间暂无样本；未采样、停机和跳轮均不计成败。',
  noSuccess: '所选线路在此区间没有成功样本；延迟曲线为空，请查看下方成功率。',
  p95: '成功延迟 P95（ms）', average: '成功平均延迟（ms）', rate: '连通成功率（%）', choose: '选择线路查看趋势', samples: '样本', failures: '失败'
} : {
  title: 'Connection stability trends', metric: 'Success latency', mean: 'Mean',
  explanation: 'Flatter latency curves mean less variation; P95 highlights high latency. Lower success rate means failures. Gaps mean missing samples or no successful latency, never zero. Hover for sample/failure counts; click a legend to hide a line.',
  select: 'Compare connections', pageOnly: 'Current page, up to 10', error: 'Trend refresh failed; displayed data is from the previous result.', retry: 'Retry', loading: 'Loading trends…',
  bucket: 'Bucket', minutes: 'minutes', noSamples: 'No samples in this range. Downtime, skipped rounds and missing samples are not failures.',
  noSuccess: 'No successful samples in this range. Latency is empty; see the success-rate chart below.',
  p95: 'Success latency P95 (ms)', average: 'Success mean latency (ms)', rate: 'Connectivity success rate (%)', choose: 'Select connections to view trends', samples: 'Samples', failures: 'Failures'
})
const state = useOverviewTrends(toRef(props, 'overview'))
const { selected, data, busy, error, rows } = state
const metric = ref<'p95_ms' | 'average_ms'>('p95_ms')
const date = (value: string) => new Date(value).toLocaleString(locale.value)
const colors = ['#2563eb', '#dc2626', '#059669', '#9333ea', '#ea580c', '#0891b2', '#db2777', '#4f46e5', '#65a30d', '#64748b']
const hasSamples = computed(() => data.value?.series.some(series => series.points.some(point => point.count > 0)))
const hasSuccesses = computed(() => data.value?.series.some(series => series.points.some(point => point.successes > 0)))
const shortName = (name: string) => Array.from(name).length > 22 ? `${Array.from(name).slice(0, 22).join('')}…` : name
const labels = computed(() => data.value?.series[0]?.points.map(point => new Date(point.at).toLocaleString(locale.value, {
  ...(data.value?.bucket_seconds === 3600 ? { month: 'numeric', day: 'numeric' } as const : {}), hour: '2-digit', minute: '2-digit'
})) || [])
const latencyChart = computed(() => ({ labels: labels.value, datasets: data.value?.series.map((series, index) => ({
  label: shortName(series.name), data: series.points.map(point => point[metric.value]), borderColor: colors[index],
  backgroundColor: colors[index], spanGaps: false, pointRadius: 2, borderWidth: 1.5, tension: 0
})) || [] }))
const rateChart = computed(() => ({ labels: labels.value, datasets: data.value?.series.map((series, index) => ({
  label: shortName(series.name), data: series.points.map(point => point.success_rate === null ? null : point.success_rate * 100),
  borderColor: colors[index], backgroundColor: colors[index], spanGaps: false, pointRadius: 2, borderWidth: 1.5, tension: 0
})) || [] }))
function options(rate: boolean): ChartOptions<'line'> {
  return {
    responsive: true, maintainAspectRatio: false, animation: false,
    interaction: { mode: 'index', intersect: false },
    plugins: { legend: { labels: { boxWidth: 12, font: { size: 11 } } }, tooltip: { callbacks: {
      label: context => `${data.value?.series[context.datasetIndex]?.name || context.dataset.label}: ${context.formattedValue}${rate ? '%' : 'ms'}`,
      afterLabel: context => {
        const point = data.value?.series[context.datasetIndex]?.points[context.dataIndex]
        return point ? `${text.value.samples}: ${point.count} · ${text.value.failures}: ${point.failures}` : ''
      }
    } } },
    scales: { x: { ticks: { maxTicksLimit: 6, maxRotation: 0 } }, y: { beginAtZero: true, ...(rate ? { max: 100 } : {}) } }
  }
}
const latencyOptions = computed(() => options(false))
const rateOptions = computed(() => options(true))
</script>
