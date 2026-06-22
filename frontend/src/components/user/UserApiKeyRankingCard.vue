<template>
  <section class="card overflow-hidden">
    <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <div class="flex items-center gap-2">
            <span class="flex h-8 w-8 items-center justify-center rounded-lg bg-amber-100 text-lg dark:bg-amber-500/15">🏆</span>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ t('usage.apiKeyRanking.title') }}
            </h2>
          </div>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('usage.apiKeyRanking.subtitle') }}
          </p>
        </div>
        <span class="rounded-full bg-gray-100 px-3 py-1 text-xs font-medium text-gray-500 dark:bg-dark-700 dark:text-gray-300">
          {{ t('usage.apiKeyRanking.participants', { count: totalKeys }) }}
        </span>
      </div>
    </div>

    <div v-if="loading" class="flex min-h-64 items-center justify-center">
      <LoadingSpinner />
    </div>

    <div v-else-if="error" class="flex min-h-64 flex-col items-center justify-center px-6 text-center">
      <span class="flex h-11 w-11 items-center justify-center rounded-full bg-red-50 text-xl dark:bg-red-500/10">!</span>
      <p class="mt-3 text-sm font-semibold text-gray-900 dark:text-white">
        {{ t('usage.apiKeyRanking.loadFailed') }}
      </p>
      <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
        {{ t('usage.apiKeyRanking.loadFailedHint') }}
      </p>
      <button type="button" class="btn btn-secondary mt-4" @click="$emit('retry')">
        {{ t('common.retry') }}
      </button>
    </div>

    <div v-else class="grid lg:grid-cols-[minmax(280px,0.8fr)_minmax(0,1.4fr)]">
      <div class="relative overflow-hidden bg-gradient-to-br from-primary-600 via-primary-500 to-cyan-500 p-5 text-white">
        <div class="pointer-events-none absolute -right-10 -top-12 h-40 w-40 rounded-full bg-white/10 blur-2xl"></div>
        <div class="pointer-events-none absolute -bottom-16 -left-10 h-40 w-40 rounded-full bg-cyan-200/15 blur-2xl"></div>

        <div class="relative">
          <p class="text-xs font-semibold uppercase tracking-[0.16em] text-white/75">
            {{ t('usage.apiKeyRanking.myBestRank') }}
          </p>

          <template v-if="bestRanking">
            <div class="mt-3 flex items-end gap-3">
              <span class="text-5xl font-black tracking-tight">#{{ bestRanking.rank }}</span>
              <span v-if="outrankPercent > 0" class="mb-1 rounded-full bg-white/15 px-2.5 py-1 text-xs font-medium backdrop-blur-sm">
                {{ t('usage.apiKeyRanking.outranked', { percent: outrankPercent }) }}
              </span>
            </div>
            <button
              type="button"
              class="mt-3 max-w-full truncate text-left text-sm font-semibold text-white/95 underline-offset-4 hover:underline"
              @click="emitKeyClick(bestRanking)"
            >
              {{ bestRanking.key_name || t('usage.apiKeyRanking.unnamedKey') }}
            </button>
            <div class="mt-5 grid grid-cols-3 gap-2">
              <div class="rounded-lg bg-black/10 p-2.5 backdrop-blur-sm">
                <p class="text-[10px] uppercase tracking-wide text-white/65">{{ t('usage.requests') }}</p>
                <p class="mt-1 text-sm font-bold">{{ bestRanking.requests.toLocaleString() }}</p>
              </div>
              <div class="rounded-lg bg-black/10 p-2.5 backdrop-blur-sm">
                <p class="text-[10px] uppercase tracking-wide text-white/65">{{ t('usage.tokens') }}</p>
                <p class="mt-1 text-sm font-bold">{{ formatTokens(bestRanking.tokens) }}</p>
              </div>
              <div class="rounded-lg bg-black/10 p-2.5 backdrop-blur-sm">
                <p class="text-[10px] uppercase tracking-wide text-white/65">{{ t('usage.cost') }}</p>
                <p class="mt-1 text-sm font-bold">${{ formatCost(bestRanking.actual_cost) }}</p>
              </div>
            </div>
          </template>

          <div v-else class="flex min-h-40 flex-col justify-center">
            <p class="text-2xl font-bold">{{ t('usage.apiKeyRanking.notRanked') }}</p>
            <p class="mt-2 text-sm leading-6 text-white/75">{{ t('usage.apiKeyRanking.notRankedHint') }}</p>
          </div>

          <div v-if="myRankings.length > 1" class="mt-5 border-t border-white/20 pt-4">
            <p class="mb-2 text-xs font-semibold text-white/75">{{ t('usage.apiKeyRanking.allMyKeys') }}</p>
            <div class="max-h-32 space-y-1.5 overflow-y-auto pr-1">
              <button
                v-for="item in myRankings"
                :key="item.api_key_id"
                type="button"
                class="flex w-full items-center justify-between gap-3 rounded-lg bg-white/10 px-3 py-2 text-left text-xs transition hover:bg-white/20"
                @click="emitKeyClick(item)"
              >
                <span class="truncate font-medium">{{ item.key_name || t('usage.apiKeyRanking.unnamedKey') }}</span>
                <span class="flex-shrink-0 font-bold">#{{ item.rank }}</span>
              </button>
            </div>
          </div>
        </div>
      </div>

      <div class="min-w-0 p-5">
        <div class="mb-3 flex items-center justify-between gap-3">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
            {{ t('usage.apiKeyRanking.topTitle') }}
          </h3>
          <span class="text-xs text-gray-400 dark:text-gray-500">{{ t('usage.apiKeyRanking.bySpend') }}</span>
        </div>

        <div v-if="ranking.length" class="overflow-x-auto">
          <table class="w-full min-w-[520px] text-sm">
            <thead>
              <tr class="border-b border-gray-100 text-xs font-medium text-gray-400 dark:border-dark-700 dark:text-gray-500">
                <th class="pb-2 pr-3 text-left">{{ t('usage.apiKeyRanking.rank') }}</th>
                <th class="pb-2 pr-3 text-left">API Key</th>
                <th class="pb-2 pr-3 text-right">{{ t('usage.requests') }}</th>
                <th class="pb-2 pr-3 text-right">{{ t('usage.tokens') }}</th>
                <th class="pb-2 text-right">{{ t('usage.cost') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="item in ranking"
                :key="`${item.rank}-${item.api_key_id || 'anonymous'}`"
                class="border-b border-gray-50 last:border-0 dark:border-dark-800"
                :class="item.is_mine ? 'bg-primary-50/80 dark:bg-primary-500/10' : ''"
              >
                <td class="py-2.5 pr-3">
                  <span
                    class="inline-flex h-7 min-w-7 items-center justify-center rounded-full px-1.5 text-xs font-bold"
                    :class="rankClass(item.rank)"
                  >
                    {{ item.rank }}
                  </span>
                </td>
                <td class="max-w-[180px] py-2.5 pr-3">
                  <button
                    v-if="item.is_mine"
                    type="button"
                    class="flex max-w-full items-center gap-2 text-left font-semibold text-primary-600 dark:text-primary-400"
                    @click="emitKeyClick(item)"
                  >
                    <span class="truncate">{{ item.key_name || t('usage.apiKeyRanking.unnamedKey') }}</span>
                    <span class="flex-shrink-0 rounded-full bg-primary-100 px-1.5 py-0.5 text-[10px] dark:bg-primary-500/20">
                      {{ t('usage.apiKeyRanking.mine') }}
                    </span>
                  </button>
                  <span v-else class="font-medium text-gray-500 dark:text-gray-400">
                    {{ item.key_name || t('usage.apiKeyRanking.anonymousKey') }}
                  </span>
                </td>
                <td class="py-2.5 pr-3 text-right text-gray-600 dark:text-gray-300">{{ item.requests.toLocaleString() }}</td>
                <td class="py-2.5 pr-3 text-right text-gray-600 dark:text-gray-300">{{ formatTokens(item.tokens) }}</td>
                <td class="py-2.5 text-right font-semibold text-green-600 dark:text-green-400">${{ formatCost(item.actual_cost) }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div v-else class="flex min-h-48 items-center justify-center text-sm text-gray-500 dark:text-gray-400">
          {{ t('usage.apiKeyRanking.noData') }}
        </div>

        <p class="mt-3 text-[11px] leading-5 text-gray-400 dark:text-gray-500">
          {{ t('usage.apiKeyRanking.privacyHint') }}
        </p>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import type { UserApiKeyRankingItem } from '@/types'

const props = withDefaults(defineProps<{
  ranking: UserApiKeyRankingItem[]
  myRankings: UserApiKeyRankingItem[]
  totalKeys?: number
  loading?: boolean
  error?: boolean
}>(), {
  totalKeys: 0,
  loading: false,
  error: false,
})

const emit = defineEmits<{
  keyClick: [item: UserApiKeyRankingItem]
  retry: []
}>()

const { t } = useI18n()

const bestRanking = computed(() => props.myRankings[0] || null)
const outrankPercent = computed(() => {
  if (!bestRanking.value || props.totalKeys <= 1) return 0
  return Math.max(0, Math.round(((props.totalKeys - bestRanking.value.rank) / (props.totalKeys - 1)) * 100))
})

const emitKeyClick = (item: UserApiKeyRankingItem) => {
  if (item.api_key_id) emit('keyClick', item)
}

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

const rankClass = (rank: number): string => {
  if (rank === 1) return 'bg-amber-100 text-amber-700 dark:bg-amber-500/20 dark:text-amber-300'
  if (rank === 2) return 'bg-slate-100 text-slate-600 dark:bg-slate-500/20 dark:text-slate-300'
  if (rank === 3) return 'bg-orange-100 text-orange-700 dark:bg-orange-500/20 dark:text-orange-300'
  return 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-300'
}
</script>
