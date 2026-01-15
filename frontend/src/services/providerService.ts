import axios from 'axios'
import type {
  Provider,
  CreateProviderRequest,
  UpdateProviderRequest,
  RemoteProvider,
} from '@/types/model'

const BASE_URL = '/admin/api/providers'

export const providerApi = {
  // 获取厂商列表
  async list(): Promise<Provider[]> {
    const response = await axios.get<Provider[]>(BASE_URL)
    return response.data
  },

  // 获取单个厂商
  async get(id: string): Promise<Provider> {
    const response = await axios.get<Provider>(`${BASE_URL}/${id}`)
    return response.data
  },

  // 创建厂商
  async create(data: CreateProviderRequest): Promise<Provider> {
    const response = await axios.post<Provider>(BASE_URL, data)
    return response.data
  },

  // 更新厂商
  async update(id: string, data: UpdateProviderRequest): Promise<Provider> {
    const response = await axios.put<Provider>(`${BASE_URL}/${id}`, data)
    return response.data
  },

  // 删除厂商
  async delete(id: string): Promise<void> {
    await axios.delete(`${BASE_URL}/${id}`)
  },

  // 同步远程厂商数据（调用后端接口）
  async syncRemoteProviders(): Promise<RemoteProvider[]> {
    const response = await axios.get<RemoteProvider[]>(`${BASE_URL}/sync`)
    return response.data
  },

  // 批量创建厂商
  async batchCreate(providers: CreateProviderRequest[]): Promise<{ created: number; failed: number; data: Provider[] }> {
    const response = await axios.post<{ created: number; failed: number; data: Provider[] }>(`${BASE_URL}/batch`, providers)
    return response.data
  },
}

export default providerApi
