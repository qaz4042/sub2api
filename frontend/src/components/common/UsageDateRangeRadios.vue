<template>
  <fieldset class="min-w-0">
    <legend class="input-label">{{ label }}</legend>
    <div class="flex flex-wrap gap-2" role="radiogroup" :aria-label="label">
      <label
        v-for="option in usageDateRangePresetOptions"
        :key="option.value"
        :class="[
          'inline-flex cursor-pointer items-center rounded-md border px-3 py-2 text-sm font-medium transition-colors',
          modelValue === option.value
            ? 'border-primary-500 bg-primary-50 text-primary-700 dark:border-primary-500 dark:bg-primary-900/30 dark:text-primary-200'
            : 'border-gray-200 bg-white text-gray-700 hover:border-gray-300 hover:bg-gray-50 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300 dark:hover:border-dark-500 dark:hover:bg-dark-700',
        ]"
      >
        <input
          class="sr-only"
          type="radio"
          :name="radioGroupName"
          :value="option.value"
          :checked="modelValue === option.value"
          @change="selectPreset(option.value)"
        />
        {{ t(option.labelKey) }}
      </label>
    </div>
    <div v-if="modelValue === 'custom'" class="mt-3 flex flex-wrap items-end gap-3">
      <div class="min-w-[150px]">
        <label class="input-label">{{ t('dates.startDate') }}</label>
        <input
          v-model="localStartDate"
          type="date"
          class="input"
          :max="localEndDate || tomorrow"
        />
      </div>
      <div class="min-w-[150px]">
        <label class="input-label">{{ t('dates.endDate') }}</label>
        <input
          v-model="localEndDate"
          type="date"
          class="input"
          :min="localStartDate"
          :max="tomorrow"
        />
      </div>
      <button
        type="button"
        class="btn btn-secondary"
        :disabled="!canApplyCustom"
        @click="applyCustom"
      >
        {{ t('dates.apply') }}
      </button>
    </div>
  </fieldset>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  formatLocalDate,
  getUsageDateRangePreset,
  usageDateRangePresetOptions,
  type UsageDateRangeMode,
} from '@/utils/usageDateRange'

interface Props {
  modelValue: UsageDateRangeMode | null
  label: string
  startDate: string
  endDate: string
}

interface Emits {
  (event: 'update:modelValue', value: UsageDateRangeMode): void
  (event: 'update:startDate', value: string): void
  (event: 'update:endDate', value: string): void
  (event: 'change', range: { startDate: string; endDate: string; preset: UsageDateRangeMode }): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()
const { t } = useI18n()
const radioGroupName = `usage-date-range-${Math.random().toString(36).slice(2)}`
const localStartDate = ref(props.startDate)
const localEndDate = ref(props.endDate)
const tomorrow = computed(() => {
  const date = new Date()
  date.setDate(date.getDate() + 1)
  return formatLocalDate(date)
})
const canApplyCustom = computed(() =>
  Boolean(localStartDate.value && localEndDate.value && localStartDate.value <= localEndDate.value),
)

const selectPreset = (preset: UsageDateRangeMode) => {
  if (preset === 'custom') {
    localStartDate.value = props.startDate
    localEndDate.value = props.endDate
    emit('update:modelValue', preset)
    return
  }
  const range = getUsageDateRangePreset(preset)
  emit('update:modelValue', preset)
  emit('update:startDate', range.start)
  emit('update:endDate', range.end)
  emit('change', {
    startDate: range.start,
    endDate: range.end,
    preset,
  })
}

const applyCustom = () => {
  if (!canApplyCustom.value) return
  emit('update:modelValue', 'custom')
  emit('update:startDate', localStartDate.value)
  emit('update:endDate', localEndDate.value)
  emit('change', {
    startDate: localStartDate.value,
    endDate: localEndDate.value,
    preset: 'custom',
  })
}

watch(
  () => props.startDate,
  (value) => {
    localStartDate.value = value
  },
)

watch(
  () => props.endDate,
  (value) => {
    localEndDate.value = value
  },
)
</script>
