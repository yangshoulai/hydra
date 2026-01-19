/**
 * 认证 API 客户端
 */

import {apiClient} from './api'
import type {LoginRequest, LoginResponse} from '../types/auth'


export const authApi = {
    /**
     * 管理员登录
     */
    async login(data: LoginRequest): Promise<LoginResponse> {
        const response = await apiClient.post<LoginResponse>(
            `/admin/api/auth/login`,
            data
        )
        return response.data
    },

    /**
     * 修改密码
     */
    async changePassword(data: { old_password: string; new_password: string }): Promise<void> {
        await apiClient.post('/admin/api/auth/change-password', data)
    }
}
