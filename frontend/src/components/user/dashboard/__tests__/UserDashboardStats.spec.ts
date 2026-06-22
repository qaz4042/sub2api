import { describe, expect, it, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { createPinia, setActivePinia } from 'pinia'

import UserDashboardStats from '../UserDashboardStats.vue'
import { useAppStore } from '@/stores'
import type { UserDashboardStats as UserStatsType } from '@/api/usage'
import type { PlatformQuotaItem, PublicSettings } from '@/types'

const messages = {
  dashboard: {
    balance: 'Balance',
    apiKeys: 'API Keys',
    todayRequests: 'Today Requests',
    todayCost: 'Today Cost',
    actual: 'Actual',
    standard: 'Standard',
    todayTokens: 'Today Tokens',
    totalTokens: 'Total Tokens',
    input: 'Input',
    output: 'Output',
    performance: 'Performance',
    avgResponse: 'Avg Response',
    averageTime: 'Average Time',
    platformBreakdown: 'Platform Breakdown',
    platformCount: '{count} platforms',
    platformOther: 'Other',
    requests: 'Requests',
    tokens: 'Tokens',
    platformQuota: {
      title: 'Quota',
      daily: 'Daily',
      weekly: 'Weekly',
      monthly: 'Monthly',
      disabled: 'Disabled',
      resetsAt: 'Resets at {time}',
    },
  },
  common: {
    active: 'active',
    available: 'available',
    total: 'Total',
  },
}

const baseSettings: PublicSettings = {
  registration_enabled: false,
  email_verify_enabled: false,
  force_email_on_third_party_signup: false,
  registration_email_suffix_whitelist: [],
  promo_code_enabled: false,
  password_reset_enabled: false,
  invitation_code_enabled: false,
  turnstile_enabled: false,
  turnstile_site_key: '',
  site_name: 'Sub2API',
  site_logo: '',
  site_subtitle: '',
  api_base_url: '',
  contact_info: '',
  doc_url: '',
  home_content: '',
  hide_ccs_import_button: false,
  payment_enabled: false,
  risk_control_enabled: false,
  table_default_page_size: 20,
  table_page_size_options: [10, 20, 50, 100],
  custom_menu_items: [],
  custom_endpoints: [],
  linuxdo_oauth_enabled: false,
  wechat_oauth_enabled: false,
  oidc_oauth_enabled: false,
  oidc_oauth_provider_name: 'OIDC',
  github_oauth_enabled: false,
  google_oauth_enabled: false,
  backend_mode_enabled: false,
  version: '',
  balance_low_notify_enabled: false,
  account_quota_notify_enabled: false,
  balance_low_notify_threshold: 0,
  channel_monitor_enabled: true,
  channel_monitor_default_interval_seconds: 60,
  available_channels_enabled: false,
  platform_anthropic_enabled: false,
  platform_gemini_enabled: false,
  platform_antigravity_enabled: false,
  service_quota_enabled: false,
  affiliate_enabled: false,
}

const stats: UserStatsType = {
  total_api_keys: 1,
  active_api_keys: 1,
  total_requests: 30,
  total_input_tokens: 300,
  total_output_tokens: 300,
  total_cache_creation_tokens: 0,
  total_cache_read_tokens: 0,
  total_tokens: 600,
  total_cost: 3,
  total_actual_cost: 3,
  today_requests: 30,
  today_input_tokens: 300,
  today_output_tokens: 300,
  today_cache_creation_tokens: 0,
  today_cache_read_tokens: 0,
  today_tokens: 600,
  today_cost: 3,
  today_actual_cost: 3,
  average_duration_ms: 120,
  rpm: 1,
  tpm: 20,
  by_platform: [
    {
      platform: 'openai',
      total_requests: 10,
      total_tokens: 200,
      total_actual_cost: 1,
      today_requests: 10,
      today_tokens: 200,
      today_actual_cost: 1,
    },
    {
      platform: 'anthropic',
      total_requests: 10,
      total_tokens: 200,
      total_actual_cost: 1,
      today_requests: 10,
      today_tokens: 200,
      today_actual_cost: 1,
    },
    {
      platform: 'gemini',
      total_requests: 10,
      total_tokens: 200,
      total_actual_cost: 1,
      today_requests: 10,
      today_tokens: 200,
      today_actual_cost: 1,
    },
  ],
}

const platformQuotas: PlatformQuotaItem[] = [
  { platform: 'openai', daily_limit_usd: 10, daily_usage_usd: 1 },
  { platform: 'anthropic', daily_limit_usd: 10, daily_usage_usd: 1 },
  { platform: 'gemini', daily_limit_usd: 10, daily_usage_usd: 1 },
] as PlatformQuotaItem[]

function mountStats(settings: Partial<PublicSettings> = {}) {
  const pinia = createPinia()
  setActivePinia(pinia)
  const appStore = useAppStore()
  appStore.cachedPublicSettings = { ...baseSettings, ...settings }

  const i18n = createI18n({
    legacy: false,
    locale: 'en',
    messages: { en: messages },
  })

  return mount(UserDashboardStats, {
    props: {
      stats,
      balance: 0,
      isSimple: false,
      platformQuotas,
    },
    global: {
      plugins: [pinia, i18n],
      stubs: {
        Icon: true,
      },
    },
  })
}

describe('UserDashboardStats 平台拆分', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('默认只展示 OpenAI，不展示已关闭平台或把它们归入其他', () => {
    const wrapper = mountStats()

    expect(wrapper.text()).toContain('OpenAI')
    expect(wrapper.text()).not.toContain('Claude')
    expect(wrapper.text()).not.toContain('Gemini')
    expect(wrapper.text()).not.toContain('Other')
    expect(wrapper.find('[data-platform-layout="single"]').exists()).toBe(true)
  })

  it('平台开关打开后展示对应平台', () => {
    const wrapper = mountStats({ platform_gemini_enabled: true })

    expect(wrapper.text()).toContain('OpenAI')
    expect(wrapper.text()).toContain('Gemini')
    expect(wrapper.text()).not.toContain('Claude')
    expect(wrapper.find('[data-platform-layout="grid"]').exists()).toBe(true)
  })
})
