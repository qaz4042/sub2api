import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: vi.fn().mockResolvedValue(true)
  })
}))

import UseKeyModal from '../UseKeyModal.vue'
import Select from '@/components/common/Select.vue'

describe('UseKeyModal', () => {
  it('renders GPT-5.5 and goals feature in OpenAI Codex config', () => {
    const wrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'sk-test',
        baseUrl: 'https://example.com/v1',
        platform: 'openai'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          Icon: {
            template: '<span />'
          }
        }
      }
    })

    const codeBlocks = wrapper.findAll('pre code').map((code) => code.text())
    const configToml = codeBlocks.find((content) => content.includes('model_provider = "OpenAI"'))

    expect(configToml).toBeDefined()
    expect(configToml).toContain('model = "gpt-5.5"')
    expect(configToml).toContain('review_model = "gpt-5.5"')
    expect(configToml).not.toContain('model = "gpt-5.4"')
    expect(configToml).not.toContain('model_context_window')
    expect(configToml).not.toContain('model_auto_compact_token_limit')
    expect(configToml).toContain('[features]\ngoals = true')
  })

  it('allows switching between available API keys', async () => {
    const wrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'sk-first-key',
        apiKeys: [
          { id: 1, name: 'First key', key: 'sk-first-key', status: 'active', platform: 'openai', groupName: 'OpenAI' },
          { id: 2, name: 'Second key', key: 'sk-second-key', status: 'active', platform: 'openai', groupName: 'OpenAI' },
        ],
        selectedKeyId: 1,
        baseUrl: 'https://example.com/v1',
        platform: 'openai'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          Icon: {
            template: '<span />'
          }
        }
      }
    })

    const selector = wrapper.findComponent(Select)
    expect(selector.exists()).toBe(true)
    expect(selector.props('options')).toHaveLength(2)

    await selector.vm.$emit('update:modelValue', 2)

    expect(wrapper.emitted('update:selectedKeyId')).toEqual([[2]])
  })

  it('renders a static OpenAI curl example without making a request', async () => {
    const wrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'sk-test',
        baseUrl: 'https://example.com/v1',
        platform: 'openai'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          Icon: {
            template: '<span />'
          }
        }
      }
    })

    const apiExampleTab = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.cliTabs.apiExample')
    )

    expect(apiExampleTab).toBeDefined()
    await apiExampleTab!.trigger('click')
    await nextTick()

    const content = wrapper.find('pre code').text()
    expect(content).toContain('https://example.com/v1/chat/completions')
    expect(content).toContain('Authorization: Bearer sk-test')
    expect(content).toContain('"model": "gpt-5.5"')
    expect(wrapper.text()).toContain('keys.useKeyModal.apiExample.copyHint')
  })

  it('uses native authentication and paths for Gemini and Antigravity examples', async () => {
    const geminiWrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'gemini-key',
        baseUrl: 'https://example.com/v1beta',
        platform: 'gemini'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          Icon: {
            template: '<span />'
          }
        }
      }
    })

    const geminiTab = geminiWrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.cliTabs.apiExample')
    )
    await geminiTab!.trigger('click')
    await nextTick()
    expect(geminiWrapper.text()).toContain('https://example.com/v1beta/models/gemini-2.0-flash:generateContent')
    expect(geminiWrapper.text()).toContain('x-goog-api-key: gemini-key')

    const antigravityWrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'antigravity-key',
        baseUrl: 'https://example.com/v1',
        platform: 'antigravity'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          Icon: {
            template: '<span />'
          }
        }
      }
    })

    const antigravityTab = antigravityWrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.cliTabs.apiExample')
    )
    await antigravityTab!.trigger('click')
    await nextTick()
    const antigravityContent = antigravityWrapper.findAll('pre code').map((code) => code.text()).join('\n')
    expect(antigravityContent).toContain('https://example.com/antigravity/v1/messages')
    expect(antigravityContent).toContain('https://example.com/antigravity/v1beta/models/gemini-2.0-flash:generateContent')
    expect(antigravityContent).toContain('x-api-key: antigravity-key')
    expect(antigravityContent).toContain('x-goog-api-key: antigravity-key')
  })

  it('runs a minimal OpenAI request and renders the successful response', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      statusText: 'OK',
      text: async () => '{"choices":[{"message":{"content":"hello"}}]}'
    })
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'sk-test',
        baseUrl: 'https://example.com/v1',
        platform: 'openai'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          Icon: {
            template: '<span />'
          }
        }
      }
    })

    const apiExampleTab = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.cliTabs.apiExample')
    )
    await apiExampleTab!.trigger('click')
    await nextTick()

    const startButton = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.apiExample.quickTestStart')
    )
    await startButton!.trigger('click')
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith(
      'https://example.com/v1/chat/completions',
      expect.objectContaining({
        method: 'POST',
        headers: {
          Authorization: 'Bearer sk-test',
          'Content-Type': 'application/json'
        }
      })
    )
    expect(wrapper.text()).toContain('hello')
    expect(wrapper.text()).toContain('keys.useKeyModal.apiExample.quickTestStatusSuccess')
    vi.unstubAllGlobals()
  })

  it('uses the selected preset model for each quick test platform', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      statusText: 'OK',
      text: async () => '{}'
    })
    vi.stubGlobal('fetch', fetchMock)

    const openaiWrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'sk-test',
        baseUrl: 'https://example.com/v1',
        platform: 'openai'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          Icon: {
            template: '<span />'
          }
        }
      }
    })

    const openaiTab = openaiWrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.cliTabs.apiExample')
    )
    await openaiTab!.trigger('click')
    await nextTick()

    const modelSelect = openaiWrapper.findComponent(Select)
    expect(modelSelect.props('options').map((option: { value: string }) => option.value)).toEqual([
      'gpt-5.5',
      'gpt-5.4',
      'gpt-5.4-mini'
    ])
    await modelSelect.vm.$emit('update:modelValue', 'gpt-5.4-mini')
    expect(modelSelect.props('modelValue')).toBe('gpt-5.4-mini')
    const openaiStartButton = openaiWrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.apiExample.quickTestStart')
    )
    await openaiStartButton!.trigger('click')
    await flushPromises()
    expect(JSON.parse(fetchMock.mock.calls[0][1].body as string).model).toBe('gpt-5.4-mini')
    await openaiWrapper.setProps({ platform: 'gemini', baseUrl: 'https://example.com/v1beta', apiKey: 'gemini-key' })
    await nextTick()
    const switchedGeminiTab = openaiWrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.cliTabs.apiExample')
    )
    await switchedGeminiTab!.trigger('click')
    await nextTick()
    expect(openaiWrapper.findComponent(Select).props('modelValue')).toBe('gemini-2.0-flash')
    await openaiWrapper.setProps({ platform: 'openai', baseUrl: 'https://example.com/v1', apiKey: 'sk-test' })
    await nextTick()
    const switchedOpenaiTab = openaiWrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.cliTabs.apiExample')
    )
    await switchedOpenaiTab!.trigger('click')
    await nextTick()
    expect(openaiWrapper.findComponent(Select).props('modelValue')).toBe('gpt-5.5')

    const geminiWrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'gemini-key',
        baseUrl: 'https://example.com/v1beta',
        platform: 'gemini'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          Icon: {
            template: '<span />'
          }
        }
      }
    })
    const geminiTab = geminiWrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.cliTabs.apiExample')
    )
    await geminiTab!.trigger('click')
    await nextTick()
    const geminiModelSelect = geminiWrapper.findComponent(Select)
    expect(geminiModelSelect.props('options').map((option: { value: string }) => option.value)).toEqual([
      'gemini-2.0-flash',
      'gemini-2.5-flash',
      'gemini-2.5-pro'
    ])
    await geminiModelSelect.vm.$emit('update:modelValue', 'gemini-2.5-pro')
    const geminiStartButton = geminiWrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.apiExample.quickTestStart')
    )
    await geminiStartButton!.trigger('click')
    await flushPromises()
    expect(fetchMock.mock.calls[1][0]).toBe('https://example.com/v1beta/models/gemini-2.5-pro:generateContent')

    const anthropicWrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'claude-key',
        baseUrl: 'https://example.com/v1',
        platform: 'anthropic'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          Icon: {
            template: '<span />'
          }
        }
      }
    })
    const anthropicTab = anthropicWrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.cliTabs.apiExample')
    )
    await anthropicTab!.trigger('click')
    await nextTick()
    const anthropicModelSelect = anthropicWrapper.findComponent(Select)
    expect(anthropicModelSelect.props('options').map((option: { value: string }) => option.value)).toEqual([
      'claude-sonnet-4-6',
      'claude-fable-5'
    ])
    await anthropicModelSelect.vm.$emit('update:modelValue', 'claude-fable-5')
    const anthropicStartButton = anthropicWrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.apiExample.quickTestStart')
    )
    await anthropicStartButton!.trigger('click')
    await flushPromises()
    expect(JSON.parse(fetchMock.mock.calls[2][1].body as string).model).toBe('claude-fable-5')

    vi.unstubAllGlobals()
  })

  it('shows a concise error when the API test fails', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: false,
      status: 401,
      statusText: 'Unauthorized',
      text: async () => '{"error":{"message":"invalid key"}}'
    })
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'sk-invalid',
        baseUrl: 'https://example.com/v1',
        platform: 'openai'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          Icon: {
            template: '<span />'
          }
        }
      }
    })

    const apiExampleTab = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.cliTabs.apiExample')
    )
    await apiExampleTab!.trigger('click')
    await nextTick()
    const startButton = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.apiExample.quickTestStart')
    )
    await startButton!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('invalid key')
    expect(wrapper.text()).toContain('keys.useKeyModal.apiExample.quickTestStatusError')
    vi.unstubAllGlobals()
  })

  it('renders GPT-5.5 and goals feature in OpenAI Codex WebSocket config', async () => {
    const wrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'sk-test',
        baseUrl: 'https://example.com/v1',
        platform: 'openai'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          Icon: {
            template: '<span />'
          }
        }
      }
    })

    const wsTab = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.cliTabs.codexCliWs')
    )

    expect(wsTab).toBeDefined()
    await wsTab!.trigger('click')
    await nextTick()

    const codeBlocks = wrapper.findAll('pre code').map((code) => code.text())
    const configToml = codeBlocks.find((content) => content.includes('supports_websockets = true'))

    expect(configToml).toBeDefined()
    expect(configToml).toContain('model = "gpt-5.5"')
    expect(configToml).toContain('review_model = "gpt-5.5"')
    expect(configToml).not.toContain('model = "gpt-5.4"')
    expect(configToml).not.toContain('model_context_window')
    expect(configToml).not.toContain('model_auto_compact_token_limit')
    expect(configToml).toContain('[features]\nresponses_websockets_v2 = true\ngoals = true')
  })

  it('renders GPT-5.4 mini entry in OpenCode config', async () => {
    const wrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'sk-test',
        baseUrl: 'https://example.com/v1',
        platform: 'openai'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          Icon: {
            template: '<span />'
          }
        }
      }
    })

    const opencodeTab = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.cliTabs.opencode')
    )

    expect(opencodeTab).toBeDefined()
    await opencodeTab!.trigger('click')
    await nextTick()

    const codeBlock = wrapper.find('pre code')
    expect(codeBlock.exists()).toBe(true)
    expect(codeBlock.text()).toContain('"name": "GPT-5.4 Mini"')
    expect(codeBlock.text()).not.toContain('"name": "GPT-5.4 Nano"')
  })

  it('renders GPT-5.6 alias and max variants in OpenCode config', async () => {
    const wrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'sk-test',
        baseUrl: 'https://example.com/v1',
        platform: 'openai'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          Icon: {
            template: '<span />'
          }
        }
      }
    })

    const opencodeTab = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.cliTabs.opencode')
    )
    expect(opencodeTab).toBeDefined()
    await opencodeTab!.trigger('click')
    await nextTick()

    const parsed = JSON.parse(wrapper.find('pre code').text())
    const models = parsed.provider.openai.models
    for (const model of ['gpt-5.6', 'gpt-5.6-sol', 'gpt-5.6-terra', 'gpt-5.6-luna']) {
      expect(models[model]).toBeDefined()
      expect(models[model].variants).toHaveProperty('max')
      expect(models[model].variants).toHaveProperty('xhigh')
    }
    expect(models['gpt-5.6'].name).toBe('GPT-5.6 (Sol)')
  })

  it('renders Claude Fable 5 OpenCode config with adaptive thinking', async () => {
    const wrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'sk-test',
        baseUrl: 'https://example.com/v1',
        platform: 'antigravity'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          Icon: {
            template: '<span />'
          }
        }
      }
    })

    const opencodeTab = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.cliTabs.opencode')
    )

    expect(opencodeTab).toBeDefined()
    await opencodeTab!.trigger('click')
    await nextTick()

    const claudeConfig = wrapper.findAll('pre code')
      .map((code) => code.text())
      .find((content) => content.includes('"antigravity-claude"'))

    expect(claudeConfig).toBeDefined()
    const parsed = JSON.parse(claudeConfig!)
    const fable = parsed.provider['antigravity-claude'].models['claude-fable-5']

    expect(fable.name).toBe('Claude Fable 5')
    expect(fable.limit).toEqual({ context: 1048576, output: 128000 })
    expect(fable.options.thinking).toEqual({ type: 'adaptive' })
    expect(fable.options.thinking).not.toHaveProperty('budgetTokens')
  })
})
