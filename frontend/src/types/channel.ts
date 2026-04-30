/**
 * 渠道相关类型定义
 */

export interface Channel {
  id: number
  name: string
  base_url: string
  use_proxy: boolean
  weight: number
  status: 'active' | 'inactive'
  description: string
  model_count?: number  // 已配置的模型数量
  model_stats?: ModelStatusCount // 模型状态统计
  key_stats?: ChannelKeyStats  // 密钥统计
  created_at: string
  updated_at: string
  channel_keys?: ChannelKey[]
  model_configs?: ChannelModelConfig[]
}

export interface ChannelKeyStats {
  active: number
  inactive: number
}

export interface ModelStatusCount {
  active: number
  inactive: number
}

export interface ChannelKey {
  id: number
  channel_id: number
  channel_key_value: string
  channel_key_preview?: string  // 脱敏的 key（前6位+**********+后4位）
  status: 'active' | 'inactive'
  channel_key_group: string
  remark: string
  created_at: string
  updated_at: string
}

export interface ChannelModelConfig {
  id: number
  channel_id: number
  model: string
  channel_model: string
  weight: number
  status: 'active' | 'inactive'
  endpoint_types?: string[]
  key_groups?: string[]
  test_prompt?: string
  remark: string
  created_at: string
  updated_at: string
}

export interface ModelRelatedChannelInfo {
  config_id: number
  config_status: 'active' | 'inactive'
  weight: number
  channel_id: number
  channel_name: string
  channel_status: 'active' | 'inactive'
  channel_model: string
  endpoint_types: string[]
}

export interface CreateChannelRequest {
  name: string
  base_url: string
  use_proxy?: boolean
  weight?: number
  status?: 'active' | 'inactive'
  description?: string
}

export interface UpdateChannelRequest {
  name?: string
  base_url?: string
  use_proxy?: boolean
  weight?: number
  status?: 'active' | 'inactive'
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
  status?: 'active' | 'inactive' | null
  sort_by?: 'id' | 'name' | 'weight' | 'status'
  sort_order?: 'asc' | 'desc'
}

export interface CreateChannelKeyRequest {
  channel_id: number
  channel_key_value: string
  channel_key_group: string
  remark?: string
}

export interface ResetChannelKeyRequest {
  status?: 'active' | 'inactive'
}

export interface ChannelKeyHealthResult {
  channel_key_id: number
  channel_key_remark: string
  status: 'healthy' | 'unhealthy' | 'error'
  message: string
  latency: string
}

export interface SingleChannelKeyHealthResult {
  channel_key_id: number
  channel_key_remark: string
  status: 'healthy' | 'unhealthy' | 'error'
  message: string
  latency: string
}

export interface ChannelHealthCheckResult {
  channel_id: number
  channel_name: string
  total_channel_keys: number
  healthy_channel_keys: number
  channel_key_results: ChannelKeyHealthResult[]
}

// 模型同步相关类型
export interface ModelDiffType {
  type: 'added' | 'removed' | 'existing'
  model: string
  channel_model: string
  key_groups?: string[]
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
  model: string
  channel_model: string
  weight?: number
  endpoint_types?: string[]
  key_groups?: string[]
  test_prompt?: string
  remark?: string
}

export interface ModelConfigUpdateItem {
  id: number
  model: string
  channel_model: string
  weight?: number
  endpoint_types?: string[]
  key_groups?: string[]
  test_prompt?: string
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

export interface TestModeResult {
  tested: boolean
  success: boolean
  message: string
  latency?: string
  content?: TestResponseContent
}

export interface TestResponseContent {
  type?: 'text' | 'image' | 'json' | 'raw' | string
  text?: string
  image_url?: string
  raw?: string
}

export interface TestModelResponse {
  success: boolean
  message: string
  channel_model: string
  model: string
  latency?: string
  non_stream: TestModeResult
  stream: TestModeResult
}
