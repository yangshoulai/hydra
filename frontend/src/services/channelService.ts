/**
 * 渠道管理 API 客户端
 */

import { apiClient } from './api'
import type {
  Channel,
  ChannelKey,
  ChannelModelConfig,
  ModelRelatedChannelInfo,
  CreateChannelRequest,
  UpdateChannelRequest,
  ChannelListResponse,
  ChannelListParams,
  CreateChannelKeyRequest,
  ResetChannelKeyRequest,
  ChannelHealthCheckResult,
  SingleChannelKeyHealthResult,
  SyncResult,
  ApplySyncRequest,
  ApplySyncResponse,
  TestModelResponse
} from '../types/channel'

export const channelApi = {
  /**
   * 获取渠道列表
   */
  async list(params?: ChannelListParams): Promise<ChannelListResponse> {
    const response = await apiClient.get<ChannelListResponse>(
      '/admin/api/channels',
      { params }
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
   * 添加渠道密钥
   */
  async addChannelKey(data: CreateChannelKeyRequest): Promise<ChannelKey> {
    const response = await apiClient.post<ChannelKey>('/admin/api/channel-keys', data)
    return response.data
  },

  /**
   * 批量添加渠道密钥
   */
  async batchAddChannelKeys(channelId: number, channelKeyValues: string[], remark?: string, channelKeyGroup?: string): Promise<any> {
    const response = await apiClient.post('/admin/api/channel-keys/batch', {
      channel_id: channelId,
      channel_key_values: channelKeyValues,
      remark: remark || '',
      channel_key_group: channelKeyGroup || 'Default'
    })
    return response.data
  },

  /**
   * 删除渠道密钥
   */
  async deleteChannelKey(id: number): Promise<void> {
    await apiClient.delete(`/admin/api/channel-keys/${id}`)
  },

  /**
   * 重置渠道密钥状态
   */
  async resetChannelKey(id: number, data?: ResetChannelKeyRequest): Promise<ChannelKey> {
    const response = await apiClient.patch<ChannelKey>(`/admin/api/channel-keys/${id}`, data || {})
    return response.data
  },

  /**
   * 重置渠道密钥状态（启用/停用）
   */
  async resetChannelKeyStatus(id: number, status: 'active' | 'inactive'): Promise<ChannelKey> {
    const response = await apiClient.patch<ChannelKey>(`/admin/api/channel-keys/${id}`, { status })
    return response.data
  },

  /**
   * 测试渠道所有渠道密钥的健康状态
   */
  async testChannelKeys(channelId: number): Promise<ChannelHealthCheckResult> {
    const response = await apiClient.post<ChannelHealthCheckResult>(
      `/admin/api/channels/${channelId}/test-channel-keys`
    )
    return response.data
  },

  /**
   * 测试单个渠道密钥的健康状态
   */
  async testSingleChannelKey(channelKeyId: number): Promise<SingleChannelKeyHealthResult> {
    const response = await apiClient.post<SingleChannelKeyHealthResult>(
      `/admin/api/channel-keys/${channelKeyId}/test`
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
    model: string,
    channelModel: string,
    weight?: number,
    endpointTypes?: string[],
    keyGroups?: string[],
    testPrompt?: string,
  ): Promise<ChannelModelConfig> {
    const response = await apiClient.post<ChannelModelConfig>(
      '/admin/api/channel-models',
      {
        channel_id: channelId,
        model: model,
        channel_model: channelModel,
        weight: weight,
        endpoint_types: endpointTypes || ['OpenAIChatCompletions'],
        key_groups: keyGroups || ['Default'],
        test_prompt: testPrompt || '',
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
  async testModel(
    channelId: number,
    channelModel: string,
    model: string,
    endpointType: string,
    keyGroups?: string[],
    options?: {
      testPrompt?: string
      imageData?: string
      imageSize?: string
      imageQuality?: string
      clientHeaderProfileId?: string
    }
  ): Promise<TestModelResponse> {
    const response = await apiClient.post<TestModelResponse>(
      `/admin/api/channels/${channelId}/test-model`,
      {
        channel_model: channelModel,
        model: model,
        endpoint_type: endpointType,
        key_groups: keyGroups || [],
        test_prompt: options?.testPrompt || '',
        image_data: options?.imageData || '',
        image_size: options?.imageSize || '',
        image_quality: options?.imageQuality || '',
        client_header_profile_id: options?.clientHeaderProfileId || '',
      }
    )
    return response.data
  },

  /**
   * 获取模型关联的渠道列表
   */
  async getChannelsByModel(modelId: number): Promise<ModelRelatedChannelInfo[]> {
    const response = await apiClient.get<ModelRelatedChannelInfo[]>(`/admin/api/models/${modelId}/channels`)
    return response.data
  },

  /**
   * 切换渠道模型配置状态
   */
  async toggleChannelModelStatus(configId: number): Promise<ChannelModelConfig> {
    const response = await apiClient.patch<ChannelModelConfig>(
      `/admin/api/channel-models/${configId}/toggle-status`
    )
    return response.data
  }
}
