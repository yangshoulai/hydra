import axios from 'axios'

const BASE_URL = '/admin/api'

// Token types
export interface Token {
  id: number
  name: string
  token_preview: string
  status: string
  created_at: string
  last_used_at?: string
  expires_at?: string // 过期时间
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

// Tokens API client
class TokensService {
  // 获取令牌列表
  async getAllTokens(): Promise<Token[]> {
    const response = await axios.get(`${BASE_URL}/tokens`)
    return response.data.data
  }

  // 创建令牌
  async createToken(request: CreateTokenRequest): Promise<CreateTokenResponse> {
    const response = await axios.post(`${BASE_URL}/tokens`, request)
    return response.data.data
  }

  // 删除令牌
  async deleteToken(id: number): Promise<void> {
    await axios.delete(`${BASE_URL}/tokens/${id}`)
  }

  // 切换令牌状态
  async toggleTokenStatus(id: number): Promise<void> {
    await axios.patch(`${BASE_URL}/tokens/${id}/toggle`)
  }
}

export default new TokensService()
