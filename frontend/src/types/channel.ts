/**
 * 渠道相关类型定义
 */

export interface Channel {
  id: number
  name: string
  base_url: string
  priority: number
  weight: number
  status: 'active' | 'disabled'
  description: string
  model_count?: number  // 已配置的模型数量
  key_stats?: KeyStats  // 密钥统计
  created_at: string
  updated_at: string
  keys?: Key[]
  model_configs?: ChannelModelConfig[]
}

export interface KeyStats {
  active: number    // 健康的密钥数量
  cooling: number   // 冷却中的密钥数量
  disabled: number  // 禁用的密钥数量
}

export interface Key {
  id: number
  channel_id: number
  key_value: string
  key_preview?: string  // 脱敏的key（前6位+**********+后4位）
  status: 'active' | 'cooling' | 'disabled'
  remark: string
  cooling_at?: string
  created_at: string
  updated_at: string
}

export interface ChannelModelConfig {
  id: number
  channel_id: number
  unified_model: string
  upstream_model: string
  status: 'active' | 'disabled'
  remark: string
  created_at: string
  updated_at: string
}

export interface CreateChannelRequest {
  name: string
  base_url: string
  priority?: number
  weight?: number
  status?: 'active' | 'disabled'
  description?: string
}

export interface UpdateChannelRequest {
  name?: string
  base_url?: string
  priority?: number
  weight?: number
  status?: 'active' | 'disabled'
  description?: string
}

export interface ChannelListResponse {
  total: number
  page: number
  page_size: number
  items: Channel[]
}

export interface ChannelListParams {
  page?: number
  page_size?: number
  name?: string
  base_url?: string
  status?: 'active' | 'disabled' | null
  sort_by?: 'id' | 'name' | 'priority' | 'weight' | 'status'
  sort_order?: 'asc' | 'desc'
}

export interface CreateKeyRequest {
  channel_id: number
  key_value: string
  remark?: string
}

export interface ResetKeyRequest {
  status?: 'active' | 'cooling' | 'disabled'
}

export interface KeyHealthResult {
  key_id: number
  key_remark: string
  key_value?: string
  key_preview?: string
  status: 'healthy' | 'unhealthy' | 'error'
  message: string
  latency: string
}

export interface SingleKeyHealthResult {
  key_id: number
  key_remark: string
  status: 'healthy' | 'unhealthy' | 'error'
  message: string
  latency: string
}

export interface ChannelHealthCheckResult {
  channel_id: number
  channel_name: string
  total_keys: number
  healthy_keys: number
  key_results: KeyHealthResult[]
}

// 模型同步相关类型
export interface ModelDiffType {
  type: 'added' | 'removed' | 'existing'
  unified_model: string
  upstream_model: string
  existing_config?: ChannelModelConfig
}

export interface SyncDiff {
  total_upstream_models: number
  total_local_models: number
  added_count: number
  removed_count: number
  existing_count: number
  diffs: ModelDiffType[]
}

export interface SyncResult {
  success: boolean
  message: string
  channel_id: number
  channel_name: string
  fetched_at: string
  upstream_models: string[]
  diff: SyncDiff
}

// 应用同步相关类型
export interface ModelConfigItem {
  unified_model: string
  upstream_model: string
  remark?: string
}

export interface ModelConfigUpdateItem {
  id: number
  unified_model: string
  upstream_model: string
  remark?: string
}

export interface ApplySyncRequest {
  add_models: ModelConfigItem[]
  update_models: ModelConfigUpdateItem[]
  delete_model_ids: number[]
}

export interface ApplySyncResponse {
  success: boolean
  message: string
  added_count: number
  updated_count: number
  deleted_count: number
}
