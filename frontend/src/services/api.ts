import axios, { AxiosError, type InternalAxiosRequestConfig } from 'axios'
import { getErrorMessage } from '@/utils/error'

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || ''

export interface RequestConfig extends InternalAxiosRequestConfig {
  // 默认不 toast，调用方自行用 toastApiError/自定义方式处理
  // 如需统一 toast，显式设置 autoToast: true
  autoToast?: boolean
}

export const apiClient = axios.create({
  baseURL: API_BASE_URL,
})

// 用于存储正在刷新的 Promise，避免多个请求同时刷新
let refreshingPromise: Promise<string> | null = null

apiClient.interceptors.request.use((config) => {
  const token = localStorage.getItem('access_token') || ''
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

apiClient.interceptors.response.use(
  (response) => response,
  async (error: AxiosError) => {
    const originalRequest = error.config as (RequestConfig & { _retry?: boolean }) | undefined

    // 401 刷新分支：保持原有行为
    if (
      originalRequest &&
      error.response?.status === 401 &&
      !originalRequest._retry &&
      !originalRequest.url?.includes('/auth/refresh')
    ) {
      originalRequest._retry = true

      try {
        if (refreshingPromise) {
          const newAccessToken = await refreshingPromise
          originalRequest.headers.Authorization = `Bearer ${newAccessToken}`
          return apiClient(originalRequest)
        }

        const refreshToken = localStorage.getItem('refresh_token')
        if (!refreshToken) {
          throw new Error('No refresh token')
        }

        refreshingPromise = (async () => {
          const response = await axios.post(`${API_BASE_URL}/admin/api/auth/refresh`, {
            refresh_token: refreshToken,
          })
          const { access_token, refresh_token: new_refresh_token } = response.data
          if (!access_token) {
            localStorage.removeItem('access_token')
            localStorage.removeItem('refresh_token')
            window.location.href = '/login'
            throw new Error('No access token in refresh response')
          }
          localStorage.setItem('access_token', access_token)
          localStorage.setItem('refresh_token', new_refresh_token)
          return access_token
        })()

        const newAccessToken = await refreshingPromise
        refreshingPromise = null

        originalRequest.headers.Authorization = `Bearer ${newAccessToken}`
        return apiClient(originalRequest)
      } catch (refreshError) {
        refreshingPromise = null
        localStorage.removeItem('access_token')
        localStorage.removeItem('refresh_token')
        window.location.href = '/login'
        return Promise.reject(refreshError)
      }
    }

    // 其他错误：仅当请求显式开启 autoToast 时才统一 toast
    if (originalRequest?.autoToast && error.response) {
      const msg = getErrorMessage(error)
      window.$message?.error(msg)
    }

    return Promise.reject(error)
  },
)
