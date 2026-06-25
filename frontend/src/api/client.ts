/**
 * Axios HTTP Client Configuration
 * Base client with interceptors for authentication, token refresh, and error handling
 */

import axios, { AxiosInstance, AxiosError, InternalAxiosRequestConfig, AxiosResponse } from 'axios'
import type { ApiResponse } from '@/types'
import { getLocale } from '@/i18n'
import {
  clearAuthSession,
  getAccessToken,
  getLegacyRefreshToken,
  setAccessToken,
  setAccessTokenExpiresIn
} from './authSession'

// ==================== Axios Instance Configuration ====================

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '/api/v1'

export const apiClient: AxiosInstance = axios.create({
  baseURL: API_BASE_URL,
  withCredentials: true,
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json'
  }
})

// ==================== Token Refresh State ====================

// Track if a token refresh is in progress to prevent multiple simultaneous refresh requests
let isRefreshing = false
// Queue of requests waiting for token refresh
let refreshSubscribers: Array<(token: string) => void> = []

/**
 * Subscribe to token refresh completion
 */
function subscribeTokenRefresh(callback: (token: string) => void): void {
  refreshSubscribers.push(callback)
}

/**
 * Notify all subscribers that token has been refreshed
 */
function onTokenRefreshed(token: string): void {
  refreshSubscribers.forEach((callback) => callback(token))
  refreshSubscribers = []
}

// ==================== Request Interceptor ====================

// Get user's timezone
const getUserTimezone = (): string => {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone
  } catch {
    return 'UTC'
  }
}

apiClient.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    // Attach access token from memory only.
    const token = getAccessToken()
    if (token && config.headers) {
      config.headers.Authorization = `Bearer ${token}`
    }

    // Attach locale for backend translations
    if (config.headers) {
      config.headers['Accept-Language'] = getLocale()
    }

    // Attach timezone for all GET requests (backend may use it for default date ranges)
    if (config.method === 'get') {
      if (!config.params) {
        config.params = {}
      }
      config.params.timezone = getUserTimezone()
    }

    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// ==================== Response Interceptor ====================

