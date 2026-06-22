<template>
  <div class="card p-4">
    <div class="mb-3 flex items-center justify-between gap-3">
      <div>
        <h3 class="text-base font-semibold text-gray-900 dark:text-white">
          {{ t('admin.usage.apiKeyRanking.title') }}
        </h3>
        <p class="text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.usage.apiKeyRanking.subtitle') }}
        </p>
      </div>
      <div class="text-right text-xs text-gray-500 dark:text-gray-400">
        <div>${{ formatCost(totalActualCost) }}</div>
        <div>{{ formatTokens(totalTokens) }} tokens</div>
      </div>
    </div>

    <div v-if="loading" class="flex items-center justify-center py-8">
      <LoadingSpinner />
    </div>
    <div v-else-if="items.length === 0" class="py-8 text-center text-sm text-gray-500 dark:text-gray-400">
      {{ t('admin.dashboard.noDataAvailable') }}
    </div>
    <div v-else class="overflow-x-auto">
      <table class="w-full text-sm">
        <thead>
          <tr class="border-b border-gray-200 text-left text-xs font-medium text-gray-500 dark:border-dark-600 dark:text-gray-400">
            <th class="pb-2 pr-3">{{ t('admin.usage.apiKeyRanking.rank') }}</th>
            <th class="pb-2 pr-3">{{ t('usage.apiKeyFilter') }}</th>
            <th class="pb-2 pr-3">{{ t('admin.usage.user') }}</th>
            <th class="pb-2 pr-3 text-right">{{ t('usage.requests') }}</th>
            <th class="pb-2 pr-3 text-right">{{ t('usage.tokens') }}</th>
            <th class="pb-2 text-right">{{ t('usage.cost') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="(item, index) in items"
            :key="item.api_key_id"
            class="border-b border-gray-100 last:border-b-0 hover:bg-gray-50 dark:border-dark-700 dark:hover:bg-dark-700/40"
          >
            <td class="py-2 pr-3 text-xs font-medium text-gray-500 dark:text-gray-400">
              #{{ index + 1 }}
            </td>
            <td class="max-w-[180px] py-2 pr-3">
              <button
                type="button"
                class="truncate text-left font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
                :title="item.key_name || `#${item.api_key_id}`"
                @click="$emit('keyClick', item)"
              >
                {{ item.key_name || `#${item.api_key_id}` }}
              </button>
            </td>
            <td class="max-w-[180px] truncate py-2 pr-3 text-gray-600 dark:text-gray-300" :title="item.email">
              {{ item.email || `User #${item.user_id}` }}
            </td>
            <td class="py-2 pr-3 text-right text-gray-600 dark:text-gray-300">
              {{ item.requests.toLocaleString() }}
            </td>
            <td class="py-2 pr-3 text-right text-gray-600 dark:text-gray-300">
              {{ formatTokens(item.tokens) }}
            </td>
            <td class="py-2 text-right font-medium text-green-600 dark:text-green-400">
              ${{ formatCost(item.actual_cost) }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import type { ApiKeySpendingRankingItem } from '@/types'

const { t } = useI18n()

defineProps<{
  items: ApiKeySpendingRankingItem[]
  loading?: boolean
  totalActualCost?: number
  totalTokens?: number
}>()

defineEmits<{
  keyClick: [item: ApiKeySpendingRankingItem]
}>()

const formatTokens = (value = 0): string => {
  if (value >= 1_000_000_000) return `${(value / 1_000_000_000).toFixed(2)}B`
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(2)}M`
  if (value >= 1_000) return `${(value / 1_000).toFixed(2)}K`
  return value.toLocaleString()
}

const formatCost = (value = 0): string => {
  if (value >= 1000) return `${(value / 1000).toFixed(2)}K`
  if (value >= 1) return value.toFixed(2)
  if (value >= 0.01) return value.toFixed(3)
  return value.toFixed(4)
}
</script>
