<template>
  <section class="card p-4" :aria-label="t('admin.accounts.stats.errorStatistics')">
    <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
      {{ t('admin.accounts.stats.errorStatistics') }}
    </h3>
    <template v-if="stats">
      <div class="mt-4 grid grid-cols-2 gap-4">
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.accounts.stats.errorCount') }}
          </p>
          <p class="mt-1 text-2xl font-bold text-rose-600 dark:text-rose-400" data-testid="error-count">
            {{ stats.error_count.toLocaleString() }}
          </p>
        </div>
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.accounts.stats.errorRate') }}
          </p>
          <p class="mt-1 text-2xl font-bold text-gray-900 dark:text-white" data-testid="error-rate">
            {{ stats.error_rate == null ? '—' : stats.error_rate.toFixed(2) + '%' }}
          </p>
        </div>
      </div>
      <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.accounts.stats.errorRateBasis', { count: stats.total_requests.toLocaleString() }) }}
      </p>
      <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.accounts.stats.errorCoverage', { start: formatCoverage(stats.coverage_start), end: formatCoverage(stats.coverage_end) }) }}
      </p>
      <p v-if="stats.partial" class="mt-2 text-xs text-amber-600 dark:text-amber-400">
        {{ t('admin.accounts.stats.errorStatsPartial') }}
      </p>
      <p v-if="stats.uncorrelated_ws_errors > 0" class="mt-1 text-xs text-amber-600 dark:text-amber-400">
        {{ t('admin.accounts.stats.errorStatsLegacyWS') }}
      </p>
      <h4 class="mb-2 mt-4 text-xs font-medium text-gray-700 dark:text-gray-300">
        {{ t('admin.accounts.stats.errorTrend') }}
      </h4>
      <div v-if="stats.history.length" class="h-56">
        <Line :data="chartData" :options="chartOptions" />
      </div>
      <p class="mt-3 text-xs leading-relaxed text-gray-500 dark:text-gray-400">
        {{ t('admin.accounts.stats.errorStatsHint') }}
      </p>
    </template>
    <p v-else class="mt-3 text-sm text-gray-500 dark:text-gray-400">
      {{ t(unavailableMessage) }}
    </p>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Tooltip,
  Legend,
  type ChartOptions
} from 'chart.js'
import { Line } from 'vue-chartjs'
import type { AccountErrorStats } from '@/types'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend)

const props = defineProps<{ stats?: AccountErrorStats | null; unavailableReason?: string }>()
const { t } = useI18n()
const unavailableMessage = computed(() => {
  if (props.unavailableReason === 'monitoring_disabled') return 'admin.accounts.stats.errorMonitoringDisabled'
  if (props.unavailableReason === 'outside_retention') return 'admin.accounts.stats.errorOutsideRetention'
  return 'admin.accounts.stats.errorStatsUnavailable'
})
const formatCoverage = (value: string) => new Date(value).toLocaleString(undefined, {
  month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit'
})

const chartData = computed(() => ({
  labels: props.stats?.history.map((point) => point.label) ?? [],
  datasets: [
    {
      label: t('admin.accounts.stats.errorCount'),
      data: props.stats?.history.map((point) => point.error_count) ?? [],
      borderColor: '#e11d48',
      backgroundColor: '#e11d48',
      yAxisID: 'y',
      tension: 0.2
    },
    {
      label: t('admin.accounts.stats.errorRate'),
      data: props.stats?.history.map((point) => point.error_rate) ?? [],
      borderColor: '#f97316',
      backgroundColor: '#f97316',
      borderDash: [5, 5],
      yAxisID: 'rate',
      tension: 0.2
    }
  ]
}))

const chartOptions = computed<ChartOptions<'line'>>(() => {
  const dark = document.documentElement.classList.contains('dark')
  const text = dark ? '#e5e7eb' : '#374151'
  const grid = dark ? '#374151' : '#e5e7eb'
  const peakRate = Math.max(0, ...(props.stats?.history.map((point) => point.error_rate ?? 0) ?? []))
  return {
    responsive: true,
    maintainAspectRatio: false,
    interaction: { intersect: false, mode: 'index' },
    plugins: {
      legend: { labels: { color: text, usePointStyle: true, boxWidth: 8 } },
      tooltip: {
        callbacks: {
          label: (context) => {
            const value = context.parsed.y ?? 0
            return `${context.dataset.label}: ${context.dataset.yAxisID === 'rate' ? value.toFixed(2) + '%' : value.toLocaleString()}`
          }
        }
      }
    },
    scales: {
      x: { ticks: { color: text, maxRotation: 45 }, grid: { color: grid } },
      y: {
        beginAtZero: true,
        ticks: { color: text, precision: 0 },
        grid: { color: grid },
        title: { display: true, text: t('admin.accounts.stats.errorCount'), color: text }
      },
      rate: {
        position: 'right',
        min: 0,
        max: Math.min(100, Math.max(1, Math.ceil(peakRate * 1.2))),
        ticks: { color: text, callback: (value) => `${value}%` },
        grid: { drawOnChartArea: false },
        title: { display: true, text: t('admin.accounts.stats.errorRate'), color: text }
      }
    }
  }
})
</script>