apiClient.interceptors.response.use(
  (response: AxiosResponse) => {
    // Unwrap standard API response format { code, message, data }
    const apiResponse = response.data as ApiResponse<unknown>
    if (apiResponse && typeof apiResponse === 'object' && 'code' in apiResponse) {
      if (apiResponse.code === 0) {
        // Success - return the data portion
        response.data = apiResponse.data
      } else {
        // API error
        const resp = apiResponse as unknown as Record<string, unknown>
        return Promise.reject({
          status: response.status,
          code: apiResponse.code,
          message: apiResponse.message || 'Unknown error',
          reason: resp.reason,
          metadata: resp.metadata,
        })
      }
    }
    return response
  },
  async (error: AxiosError<ApiResponse<unknown>>) => {
    // Request cancellation: keep the original axios cancellation error so callers can ignore it.
    // Otherwise we'd misclassify it as a generic "network error".
    if (error.code === 'ERR_CANCELED' || axios.isCancel(error)) {
      return Promise.reject(error)
    }

    const originalRequest = error.config as InternalAxiosRequestConfig & { _retry?: boolean }

    // Handle common errors
    if (error.response) {
      const { status, data } = error.response
      const url = String(error.config?.url || '')

      // Validate `data` shape to avoid HTML error pages breaking our error handling.
      const apiData = (typeof data === 'object' && data !== null ? data : {}) as Record<string, any>

      // Ops monitoring disabled: treat as feature-flagged 404, and proactively redirect away
      // from ops pages to avoid broken UI states.
      if (status === 404 && apiData.message === 'Ops monitoring is disabled') {
        try {
          localStorage.setItem('ops_monitoring_enabled_cached', 'false')
        } catch {
          // ignore localStorage failures
        }
        try {
          window.dispatchEvent(new CustomEvent('ops-monitoring-disabled'))
        } catch {
          // ignore event failures
        }

        if (window.location.pathname.startsWith('/admin/ops')) {
          window.location.href = '/admin/settings'
        }

        return Promise.reject({
          status,
          code: 'OPS_DISABLED',
          message: apiData.message || error.message,
          url
        })
      }

      if (status === 423 && apiData.code === 'ADMIN_COMPLIANCE_ACK_REQUIRED') {
        try {
          window.dispatchEvent(new CustomEvent('admin-compliance-required', {
            detail: apiData.metadata || {}
          }))
        } catch {
          // ignore event failures
        }

        return Promise.reject({
          status,
          code: apiData.code,
          message: apiData.message || error.message,
          metadata: apiData.metadata,
        })
      }

      // 401: Try to refresh the token via HttpOnly cookie.
      // This handles TOKEN_EXPIRED, INVALID_TOKEN, TOKEN_REVOKED, etc.
      if (status === 401 && !originalRequest._retry) {
        const legacyRefreshToken = getLegacyRefreshToken()
        const isAuthEndpoint =
          url.includes('/auth/login') || url.includes('/auth/register') || url.includes('/auth/refresh')

        // If this is not an auth endpoint, try to refresh with cookie first.
        if (!isAuthEndpoint) {
          if (isRefreshing) {
            // Wait for the ongoing refresh to complete
            return new Promise((resolve, reject) => {
              subscribeTokenRefresh((newToken: string) => {
                if (newToken) {
                  // Mark as retried to prevent infinite loop if retry also returns 401
                  originalRequest._retry = true
                  if (originalRequest.headers) {
                    originalRequest.headers.Authorization = `Bearer ${newToken}`
                  }
                  resolve(apiClient(originalRequest))
                } else {
                  // Refresh failed, reject with original error
                  reject({
                    status,
                    code: apiData.code,
                    message: apiData.message || apiData.detail || error.message
                  })
                }
              })
            })
          }

          originalRequest._retry = true
          isRefreshing = true

          try {
            // Call refresh endpoint directly to avoid circular dependency
            const refreshResponse = await axios.post(
              `${API_BASE_URL}/auth/refresh`,
              legacyRefreshToken ? { refresh_token: legacyRefreshToken } : {},
              {
                headers: { 'Content-Type': 'application/json' },
                withCredentials: true
              }
            )

            const refreshData = refreshResponse.data as ApiResponse<{
              access_token: string
              refresh_token?: string
              expires_in: number
            }>

            if (refreshData.code === 0 && refreshData.data) {
              const { access_token, expires_in } = refreshData.data

              setAccessToken(access_token)
              setAccessTokenExpiresIn(expires_in)
              localStorage.removeItem('auth_token')
              localStorage.removeItem('refresh_token')
              localStorage.removeItem('token_expires_at')

              // Notify subscribers with new token
              onTokenRefreshed(access_token)

              // Retry the original request with new token
              if (originalRequest.headers) {
                originalRequest.headers.Authorization = `Bearer ${access_token}`
              }

              isRefreshing = false
              return apiClient(originalRequest)
            }

            // Refresh response was not successful, fall through to clear auth
            throw new Error('Token refresh failed')
          } catch (refreshError) {
            // Refresh failed - notify subscribers with empty token
            onTokenRefreshed('')
            isRefreshing = false

            // Clear tokens and redirect to login
            clearAuthSession()
            localStorage.removeItem('auth_user')
            sessionStorage.setItem('auth_expired', '1')

            if (!window.location.pathname.includes('/login')) {
              window.location.href = '/login'
            }

            return Promise.reject({
              status: 401,
              code: 'TOKEN_REFRESH_FAILED',
              message: 'Session expired. Please log in again.'
            })
          }
        }

        // Auth endpoint failed or retry was not attempted - clear auth and redirect
        const hasToken = !!getAccessToken() || !!localStorage.getItem('auth_token')
        const headers = error.config?.headers as Record<string, unknown> | undefined
        const authHeader = headers?.Authorization ?? headers?.authorization
        const sentAuth =
          typeof authHeader === 'string'
            ? authHeader.trim() !== ''
            : Array.isArray(authHeader)
              ? authHeader.length > 0
              : !!authHeader

        clearAuthSession()
        localStorage.removeItem('auth_user')
        if ((hasToken || sentAuth) && !isAuthEndpoint) {
          sessionStorage.setItem('auth_expired', '1')
        }
        // Only redirect if not already on login page
        if (!window.location.pathname.includes('/login')) {
          window.location.href = '/login'
        }
      }

      // Return structured error
      return Promise.reject({
        status,
        code: apiData.code,
        reason: apiData.reason,
        error: apiData.error,
        message: apiData.message || apiData.detail || error.message,
        metadata: apiData.metadata,
      })
    }

    // Network error
    return Promise.reject({
      status: 0,
      message: 'Network error. Please check your connection.'
    })
  }
)

export default apiClient
