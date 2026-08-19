import { describe, expect, it, vi } from 'vitest'
import { getAdminSteps, getUserSteps } from '../steps'

describe('onboarding steps branding', () => {
  it('passes the configured site name to admin and user welcome copy', () => {
    const translate = vi.fn((key: string, params?: Record<string, string>) =>
      `${key}:${params?.siteName || ''}`
    )

    const adminWelcome = getAdminSteps(translate, false, 'Example Site')[0].popover
    const userWelcome = getUserSteps(translate, 'Example Site')[0].popover

    expect(adminWelcome?.title).toBe('onboarding.admin.welcome.title:Example Site')
    expect(adminWelcome?.description).toBe('onboarding.admin.welcome.description:Example Site')
    expect(userWelcome?.title).toBe('onboarding.user.welcome.title:Example Site')
    expect(userWelcome?.description).toBe('onboarding.user.welcome.description:Example Site')
  })
})
