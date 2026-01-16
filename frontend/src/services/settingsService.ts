import axios from 'axios'

const BASE_URL = '/admin/api'

// Settings types
export interface SystemSettings {
  [key: string]: string
}

export interface UpdateSettingsRequest {
  settings: SystemSettings
}

export interface Setting {
  key: string
  value: string
  created_at: string
  updated_at: string
}

// 明文错误规则相关类型
export interface PlainTextErrorRulesResponse {
  data: string[]
}

export interface UpdatePlainTextErrorRulesRequest {
  keywords: string[]
}

// Settings API client
class SettingsService {
  // 获取所有系统设置
  async getAllSettings(): Promise<SystemSettings> {
    const response = await axios.get(`${BASE_URL}/settings`)
    return response.data.data
  }

  // 获取单个设置
  async getSetting(key: string): Promise<Setting> {
    const response = await axios.get(`${BASE_URL}/settings/${key}`)
    return response.data.data
  }

  // 批量更新设置
  async updateSettings(request: UpdateSettingsRequest): Promise<void> {
    await axios.put(`${BASE_URL}/settings`, request)
  }

  // 更新单个设置
  async updateSetting(key: string, value: string): Promise<void> {
    await axios.put(`${BASE_URL}/settings/${key}`, { value })
  }

  // 获取明文错误规则
  async getPlainTextErrorRules(): Promise<string[]> {
    const response = await axios.get<PlainTextErrorRulesResponse>(`${BASE_URL}/settings/sniffer/plain-text-rules`)
    return response.data.data
  }

  // 更新明文错误规则
  async updatePlainTextErrorRules(keywords: string[]): Promise<void> {
    const request: UpdatePlainTextErrorRulesRequest = { keywords }
    await axios.put(`${BASE_URL}/settings/sniffer/plain-text-rules`, request)
  }
}

export default new SettingsService()
