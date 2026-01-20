/**
 * 端点管理 API 客户端
 */

import { apiClient } from './api'
import type { EndpointInfo } from '../types/endpoint'

export const endpointApi = {
  /**
   * 获取所有支持的端点类型
   */
  async list(): Promise<EndpointInfo[]> {
    const response = await apiClient.get<EndpointInfo[]>('/admin/api/endpoints')
    return response.data
  }
}
