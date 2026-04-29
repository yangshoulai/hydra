import axios, { AxiosError, type InternalAxiosRequestConfig } from 'axios'
import { getErrorMessage } from '@/utils/error'
import { clearAuthTokens, getAccessToken, redirectToLogin, refreshAuthSession } from '@/services/authSession'
import { feedback } from '@/services/feedback'

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || ''

export interface RequestConfig extends InternalAxiosRequestConfig {
  // 默认不 toast，调用方自行用 toastApiError/自定义方式处理
  // 如需统一 toast，显式设置 autoToast: true
  autoToast?: boolean
}

export const apiClient = axios.create({
  baseURL: API_BASE_URL,
})

apiClient.interceptors.request.use((config) => {
  const token = getAccessToken()
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

apiClient.interceptors.response.use(
  (response) => response,
  async (error: AxiosError) => {
    const originalRequest = error.config as (RequestConfig & { _retry?: boolean }) | undefined
    const requestUrl = originalRequest?.url || ''
    const isAuthLoginRequest = requestUrl.includes('/auth/login')
    const isAuthRefreshRequest = requestUrl.includes('/auth/refresh')

    // 401 刷新分支：保持原有行为
    if (
      originalRequest &&
      error.response?.status === 401 &&
      !originalRequest._retry &&
      !isAuthLoginRequest &&
      !isAuthRefreshRequest
    ) {
      originalRequest._retry = true

      try {
        const newAccessToken = await refreshAuthSession()
        originalRequest.headers.Authorization = `Bearer ${newAccessToken}`
        return apiClient(originalRequest)
      } catch (refreshError) {
        clearAuthTokens()
        redirectToLogin()
        return Promise.reject(refreshError)
      }
    }

    // 其他错误：仅当请求显式开启 autoToast 时才统一 toast
    if (originalRequest?.autoToast && error.response) {
      const msg = getErrorMessage(error)
      feedback.message?.error(msg)
    }

    return Promise.reject(error)
  },
)
