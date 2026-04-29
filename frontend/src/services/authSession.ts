const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || ''

let refreshingPromise: Promise<string> | null = null

function resolveURL(path: string): string {
  if (/^https?:\/\//i.test(path)) return path
  return `${API_BASE_URL}${path}`
}

export function getAccessToken(): string {
  return localStorage.getItem('access_token') || ''
}

export function getRefreshToken(): string {
  return localStorage.getItem('refresh_token') || ''
}

export function saveAuthTokens(accessToken: string, refreshToken: string) {
  localStorage.setItem('access_token', accessToken)
  localStorage.setItem('refresh_token', refreshToken)
}

export function clearAuthTokens() {
  localStorage.removeItem('access_token')
  localStorage.removeItem('refresh_token')
}

function decodeJWTPayload(token: string): { exp?: number } | null {
  const [, payload] = token.split('.')
  if (!payload) return null
  try {
    const normalized = payload.replace(/-/g, '+').replace(/_/g, '/')
    const padded = normalized.padEnd(Math.ceil(normalized.length / 4) * 4, '=')
    return JSON.parse(atob(padded)) as { exp?: number }
  } catch {
    return null
  }
}

export function isAccessTokenExpired(token = getAccessToken(), skewMs = 30_000): boolean {
  if (!token) return true
  const payload = decodeJWTPayload(token)
  if (!payload?.exp) return false
  return Date.now() + skewMs >= payload.exp * 1000
}

export async function refreshAuthSession(): Promise<string> {
  if (refreshingPromise) return refreshingPromise

  refreshingPromise = (async () => {
    const refreshToken = getRefreshToken()
    if (!refreshToken) {
      throw new Error('No refresh token')
    }

    const response = await fetch(resolveURL('/admin/api/auth/refresh'), {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        refresh_token: refreshToken,
      }),
    })

    if (!response.ok) {
      throw new Error(`refresh token failed: ${response.status}`)
    }

    const data = await response.json()
    const accessToken = data?.access_token
    const newRefreshToken = data?.refresh_token
    if (!accessToken || !newRefreshToken) {
      throw new Error('invalid refresh response')
    }

    saveAuthTokens(accessToken, newRefreshToken)
    return accessToken
  })()

  try {
    return await refreshingPromise
  } finally {
    refreshingPromise = null
  }
}

export function redirectToLogin() {
  if (window.location.pathname !== '/login') {
    window.location.href = '/login'
  }
}
