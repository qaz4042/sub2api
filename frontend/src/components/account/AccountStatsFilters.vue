<template>
  <div class="space-y-2">
    <div class="flex flex-wrap items-end gap-3">
      <div class="w-40">
        <label for="account-stats-range" class="mb-1 block text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.accounts.stats.timeRange') }}
        </label>
        <Select
          id="account-stats-range"
          :model-value="days"
          :options="rangeOptions"
          :aria-label="t('admin.accounts.stats.timeRange')"
          @update:model-value="emit('update:days', Number($event))"
        />
      </div>
      <div class="w-40">
        <label for="account-stats-granularity" class="mb-1 block text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.accounts.stats.granularity') }}
        </label>
        <Select
          id="account-stats-granularity"
          :model-value="granularity"
          :options="granularityOptions"
          :aria-label="t('admin.accounts.stats.granularity')"
          @update:model-value="emit('update:granularity', $event === 'hour' ? 'hour' : 'day')"
        />
      </div>
      <label class="flex min-h-10 cursor-pointer items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
        <input
          type="checkbox"
          :checked="includeTests"
          class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-gray-600 dark:bg-dark-700"
          @change="emit('update:includeTests', ($event.target as HTMLInputElement).checked)"
        />
        {{ t('admin.accounts.stats.includeTests') }}
      </label>
    </div>
    <div v-if="includeTests" class="space-y-1 text-xs text-gray-500 dark:text-gray-400">
      <p>{{ t('admin.accounts.stats.includeTestsHint') }}</p>
      <p v-if="tests">
        {{ t('admin.accounts.stats.includedTests', { count: tests.total_requests.toLocaleString(), errors: tests.error_count.toLocaleString() }) }}
      </p>
      <p v-if="tests?.partial" class="text-amber-600 dark:text-amber-400">
        {{ t('admin.accounts.stats.testsPartial', { start: new Date(tests.complete_since).toLocaleString() }) }}
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
import type { AccountStatsGranularity, AccountUsageStatsResponse } from '@/types'

defineProps<{
  days: number
  granularity: AccountStatsGranularity
  includeTests: boolean
  tests?: AccountUsageStatsResponse['scheduled_tests']
}>()
const emit = defineEmits<{
  'update:days': [value: number]
  'update:granularity': [value: AccountStatsGranularity]
  'update:includeTests': [value: boolean]
}>()
const { t } = useI18n()
const rangeOptions = computed(() => [
  { value: 1, label: t('admin.accounts.stats.rangeToday') },
  ...[7, 30].map((days) => ({ value: days, label: t('admin.accounts.stats.rangeDays', { days }) }))
])
const granularityOptions = computed(() => [
  { value: 'day', label: t('admin.accounts.stats.byDay') },
  { value: 'hour', label: t('admin.accounts.stats.byHour') }
])
</script>
