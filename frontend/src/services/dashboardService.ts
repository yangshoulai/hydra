import axios from 'axios'

const BASE_URL = '/admin/api'

// Dashboard types
export interface QPSDataPoint {
  timestamp: string
  qps: number
}

export interface SuccessRateStats {
  total_requests: number
  success_requests: number
  failed_requests: number
  success_rate: number
}

export interface ChannelHealthInfo {
  channel_id: number
  channel_name: string
  status: string
  priority: number
  weight: number
  total_keys: number
  healthy_keys: number
  unhealthy_keys: number
  health_percentage: number
  last_request_time?: string
  success_rate: number
  total_requests: number
  error_distribution?: Record<string, number>
}

export interface OverallHealthStatus {
  total_channels: number
  overall_health: number
  total_keys: number
  healthy_keys: number
  unhealthy_keys: number
}

export interface ModelStats {
  active_models: number
  total_requests: number
  success_requests: number
  failed_requests: number
  model_list: ModelDetailInfo[]
}

export interface ModelDetailInfo {
  model_name: string
  total_requests: number
  success_requests: number
  failed_requests: number
  success_rate: number
}

export interface DashboardMetrics {
  current_qps: number
  qps_time_series: QPSDataPoint[]
  today_success_rate: SuccessRateStats
  overall_health: OverallHealthStatus
  channel_health_list: ChannelHealthInfo[]
  model_stats: ModelStats
  total_requests_today: number
  total_channels: number
  total_keys: number
  active_channels: number
  generated_at: string
}

export interface QPSMetrics {
  current_qps: number
  qps_time_series: QPSDataPoint[]
}

export interface SuccessRateMetrics {
  today_success_rate: SuccessRateStats
  channel_success_rates?: Record<number, SuccessRateStats>
}

export interface ChannelHealthMetrics {
  overall_health: OverallHealthStatus
  channel_health_list: ChannelHealthInfo[]
}

// Dashboard API client
class DashboardService {
  // 获取完整的仪表盘指标
  async getMetrics(): Promise<DashboardMetrics> {
    const response = await axios.get(`${BASE_URL}/dashboard/metrics`)
    return response.data.data
  }

  // 获取 QPS 指标
  async getQPSMetrics(): Promise<QPSMetrics> {
    const response = await axios.get(`${BASE_URL}/dashboard/metrics/qps`)
    return response.data.data
  }

  // 获取成功率指标
  async getSuccessRateMetrics(): Promise<SuccessRateMetrics> {
    const response = await axios.get(`${BASE_URL}/dashboard/metrics/success-rate`)
    return response.data.data
  }

  // 获取渠道健康指标
  async getChannelHealthMetrics(): Promise<ChannelHealthMetrics> {
    const response = await axios.get(`${BASE_URL}/dashboard/metrics/channel-health`)
    return response.data.data
  }
}

export default new DashboardService()
