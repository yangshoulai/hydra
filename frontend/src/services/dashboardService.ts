import { apiClient } from './api'
import { clearAuthTokens, getAccessToken, redirectToLogin, refreshAuthSession } from '@/services/authSession'

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || ''

export type DashboardQPSRange = '1h' | '6h' | '24h'

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
  status: 'active' | 'inactive'
  failed_requests: number
  success_rate: number
  success_requests: number
  total_requests: number
  prompt_tokens: number
  completion_tokens: number
  healthy_keys: number
  total_keys: number
  health_percentage: number
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
  qps: number
  current_qps: number
  qps_trend: QPSDataPoint[]
  today_success_rate: SuccessRateStats
  overall_health: OverallHealthStatus
  channel_stats: ChannelHealthInfo[]
  channel_health_list: ChannelHealthInfo[]
  model_stats: ModelStats
  total_requests: number
  total_channels: number
  total_keys: number
  active_channels: number
  total_prompt_tokens: number
  total_completion_tokens: number
  generated_at: string
  success_rate: number
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

export interface CircuitSnapshot {
  kind: 'key' | 'model'
  id: number
  channel_id: number
  state: 'cooling' | 'inactive'
  failure_count: number
  last_failure: string
  cooling_ends_at: string
  remaining_sec: number
}

export interface DashboardFrame {
  metrics: DashboardMetrics
  circuits: CircuitSnapshot[]
}

function resolveURL(path: string): string {
  if (/^https?:\/\//i.test(path)) return path
  return `${API_BASE_URL}${path}`
}

function parseSSEBlock<T>(rawBlock: string): { event: string; data: T } | null {
  const lines = rawBlock.split('\n')
  let eventName = 'message'
  const dataLines: string[] = []

  for (const rawLine of lines) {
    const line = rawLine.trimEnd()
    if (!line || line.startsWith(':')) continue
    if (line.startsWith('event:')) {
      eventName = line.slice('event:'.length).trim() || 'message'
      continue
    }
    if (line.startsWith('data:')) {
      dataLines.push(line.slice('data:'.length).trimStart())
    }
  }

  if (!dataLines.length) {
    return null
  }

  const payload = JSON.parse(dataLines.join('\n')) as T
  return { event: eventName, data: payload }
}

class DashboardService {
  async getMetrics(qpsRange: DashboardQPSRange = '1h'): Promise<DashboardMetrics> {
    const response = await apiClient.get('/admin/api/dashboard/metrics', {
      params: { qps_range: qpsRange },
    })
    return response.data.data
  }

  async getQPSMetrics(qpsRange: DashboardQPSRange = '1h'): Promise<QPSMetrics> {
    const response = await apiClient.get('/admin/api/dashboard/metrics/qps', {
      params: { qps_range: qpsRange },
    })
    return response.data.data
  }

  async getSuccessRateMetrics(): Promise<SuccessRateMetrics> {
    const response = await apiClient.get('/admin/api/dashboard/metrics/success-rate')
    return response.data.data
  }

  async getChannelHealthMetrics(): Promise<ChannelHealthMetrics> {
    const response = await apiClient.get('/admin/api/dashboard/metrics/channel-health')
    return response.data.data
  }

  async getCircuitStatus(): Promise<CircuitSnapshot[]> {
    const response = await apiClient.get('/admin/api/dashboard/circuits')
    return response.data.data || []
  }

  async clearCircuit(kind: CircuitSnapshot['kind'], id: number): Promise<void> {
    await apiClient.post('/admin/api/dashboard/circuits/clear', {
      kind,
      id,
    })
  }

  openMetricsStream(
    onMessage: (frame: DashboardFrame) => void,
    qpsRange: DashboardQPSRange = '1h',
  ): { close: () => void } {
    const controller = new AbortController()
    const url = resolveURL(`/admin/api/dashboard/metrics/stream?qps_range=${encodeURIComponent(qpsRange)}`)
    const accessToken = getAccessToken()

    const headers: Record<string, string> = {
      Accept: 'text/event-stream',
      'Cache-Control': 'no-cache',
    }
    if (accessToken) {
      headers.Authorization = `Bearer ${accessToken}`
    }

    const start = async () => {
      try {
        const response = await fetch(url, {
          method: 'GET',
          headers,
          signal: controller.signal,
        })

        if (response.status === 401) {
          try {
            await refreshAuthSession()
          } catch {
            clearAuthTokens()
            redirectToLogin()
          }
          throw new Error('stream request failed: 401')
        }

        if (!response.ok || !response.body) {
          throw new Error(`stream request failed: ${response.status}`)
        }

        const reader = response.body.getReader()
        const decoder = new TextDecoder()
        let buffer = ''

        while (true) {
          const { done, value } = await reader.read()
          if (done) break

          buffer += decoder.decode(value, { stream: true }).replace(/\r\n/g, '\n')
          let splitIndex = buffer.indexOf('\n\n')
          while (splitIndex >= 0) {
            const block = buffer.slice(0, splitIndex)
            buffer = buffer.slice(splitIndex + 2)

            try {
              const parsed = parseSSEBlock<DashboardFrame>(block)
              if (parsed && parsed.event === 'metrics') {
                onMessage(parsed.data)
              }
            } catch (error) {
              console.error('解析仪表盘 SSE 帧失败', error)
            }

            splitIndex = buffer.indexOf('\n\n')
          }
        }
      } catch (error) {
        if (!(error instanceof Error && error.name === 'AbortError')) {
          console.error('仪表盘 SSE 连接异常', error)
        }
      }
    }

    void start()

    return {
      close: () => controller.abort(),
    }
  }
}

export const dashboardService = new DashboardService()
