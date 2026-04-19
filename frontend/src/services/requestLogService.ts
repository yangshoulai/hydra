import { apiClient } from './api'

export interface RequestLog {
  id: number
  created_at: string
  trace_id: string
  client_ip: string
  access_token_id: number
  access_token_name: string
  method: string
  path: string
  endpoint_type: string
  model: string
  is_stream: boolean
  status_code: number
  success: boolean
  duration_ms: number
  route_attempts: number
  retry_count: number
  final_channel_id: number
  final_channel_name: string
  final_key_id: number
  final_model_config_id: number
  final_channel_model: string
  prompt_tokens: number
  completion_tokens: number
  failure_type: string
  failure_scope: string
  failure_stage: string
  error_message: string
}

export interface RequestLogDetail {
  trace_id: string
  created_at: string
  request_headers_json: string
  request_body: string
  request_body_size: number
  response_headers_json: string
  response_body: string
  response_body_size: number
}

export interface RequestLogAttempt {
  id: number
  created_at: string
  trace_id: string
  attempt_num: number
  channel_id: number
  channel_name: string
  channel_model: string
  key_id: number
  key_name: string
  key_masked: string
  upstream_url: string
  duration_ms: number
  upstream_status_code: number
  success: boolean
  failure_type: string
  failure_scope: string
  failure_stage: string
  error_message: string
  upstream_request_headers_json: string
  upstream_request_body: string
  upstream_request_body_size: number
  upstream_response_headers_json: string
  upstream_response_body: string
  upstream_response_body_size: number
}

export interface RequestLogFull {
  log: RequestLog
  detail: RequestLogDetail | null
  attempts: RequestLogAttempt[]
}

export interface RequestLogListParams {
  page?: number
  page_size?: number
  start_at_ms?: number
  end_at_ms?: number
  model?: string
  channel_id?: number
  access_token_id?: number
  endpoint_type?: string
  status?: 'success' | 'failed' | ''
  has_retry?: 'true' | 'false' | ''
  trace_id?: string
  sort_by?: 'created_at' | 'duration_ms'
  sort_order?: 'asc' | 'desc'
}

export interface RequestLogListResponse {
  total: number
  page: number
  page_size: number
  items: RequestLog[]
}

class RequestLogService {
  async list(params?: RequestLogListParams): Promise<RequestLogListResponse> {
    const res = await apiClient.get<RequestLogListResponse>('/admin/api/request-logs', { params })
    return res.data
  }

  async get(traceID: string): Promise<RequestLogFull> {
    const res = await apiClient.get<RequestLogFull>(`/admin/api/request-logs/${traceID}`)
    return res.data
  }
}

export const requestLogService = new RequestLogService()
