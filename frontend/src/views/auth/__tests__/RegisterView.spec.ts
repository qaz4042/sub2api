import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import RegisterView from '@/views/auth/RegisterView.vue'

const {
  pushMock,
  showErrorMock,
  showSuccessMock,
  registerMock,
  getPublicSettingsMock,
  sendVerifyCodeMock,
  validatePromoCodeMock,
  validateInvitationCodeMock,
} = vi.hoisted(() => ({
  pushMock: vi.fn(),
  showErrorMock: vi.fn(),
  showSuccessMock: vi.fn(),
  registerMock: vi.fn(),
  getPublicSettingsMock: vi.fn(),
  sendVerifyCodeMock: vi.fn(),
  validatePromoCodeMock: vi.fn(),
  validateInvitationCodeMock: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: pushMock,
  }),
  useRoute: () => ({
    query: {},
  }),
}))

vi.mock('vue-i18n', () => ({
  createI18n: () => ({
    global: {
      t: (key: string) => key,
    },
  }),
  useI18n: () => ({
    t: (key: string, params?: Record<string, string | number>) => {
      if (key === 'auth.accountCreatedSuccess') {
        return `Account created for ${params?.siteName ?? 'Sub2API'}`
      }
      return key
    },
    locale: { value: 'en' },
  }),
}))

vi.mock('@/stores', () => ({
  useAuthStore: () => ({
    register: (...args: any[]) => registerMock(...args),
  }),
  useAppStore: () => ({
    showError: (...args: any[]) => showErrorMock(...args),
    showSuccess: (...args: any[]) => showSuccessMock(...args),
    showWarning: vi.fn(),
  }),
}))

vi.mock('@/api/auth', () => ({
  getPublicSettings: (...args: any[]) => getPublicSettingsMock(...args),
  isWeChatWebOAuthEnabled: () => false,
  sendVerifyCode: (...args: any[]) => sendVerifyCodeMock(...args),
  validatePromoCode: (...args: any[]) => validatePromoCodeMock(...args),
  validateInvitationCode: (...args: any[]) => validateInvitationCodeMock(...args),
}))

function mountRegisterView() {
  return mount(RegisterView, {
    global: {
      stubs: {
        AuthLayout: { template: '<div><slot /><slot name="footer" /></div>' },
        EmailOAuthButtons: true,
        Icon: true,
        LinuxDoOAuthSection: true,
        LoginAgreementPrompt: true,
        OidcOAuthSection: true,
        RouterLink: { template: '<a><slot /></a>' },
        TurnstileWidget: true,
        WechatOAuthSection: true,
        transition: false,
      },
    },
  })
}

describe('RegisterView', () => {
  beforeEach(() => {
    pushMock.mockReset()
    showErrorMock.mockReset()
    showSuccessMock.mockReset()
    registerMock.mockReset()
    getPublicSettingsMock.mockReset()
    sendVerifyCodeMock.mockReset()
    validatePromoCodeMock.mockReset()
    validateInvitationCodeMock.mockReset()
    sessionStorage.clear()
    localStorage.clear()

    getPublicSettingsMock.mockResolvedValue({
      registration_enabled: true,
      email_verify_enabled: true,
      promo_code_enabled: false,
      invitation_code_enabled: false,
      turnstile_enabled: false,
      turnstile_site_key: '',
      site_name: 'Sub2API',
      linuxdo_oauth_enabled: false,
      oidc_oauth_enabled: false,
      oidc_oauth_provider_name: 'OIDC',
      github_oauth_enabled: false,
      google_oauth_enabled: false,
      registration_email_suffix_whitelist: [],
      login_agreement_enabled: false,
      login_agreement_documents: [],
    })
    sendVerifyCodeMock.mockResolvedValue({ countdown: 60 })
  })

  it('sends the verification code before navigating to the email verification page', async () => {
    const wrapper = mountRegisterView()
    await flushPromises()

    await wrapper.get('#email').setValue('new@example.com')
    await wrapper.get('#password').setValue('secret-123')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(sendVerifyCodeMock).toHaveBeenCalledWith({
      email: 'new@example.com',
      turnstile_token: undefined,
    })
    expect(pushMock).toHaveBeenCalledWith('/email-verify')

    const registerData = JSON.parse(sessionStorage.getItem('register_data') || '{}')
    expect(registerData).toMatchObject({
      email: 'new@example.com',
      password: 'secret-123',
      code_sent: true,
      code_sent_countdown: 60,
    })
  })

  it('stays on the registration page when sending the verification code fails', async () => {
    sendVerifyCodeMock.mockRejectedValue({
      response: {
        data: {
          message: 'email already exists',
        },
      },
    })

    const wrapper = mountRegisterView()
    await flushPromises()

    await wrapper.get('#email').setValue('used@example.com')
    await wrapper.get('#password').setValue('secret-123')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(sendVerifyCodeMock).toHaveBeenCalledWith({
      email: 'used@example.com',
      turnstile_token: undefined,
    })
    expect(pushMock).not.toHaveBeenCalled()
    expect(sessionStorage.getItem('register_data')).toBeNull()
    expect(showErrorMock).toHaveBeenCalledWith('email already exists')
  })
})
