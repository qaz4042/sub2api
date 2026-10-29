<template>
  <AppLayout>
    <div class="space-y-4">
      <section>
        <div class="space-y-3">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <h2 class="text-lg font-semibold">{{ text.title }}</h2>
            <RouterLink to="/admin/proxies" class="btn btn-secondary">{{ text.back }}</RouterLink>
          </div>
          <p class="text-xs text-gray-500 dark:text-dark-400">{{ text.description }}</p>
          <form class="flex flex-wrap items-center gap-2" @submit.prevent="searchProxies">
            <input v-model="search" class="input w-full sm:w-52" :placeholder="text.search" :aria-label="text.search" maxlength="100" />
            <button type="submit" class="btn btn-secondary">{{ text.searchButton }}</button>
            <select v-model="query.hours" class="input w-auto" :aria-label="text.range" @change="state.reset()">
              <option :value="1">{{ detail.hour }}</option><option :value="24">{{ detail.day }}</option><option :value="168">{{ detail.week }}</option>
            </select>
            <select v-model="query.mode" class="input w-auto" :aria-label="text.filter" @change="state.reset()">
              <option value="all">{{ text.all }}</option><option value="enabled">{{ text.enabled }}</option><option value="paused">{{ text.paused }}</option>
            </select>
            <select v-model="query.sort" class="input w-auto" :aria-label="text.sort" @change="state.reset()">
              <option v-for="(label, value) in text.sorts" :key="value" :value="value">{{ label }}</option>
            </select>
            <button type="button" class="btn btn-secondary" :disabled="busy" @click="state.load()">{{ detail.refresh }}</button>
          </form>
          <p v-if="error" class="text-sm text-red-600" role="alert">{{ text.error }}</p>
          <p class="text-xs text-gray-500 dark:text-dark-400" role="status">
            {{ busy ? detail.loading : text.autoRefresh }}
            <span v-if="data"> · {{ text.updated }} {{ date(data.to) }} · {{ data.total }} {{ text.proxies }}</span>
          </p>
        </div>
      </section>
      <OverviewTrends :overview="data" />
      <section class="card overflow-hidden">
        <DataTable :columns="columns" :data="data?.rows || []" :loading="busy && !data" row-key="id">
          <template #cell-name="{ row }"><span class="inline-block max-w-64 break-words">{{ row.name }}</span></template>
          <template #cell-status="{ row }">
            <span class="badge" :class="badge(row)">{{ text.statuses[monitorStatus(row)] }}</span>
            <span v-if="row.config.enabled" class="ml-2 text-xs text-gray-500">{{ row.config.interval_seconds }}s</span>
          </template>
          <template #cell-latest="{ row }">
            <span :class="row.stale ? 'text-gray-400' : ''">{{ row.latest ? row.latest.success ? ms(row.latest.latency_ms) : `${text.failed} (${row.latest.http_status || row.latest.error_class})` : '—' }}</span>
          </template>
          <template #cell-rate="{ row }">{{ row.summary.success_rate === null ? '—' : `${(row.summary.success_rate * 100).toFixed(1)}%` }}</template>
          <template #cell-average="{ row }">{{ ms(row.summary.average_ms) }}</template>
          <template #cell-p95="{ row }">{{ ms(row.summary.p95_ms) }}</template>
          <template #cell-samples="{ row }">{{ row.summary.count }} / {{ row.summary.failures }}</template>
          <template #cell-checked="{ row }">{{ row.latest ? date(row.latest.started_at) : '—' }}</template>
          <template #cell-actions="{ row }"><MonitorDialog :proxy="row" :label="text.details" /></template>
          <template #empty><div class="p-8 text-center text-sm text-gray-500">{{ error ? text.error : text.empty }}</div></template>
        </DataTable>
      </section>
      <div>
        <Pagination v-if="data && data.total > 0" :page="data.page" :page-size="data.page_size" :total="data.total" :show-page-size-selector="false" @update:page="state.page" />
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import OverviewTrends from './OverviewTrends.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import MonitorDialog from './MonitorDialog.vue'
import { copy } from './text'
import { overviewCopy } from './overview-text'
import { monitorStatus, useOverview } from './overview-state'
import type { OverviewRow } from './api'
const { locale } = useI18n()
const text = computed(() => locale.value.startsWith('zh') ? overviewCopy.zh : overviewCopy.en)
const detail = computed(() => locale.value.startsWith('zh') ? copy.zh : copy.en)
const state = useOverview()
const { query, data, busy, error } = state
const search = ref('')
const searchProxies = () => { query.search = search.value.trim(); state.reset() }
const date = (value: string) => new Date(value).toLocaleString(locale.value)
const ms = (value: number | null) => value === null ? '—' : `${value.toFixed(0)}ms`
const badge = (row: OverviewRow) => {
  const status = monitorStatus(row)
  return status === 'success' ? 'badge-success' : status === 'failed' ? 'badge-danger' : 'badge-gray'
}
const columns = computed(() => [
  { key: 'name', label: text.value.name }, { key: 'status', label: text.value.state },
  { key: 'latest', label: text.value.latest }, { key: 'rate', label: detail.value.rate },
  { key: 'average', label: detail.value.average }, { key: 'p95', label: 'P95' },
  { key: 'samples', label: text.value.samples }, { key: 'checked', label: detail.value.last },
  { key: 'actions', label: text.value.details }
])
</script>
