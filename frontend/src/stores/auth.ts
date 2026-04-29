/**
 * 认证状态管理 Store
 */

import {defineStore} from 'pinia'
import {computed, ref} from 'vue'
import {authApi} from '../services/authService'
import {isAccessTokenExpired, saveAuthTokens} from '../services/authSession'
import type {AdminUser} from '../types/auth'

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
     * 登录
     */
    async function login(credentials: { username: string; password: string; rememberMe?: boolean }) {
        loading.value = true
        error.value = null

        try {
            const response = await authApi.login({
                username: credentials.username,
                password: credentials.password,
                remember_me: !!credentials.rememberMe,
            })

            // 保存 access_token 和 refresh_token
            token.value = response.access_token
            saveAuthTokens(response.access_token, response.refresh_token)

            // 保存用户信息
            user.value = response.user

            return {success: true}
        } catch (err) {
            const axiosErr = err as { response?: { data?: { error?: string } } }
            error.value = axiosErr.response?.data?.error || '登录失败'
            return {success: false, error: error.value}
        } finally {
            loading.value = false
        }
    }

    /**
     * 检查token是否过期
     */
    function isTokenExpired(): boolean {
        return isAccessTokenExpired()
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

    function clearError() {
        error.value = null
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
        login,
        isTokenExpired,
        setAuth,
        clearError
    }
})
