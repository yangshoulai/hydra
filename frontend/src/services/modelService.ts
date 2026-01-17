/**
 * 模型管理 API 客户端
 */

import { apiClient } from './api'
import type {
  Model,
  CreateModelRequest,
  UpdateModelRequest,
} from '@/types/model'

export const modelApi = {
  /**
   * 获取所有模型
   */
  async list(): Promise<Model[]> {
    const response = await apiClient.get<Model[]>('/admin/api/models')
    return response.data
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
