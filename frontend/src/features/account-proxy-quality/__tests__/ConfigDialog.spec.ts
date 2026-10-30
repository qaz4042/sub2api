import { mount, flushPromises } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { describe, expect, it, vi } from 'vitest'
import ConfigDialog from '../ConfigDialog.vue'
import Action from '../Action.vue'
import { loadSettings, saveSettings, type Settings } from '../api'
import type { Account } from '@/types'

vi.mock('../api', () => ({ loadSettings: vi.fn(), saveSettings: vi.fn() }))
const makeAccount = (overrides: Partial<Account> = {}) => ({ id: 1, name: '账号 A', platform: 'openai', parent_account_id: null, ...overrides }) as Account
const fixture: Settings = { enabled: false, all_eligible: false, candidate_count: 0, proxy_ids: [], version: 0, current_proxy_id: 1, reason: 'disabled', last_evaluated_at: null, last_switch_at: null, choices: [{ id: 1, name: '代理 A' }, { id: 2, name: '代理 B' }], events: [], samples: [] }
const global = () => ({ plugins: [createI18n({ legacy: false, locale: 'zh' })], stubs: { BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' } } })

describe('proxy quality dialog', () => {
  it('shows when consecutive 5xx pauses optimization', async () => {
    vi.mocked(loadSettings).mockResolvedValue({ ...fixture, enabled: true, reason: 'business_5xx_waiting' })
    const wrapper = mount(ConfigDialog, { props: { account: makeAccount() }, global: global() })
    await flushPromises()
    expect(wrapper.text()).toContain('连续 5xx，等待候选业务成功证据')
    wrapper.unmount()
  })
  it('requires a bounded pool containing the current proxy before enabling', async () => {
    vi.mocked(loadSettings).mockResolvedValue(fixture)
    const wrapper = mount(ConfigDialog, { props: { account: makeAccount() }, global: global() })
    await flushPromises()
    const checkboxes = wrapper.findAll('input[type="checkbox"]')
    await checkboxes[0]!.setValue(true)
    expect(wrapper.text()).toContain('请选择 2–10 条代理并包含当前代理')
    const save = wrapper.findAll('button').find(b => b.text() === '保存配置')!
    expect(save.attributes('disabled')).toBeDefined()
    await checkboxes[1]!.setValue(true); await checkboxes[2]!.setValue(true)
    expect(save.attributes('disabled')).toBeUndefined()
    vi.mocked(saveSettings).mockResolvedValue()
    vi.mocked(loadSettings).mockResolvedValue({ ...fixture, enabled: true, proxy_ids: [1, 2], version: 1 })
    await save.trigger('click'); await flushPromises()
    expect(wrapper.emitted('updated')).toHaveLength(1)
    expect(wrapper.text()).toContain('配置已保存')
    wrapper.unmount()
  })
  it('can disable with unavailable pool entries and shows recent switch history', async () => {
    vi.mocked(loadSettings).mockResolvedValue({ ...fixture, enabled: true, proxy_ids: [1, 9], reason: 'no_candidate', events: [{ from_proxy_id: 9, to_proxy_id: 1, reason: 'degraded', created_at: '2026-10-29T10:00:00Z' }] })
    const wrapper = mount(ConfigDialog, { props: { account: makeAccount() }, global: global() })
    await flushPromises()
    expect(wrapper.text()).toContain('当前不可选')
    expect(wrapper.text()).toContain('当前质量下降，已切换')
    await wrapper.find('input[type="checkbox"]').setValue(false)
    expect(wrapper.findAll('button').find(b => b.text() === '保存配置')!.attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })
  it('shows saved candidate sample counts by model and ingress', async () => {
    vi.mocked(loadSettings).mockResolvedValue({ ...fixture, enabled: true, proxy_ids: [1, 2], samples: [{ proxy_id: 2, model: 'gpt-test', transport: 'http', total: 7, successes: 6, first_tokens: 4, http_5xx: 2 }] })
    const wrapper = mount(ConfigDialog, { props: { account: makeAccount() }, global: global() })
    await flushPromises()
    expect(wrapper.text()).toContain('近 1 小时暂无样本')
    expect(wrapper.text()).toContain('gpt-test · http · 成功 6/7 · 首字 4 · 5xx 2')
    wrapper.unmount()
  })
  it('restricts the action to independent OpenAI accounts', async () => {
    const wrapper = mount(Action, { props: { account: makeAccount() }, global: global() })
    expect(wrapper.find('button').exists()).toBe(true)
    await wrapper.setProps({ account: makeAccount({ parent_account_id: 2 }) })
    expect(wrapper.find('button').exists()).toBe(false)
    await wrapper.setProps({ account: makeAccount({ platform: 'anthropic' }) })
    expect(wrapper.find('button').exists()).toBe(false)
    wrapper.unmount()
  })
})

it('enables dynamic scope without selecting a pool and sends the scope explicitly', async () => {
  vi.mocked(loadSettings).mockResolvedValue({ ...fixture, all_eligible: true })
  vi.mocked(saveSettings).mockResolvedValue()
  const wrapper = mount(ConfigDialog, { props: { account: makeAccount() }, global: global() })
  await flushPromises()
  expect(wrapper.text()).toContain('新增符合条件的代理自动参与')
  expect(wrapper.text()).toContain('监测合格备选：0 条')
  expect(wrapper.find('input[type="checkbox"]').exists()).toBe(true)
  expect(wrapper.findAll('input[type="checkbox"]')).toHaveLength(1)
  await wrapper.find('input[type="checkbox"]').setValue(true)
  const save = wrapper.findAll('button').find(b => b.text() === '保存配置')!
  expect(save.attributes('disabled')).toBeUndefined()
  await save.trigger('click'); await flushPromises()
  expect(saveSettings).toHaveBeenLastCalledWith(1, { enabled: true, all_eligible: true, proxy_ids: [], version: 0 }, expect.any(AbortSignal))
  wrapper.unmount()
})
