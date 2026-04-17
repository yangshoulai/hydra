/**
 * 模型管理 API 客户端
 */

import { apiClient } from './api'
import type {
  Model,
  CreateModelRequest,
  UpdateModelRequest,
  ModelListParams,
  ModelListResponse,
} from '@/types/model'

export const modelApi = {
  /**
   * 获取模型列表（分页，支持过滤和排序）
   */
  async list(params?: ModelListParams): Promise<ModelListResponse> {
    const response = await apiClient.get<Partial<ModelListResponse>>('/admin/api/models', { params })
    const data = response.data ?? {}

    return {
      total: typeof data.total === 'number' ? data.total : 0,
      page: typeof data.page === 'number' ? data.page : (params?.page ?? 1),
      page_size: typeof data.page_size === 'number' ? data.page_size : (params?.page_size ?? 20),
      items: Array.isArray(data.items) ? data.items : [],
    }
  },

  /**
   * 获取所有模型（不分页，已废弃）
   */
  async listAll(): Promise<Model[]> {
    const response = await apiClient.get<Model[]>('/admin/api/models')
    return Array.isArray(response.data) ? response.data : []
  },

  /**
   * 获取模型详情
   */
  async get(id: number): Promise<Model> {
    const response = await apiClient.get<Model>(`/admin/api/models/${id}`)
    return response.data
  },

  /**
   * 创建模型
   */
  async create(data: CreateModelRequest): Promise<Model> {
    const response = await apiClient.post<Model>('/admin/api/models', data)
    return response.data
  },

  /**
   * 更新模型
   */
  async update(id: number, data: UpdateModelRequest): Promise<Model> {
    const response = await apiClient.put<Model>(`/admin/api/models/${id}`, data)
    return response.data
  },

  /**
   * 删除模型
   */
  async delete(id: number): Promise<void> {
    await apiClient.delete(`/admin/api/models/${id}`)
  }
}

export type { CreateModelRequest, UpdateModelRequest }
