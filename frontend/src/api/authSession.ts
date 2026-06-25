let accessToken: string | null = null
let tokenExpiresAt: number | null = null
let legacyRefreshToken: string | null = null

const LEGACY_AUTH_TOKEN_KEY = 'auth_token'
const LEGACY_REFRESH_TOKEN_KEY = 'refresh_token'
const LEGACY_TOKEN_EXPIRES_AT_KEY = 'token_expires_at'

export function setAccessToken(token: string | null): void {
  accessToken = token && token.trim() ? token : null
}

export function getAccessToken(): string | null {
  return accessToken
}

export function setAccessTokenExpiresIn(expiresIn: number | undefined): number | null {
  if (!expiresIn) {
    tokenExpiresAt = null
    return null
  }
  tokenExpiresAt = Date.now() + expiresIn * 1000
  return tokenExpiresAt
}

export function setAccessTokenExpiresAt(expiresAt: number | null): void {
  tokenExpiresAt = expiresAt
}

export function getAccessTokenExpiresAt(): number | null {
  return tokenExpiresAt
}

export function getLegacyRefreshToken(): string | null {
  return legacyRefreshToken || localStorage.getItem(LEGACY_REFRESH_TOKEN_KEY)
}

export function setLegacyRefreshToken(token: string | null): void {
  legacyRefreshToken = token && token.trim() ? token : null
}

export function getLegacyTokenExpiresAt(): number | null {
  const value = localStorage.getItem(LEGACY_TOKEN_EXPIRES_AT_KEY)
  return value ? parseInt(value, 10) : null
}

export function clearLegacyTokenStorage(): void {
  legacyRefreshToken = null
  localStorage.removeItem(LEGACY_AUTH_TOKEN_KEY)
  localStorage.removeItem(LEGACY_REFRESH_TOKEN_KEY)
  localStorage.removeItem(LEGACY_TOKEN_EXPIRES_AT_KEY)
}

export function clearAuthSession(): void {
  accessToken = null
  tokenExpiresAt = null
  clearLegacyTokenStorage()
}
