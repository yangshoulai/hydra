import { apiClient } from './api'

export interface Token {
  id: number
  name: string
  token: string
  token_preview: string
  status: string
  created_at: string
  last_used_at?: string
  expires_at?: string
  prompt_tokens: number
  completion_tokens: number
  allowed_models: string[]
}

export interface CreateTokenRequest {
  name: string
  expires_at?: string
  allowed_models?: string[]
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

class TokensService {
  async list(params?: TokenListParams): Promise<TokenListResponse> {
    const response = await apiClient.get<TokenListResponse>('/admin/api/tokens', { params })
    return response.data
  }

  async createToken(request: CreateTokenRequest): Promise<CreateTokenResponse> {
    const response = await apiClient.post('/admin/api/tokens', request)
    return response.data.data
  }

  async deleteToken(id: number): Promise<void> {
    await apiClient.delete(`/admin/api/tokens/${id}`)
  }

  async toggleTokenStatus(id: number): Promise<void> {
    await apiClient.patch(`/admin/api/tokens/${id}/toggle`)
  }

  async updateTokenModels(id: number, allowedModels: string[]): Promise<void> {
    await apiClient.patch(`/admin/api/tokens/${id}/models`, {
      allowed_models: allowedModels,
    })
  }
}

export default new TokensService()
