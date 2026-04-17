/**
 * 统一模型相关类型定义
 */

export interface Provider {
  id: string
  name: string
  icon: string
  remark: string
  created_at: string
  updated_at: string
  /** 关联模型数（由后端列表接口聚合返回） */
  model_count?: number
}

export interface Model {
  id: number
  name: string
  provider_id: string | null
  provider?: Provider
  remark: string
  created_at: string
  updated_at: string
  /** 关联渠道数（由后端列表接口聚合返回） */
  channel_count?: number
}

export interface CreateProviderRequest {
  id: string
  name: string
  icon?: string
  remark?: string
}

export interface UpdateProviderRequest {
  name?: string
  icon?: string
  remark?: string
}

export interface RemoteProvider {
  id: string
  name: string
  iconURL: string
}

export interface CreateModelRequest {
  name: string
  provider_id: string | null
  remark?: string
}

export interface UpdateModelRequest {
  name?: string
  provider_id?: string | null
  remark?: string
}

export interface ModelListParams {
  page?: number
  page_size?: number
  name?: string
  provider_id?: string | null
  sort_by?: 'id' | 'name'
  sort_order?: 'asc' | 'desc'
}

export interface ModelListResponse {
  total: number
  page: number
  page_size: number
  items: Model[]
}
