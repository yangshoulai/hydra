/**
 * 统一模型相关类型定义
 */

export interface Provider {
  id: string
  name: string
  icon: string
  lobeIcon?: string
  remark: string
  created_at: string
  updated_at: string
}

export interface Model {
  id: number
  name: string
  provider_id: string | null
  provider?: Provider
  remark: string
  channel_count: number
  created_at: string
  updated_at: string
}

export interface CreateProviderRequest {
  id: string
  name: string
  icon?: string
  lobeIcon?: string
  remark?: string
}

export interface UpdateProviderRequest {
  name?: string
  icon?: string
  lobeIcon?: string
  remark?: string
}

export interface RemoteProvider {
  id: string
  name: string
  iconURL: string
  lobeIcon?: string
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
