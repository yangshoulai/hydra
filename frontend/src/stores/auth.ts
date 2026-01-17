/**
 * 认证状态管理 Store
 */

import {defineStore} from 'pinia'
import {computed, ref} from 'vue'
import {authApi} from '../services/authService'
import type {AdminUser, LoginRequest} from '../types/auth'

export const useAuthStore = defineStore('auth', () => {
    // State
    const token = ref<string | null>(null)
    const user = ref<AdminUser | null>(null)
    const loading = ref(false)
    const error = ref<string | null>(null)

    // Getters
    const isAuthenticated = computed(() => !!token.value)
    const currentUser = computed(() => user.value)

    // Actions

    /**
     * 初始化认证状态 - 从localStorage恢复token
     */
    function initialize() {
        const savedToken = localStorage.getItem('access_token')
        if (savedToken) {
            token.value = savedToken
            // 可以选择自动获取用户信息
            fetchUserInfo()
        }
    }

    /**
     * 登录
     */
    async function login(credentials: LoginRequest) {
        loading.value = true
        error.value = null

        try {
            const response = await authApi.login(credentials)

            // 保存token
            token.value = response.token
            localStorage.setItem('access_token', response.token)

            // 保存用户信息
            user.value = response.user

            return {success: true}
        } catch (err: any) {
            error.value = err.response?.data?.error || '登录失败'
            return {success: false, error: error.value}
        } finally {
            loading.value = false
        }
    }

    /**
     * 登出
     */
    async function logout() {
        try {
            if (token.value) {
                await authApi.logout()
            }
        } catch (err) {
            console.error('Logout error:', err)
        } finally {
            // 清除本地状态
            token.value = null
            user.value = null
            error.value = null

            // 清除localStorage
            localStorage.removeItem('access_token')
            localStorage.removeItem('token_expires_at')
        }
    }

    /**
     * 获取当前用户信息
     */
    async function fetchUserInfo() {
        if (!token.value) {
            return
        }

        try {
            const userInfo = await authApi.me()
            user.value = userInfo
        } catch (err: any) {
            console.error('Failed to fetch user info:', err)
            // 如果token无效，清除认证状态
            if (err.response?.status === 401) {
                await logout()
            }
        }
    }

    /**
     * 检查token是否过期
     */
    function isTokenExpired(): boolean {
        const expiresAt = localStorage.getItem('token_expires_at')
        if (!expiresAt) {
            return false
        }
        return Date.now() > parseInt(expiresAt)
    }

    /**
     * 设置认证状态（用于从localStorage恢复）
     */
    function setAuth(authToken: string, authUser?: AdminUser) {
        token.value = authToken
        if (authUser) {
            user.value = authUser
        }
    }

    return {
        // State
        token,
        user,
        loading,
        error,

        // Getters
        isAuthenticated,
        currentUser,

        // Actions
        initialize,
        login,
        logout,
        fetchUserInfo,
        isTokenExpired,
        setAuth
    }
})
