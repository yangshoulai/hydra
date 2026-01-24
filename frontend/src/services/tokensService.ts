import {apiClient} from './api'

// Token types
export interface Token {
    id: number
    name: string
    token: string
    token_preview: string
    status: string
    created_at: string
    last_used_at?: string
    expires_at?: string // 过期时间
    prompt_tokens: number
    completion_tokens: number
}

export interface CreateTokenRequest {
    name: string
    expires_at?: string // 过期时间，格式：YYYY-MM-DD HH:mm:ss
}

export interface CreateTokenResponse {
    id: number
    name: string
    token_preview: string
    access_token: string
    created_at: string
    message: string
}

export interface TokenListParams {
    page?: number
    page_size?: number
    name?: string
    status?: string | null
    token?: string
    sort_by?: 'id' | 'status' | 'created_at' | 'last_used_at'
    sort_order?: 'asc' | 'desc'
}

export interface TokenListResponse {
    total: number
    page: number
    page_size: number
    items: Token[]
}

// Tokens API client
class TokensService {
    // 获取令牌列表（分页，支持过滤和排序）
    async list(params?: TokenListParams): Promise<TokenListResponse> {
        const response = await apiClient.get<TokenListResponse>('/admin/api/tokens', { params })
        return response.data
    }

    // 获取所有令牌（不分页，已废弃）
    async getAllTokens(): Promise<Token[]> {
        const response = await apiClient.get('/admin/api/tokens')
        return response.data.data
    }

    // 创建令牌
    async createToken(request: CreateTokenRequest): Promise<CreateTokenResponse> {
        const response = await apiClient.post('/admin/api/tokens', request)
        return response.data.data
    }

    // 删除令牌
    async deleteToken(id: number): Promise<void> {
        await apiClient.delete(`/admin/api/tokens/${id}`)
    }

    // 切换令牌状态
    async toggleTokenStatus(id: number): Promise<void> {
        await apiClient.patch(`/admin/api/tokens/${id}/toggle`)
    }
}

export default new TokensService()
