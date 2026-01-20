/**
 * 认证相关类型定义
 */

export interface LoginRequest {
    username: string
    password: string
}

export interface LoginResponse {
    access_token: string
    refresh_token: string
    user: AdminUser
}

export interface RefreshTokenRequest {
    refresh_token: string
}

export interface RefreshTokenResponse {
    access_token: string
    refresh_token: string
}

export interface AdminUser {
    id: number
    username: string
    email?: string
    created_at: string
    updated_at: string
}
