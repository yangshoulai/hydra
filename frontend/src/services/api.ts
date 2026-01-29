import axios from 'axios'

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL

export const apiClient = axios.create({
    baseURL: API_BASE_URL
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
    async (error) => {
        const originalRequest = error.config

        // 如果是 401 错误且不是刷新接口本身，尝试刷新 token
        if (error.response?.status === 401 && !originalRequest._retry && !originalRequest.url?.includes('/auth/refresh')) {
            originalRequest._retry = true

            try {
                // 如果已经有正在刷新的请求，等待它完成
                if (refreshingPromise) {
                    const newAccessToken = await refreshingPromise
                    originalRequest.headers.Authorization = `Bearer ${newAccessToken}`
                    return apiClient(originalRequest)
                }

                // 开始刷新
                const refreshToken = localStorage.getItem('refresh_token')
                if (!refreshToken) {
                    throw new Error('No refresh token')
                }

                refreshingPromise = (async () => {
                    const response = await axios.post(`${API_BASE_URL}/admin/api/auth/refresh`, {
                        refresh_token: refreshToken
                    })
                    const {access_token, refresh_token: new_refresh_token} = response.data
                    // 防御性校验：后端未返回 access_token 时直接回登录页
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

                // 使用新 token 重试原请求
                originalRequest.headers.Authorization = `Bearer ${newAccessToken}`
                return apiClient(originalRequest)
            } catch (refreshError) {
                // 刷新失败，清除 token 并跳转登录页
                refreshingPromise = null
                localStorage.removeItem('access_token')
                localStorage.removeItem('refresh_token')
                window.location.href = '/login'
                return Promise.reject(refreshError)
            }
        }

        return Promise.reject(error)
    }
)
