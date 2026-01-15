/**
 * 渠道管理 API 客户端
 */

import axios from 'axios'
import type {
  Channel,
  Key,
  ChannelModelConfig,
  CreateChannelRequest,
  UpdateChannelRequest,
  ChannelListResponse,
  CreateKeyRequest,
  ResetKeyRequest,
  ChannelHealthCheckResult,
  SingleKeyHealthResult,
  SyncResult,
  ApplySyncRequest,
  ApplySyncResponse
} from '../types/channel'

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080'

// 获取认证token
const getAuthToken = () => {
  return localStorage.getItem('access_token') || ''
}

// 创建axios实例
const apiClient = axios.create({
  baseURL: API_BASE_URL
})

// 请求拦截器 - 添加认证token
apiClient.interceptors.request.use((config) => {
  const token = getAuthToken()
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

export const channelApi = {
  /**
   * 获取渠道列表
   */
  async list(page = 1, pageSize = 20): Promise<ChannelListResponse> {
    const response = await apiClient.get<ChannelListResponse>(
      '/admin/api/channels',
      { params: { page, page_size: pageSize } }
    )
    return response.data
  },

  /**
   * 获取渠道详情
   */
  async get(id: number): Promise<Channel> {
    const response = await apiClient.get<Channel>(`/admin/api/channels/${id}`)
    return response.data
  },

  /**
   * 创建渠道
   */
  async create(data: CreateChannelRequest): Promise<Channel> {
    const response = await apiClient.post<Channel>('/admin/api/channels', data)
    return response.data
  },

  /**
   * 更新渠道
   */
  async update(id: number, data: UpdateChannelRequest): Promise<Channel> {
    const response = await apiClient.put<Channel>(`/admin/api/channels/${id}`, data)
    return response.data
  },

  /**
   * 删除渠道
   */
  async delete(id: number): Promise<void> {
    await apiClient.delete(`/admin/api/channels/${id}`)
  },

  /**
   * 添加Key
   */
  async addKey(data: CreateKeyRequest): Promise<Key> {
    const response = await apiClient.post<Key>('/admin/api/keys', data)
    return response.data
  },

  /**
   * 批量添加Keys
   */
  async batchAddKeys(channelId: number, keyValues: string[], remark?: string): Promise<any> {
    const response = await apiClient.post('/admin/api/keys/batch', {
      channel_id: channelId,
      key_values: keyValues,
      remark: remark || ''
    })
    return response.data
  },

  /**
   * 删除Key
   */
  async deleteKey(id: number): Promise<void> {
    await apiClient.delete(`/admin/api/keys/${id}`)
  },

  /**
   * 重置Key状态
   */
  async resetKey(id: number, data?: ResetKeyRequest): Promise<Key> {
    const response = await apiClient.patch<Key>(`/admin/api/keys/${id}`, data || {})
    return response.data
  },

  /**
   * 测试渠道所有Key的健康状态
   */
  async testKeys(channelId: number): Promise<ChannelHealthCheckResult> {
    const response = await apiClient.post<ChannelHealthCheckResult>(
      `/admin/api/channels/${channelId}/test-keys`
    )
    return response.data
  },

  /**
   * 测试单个Key的健康状态
   */
  async testSingleKey(keyId: number): Promise<SingleKeyHealthResult> {
    const response = await apiClient.post<SingleKeyHealthResult>(
      `/admin/api/keys/${keyId}/test`
    )
    return response.data
  },

  /**
   * 同步渠道模型
   */
  async syncModels(channelId: number): Promise<SyncResult> {
    const response = await apiClient.post<SyncResult>(
      `/admin/api/channels/${channelId}/sync-models`
    )
    return response.data
  },

  /**
   * 创建模型配置
   */
  async createModelConfig(
    channelId: number,
    unifiedModel: string,
    upstreamModel: string,
    remark?: string
  ): Promise<ChannelModelConfig> {
    const response = await apiClient.post<ChannelModelConfig>(
      '/admin/api/channel-models',
      {
        channel_id: channelId,
        unified_model: unifiedModel,
        upstream_model: upstreamModel,
        remark: remark || ''
      }
    )
    return response.data
  },

  /**
   * 更新模型配置
   */
  async updateModelConfig(
    id: number,
    data: Partial<ChannelModelConfig>
  ): Promise<ChannelModelConfig> {
    const response = await apiClient.put<ChannelModelConfig>(
      `/admin/api/channel-models/${id}`,
      data
    )
    return response.data
  },

  /**
   * 删除模型配置
   */
  async deleteModelConfig(id: number): Promise<void> {
    await apiClient.delete(`/admin/api/channel-models/${id}`)
  },

  /**
   * 应用模型同步
   */
  async applySync(channelId: number, data: ApplySyncRequest): Promise<ApplySyncResponse> {
    const response = await apiClient.post<ApplySyncResponse>(
      `/admin/api/channels/${channelId}/apply-sync`,
      data
    )
    return response.data
  },

  /**
   * 测试单个模型
   */
  async testModel(channelId: number, upstreamModel: string, unifiedModel: string): Promise<any> {
    const response = await apiClient.post(
      `/admin/api/channels/${channelId}/test-model`,
      {
        upstream_model: upstreamModel,
        unified_model: unifiedModel
      }
    )
    return response.data
  }
}
