import { apiClient } from './api'
import type {
  Provider,
  CreateProviderRequest,
  UpdateProviderRequest,
  RemoteProvider,
} from '@/types/model'

export const providerApi = {
  // 获取厂商列表
  async list(): Promise<Provider[]> {
    const response = await apiClient.get<Provider[] | null>('/admin/api/providers')
    return Array.isArray(response.data) ? response.data : []
  },

  // 获取单个厂商
  async get(id: string): Promise<Provider> {
    const response = await apiClient.get<Provider>(`/admin/api/providers/${id}`)
    return response.data
  },

  // 创建厂商
  async create(data: CreateProviderRequest): Promise<Provider> {
    const response = await apiClient.post<Provider>('/admin/api/providers', data)
    return response.data
  },

  // 更新厂商
  async update(id: string, data: UpdateProviderRequest): Promise<Provider> {
    const response = await apiClient.put<Provider>(`/admin/api/providers/${id}`, data)
    return response.data
  },

  // 删除厂商
  async delete(id: string): Promise<void> {
    await apiClient.delete(`/admin/api/providers/${id}`)
  },

  // 同步远程厂商数据（调用后端接口）
  async syncRemoteProviders(): Promise<RemoteProvider[]> {
    const response = await apiClient.get<RemoteProvider[] | null>('/admin/api/providers/sync')
    return Array.isArray(response.data) ? response.data : []
  },

  // 批量创建厂商
  async batchCreate(providers: CreateProviderRequest[]): Promise<{ created: number; failed: number; data: Provider[] }> {
    const response = await apiClient.post<{ created: number; failed: number; data: Provider[] }>('/admin/api/providers/batch', providers)
    return response.data
  },
}
