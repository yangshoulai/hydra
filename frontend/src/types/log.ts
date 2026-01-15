/**
 * 日志相关类型定义
 */

export interface RequestLog {
  id: number
  created_at: string
  trace_id: string
  access_token: string
  request_path: string
  request_method: string
  requested_model: string
  unified_model: string
  upstream_model: string
  channel_id?: number
  channel_name: string
  key_id?: number
  status_code: number
  response_time: number
  is_success: boolean
  error_message?: string
  retry_count: number
  is_stream: boolean
  stream_chunks: number
  client_ip: string
  user_agent: string
}

export interface LogQueryRequest {
  page?: number
  page_size?: number
  trace_id?: string
  access_token?: string
  requested_model?: string
  channel_id?: number
  status_code?: number
  is_success?: boolean
  start_time?: string
  end_time?: string
  order_by?: 'created_at' | 'response_time'
  order?: 'asc' | 'desc'
}

export interface LogQueryResponse {
  total: number
  page: number
  page_size: number
  logs: RequestLog[]
}

export interface LogStatistics {
  total_requests: number
  success_requests: number
  failed_requests: number
  success_rate: number
  avg_response_time: number
}
