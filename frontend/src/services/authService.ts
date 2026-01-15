/**
 * 认证 API 客户端
 */

import axios from 'axios'
import type { LoginRequest, LoginResponse, AuthMeResponse } from '../types/auth'

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080'

export const authApi = {
  /**
   * 管理员登录
   */
  async login(data: LoginRequest): Promise<LoginResponse> {
    const response = await axios.post<LoginResponse>(
      `${API_BASE_URL}/admin/api/auth/login`,
      data
    )
    return response.data
  },

  /**
   * 登出
   */
  async logout(token: string): Promise<void> {
    await axios.post(
      `${API_BASE_URL}/admin/api/auth/logout`,
      {},
      {
        headers: {
          Authorization: `Bearer ${token}`
        }
      }
    )
  },

  /**
   * 获取当前用户信息
   */
  async me(token: string): Promise<AuthMeResponse> {
    const response = await axios.get<AuthMeResponse>(
      `${API_BASE_URL}/admin/api/auth/me`,
      {
        headers: {
          Authorization: `Bearer ${token}`
        }
      }
    )
    return response.data
  }
}
