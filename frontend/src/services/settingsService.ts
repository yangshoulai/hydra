import { apiClient } from './api'

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

export interface TestNotificationRequest {
  channel: 'telegram'
  telegram_bot_token?: string
  telegram_chat_id?: string
}

// Settings API client
class SettingsService {
  // 获取所有系统设置
  async getAllSettings(): Promise<SystemSettings> {
    const response = await apiClient.get('/admin/api/settings')
    return response.data.data
  }

  // 获取单个设置
  async getSetting(key: string): Promise<Setting> {
    const response = await apiClient.get(`/admin/api/settings/${key}`)
    return response.data.data
  }

  // 批量更新设置
  async updateSettings(request: UpdateSettingsRequest): Promise<void> {
    await apiClient.put('/admin/api/settings', request)
  }

  // 更新单个设置
  async updateSetting(key: string, value: string): Promise<void> {
    await apiClient.put(`/admin/api/settings/${key}`, { value })
  }

  // 测试通知渠道
  async testNotification(request: TestNotificationRequest): Promise<void> {
    await apiClient.post('/admin/api/settings/notifications/test', request)
  }
}

export const settingsService = new SettingsService()
