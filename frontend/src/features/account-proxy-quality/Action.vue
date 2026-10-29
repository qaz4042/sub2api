<template>
  <button v-if="account.platform === 'openai' && account.parent_account_id == null" class="flex w-full items-center gap-2 px-4 py-2 text-sm hover:bg-gray-100 dark:hover:bg-dark-700" @click="$emit('open', account)">
    <Icon name="sync" size="sm" class="text-indigo-500" />{{ text.title }}
  </button>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Icon } from '@/components/icons'
import type { Account } from '@/types'
import { copy } from './text'
defineProps<{ account: Account }>()
defineEmits<{ open: [account: Account] }>()
const { locale } = useI18n()
const text = computed(() => locale.value.startsWith('zh') ? copy.zh : copy.en)
</script>
