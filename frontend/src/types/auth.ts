/**
 * 认证相关类型定义
 */

export interface LoginRequest {
  username: string
  password: string
}

export interface LoginResponse {
  access_token: string
  token_type: string
  expires_in: number
  user: AdminUser
}

export interface AdminUser {
  id: number
  username: string
  email?: string
  created_at: string
  updated_at: string
}

export interface AuthMeResponse {
  id: number
  username: string
  email?: string
  created_at: string
  updated_at: string
}
