/**
 * 认证相关类型定义
 */

export interface LoginRequest {
    username: string
    password: string
}

export interface LoginResponse {
    token: string
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
