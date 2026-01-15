/**
 * 日志查询 API 客户端
 */

import axios from 'axios'
import type { LogQueryRequest, LogQueryResponse, LogStatistics, RequestLog } from '../types/log'

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080'

// 获取认证token
const getAuthToken = () => {
  return localStorage.getItem('access_token') || ''
}

// 创建axios实例
const apiClient = axios.create({
  baseURL: API_BASE_URL
})

// 请求拦截器 - 添加认证token
apiClient.interceptors.request.use((config) => {
  const token = getAuthToken()
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

export const logApi = {
  /**
   * 查询日志列表
   */
  async query(params: LogQueryRequest = {}): Promise<LogQueryResponse> {
    const response = await apiClient.get<LogQueryResponse>('/admin/api/logs', {
      params
    })
    return response.data
  },

  /**
   * 根据TraceID获取日志详情
   */
  async getByTraceID(traceId: string): Promise<RequestLog> {
    const response = await apiClient.get<RequestLog>(`/admin/api/logs/${traceId}`)
    return response.data
  },

  /**
   * 获取日志统计信息
   */
  async getStatistics(startTime?: string, endTime?: string): Promise<LogStatistics> {
    const params: any = {}
    if (startTime) params.start_time = startTime
    if (endTime) params.end_time = endTime

    const response = await apiClient.get<LogStatistics>('/admin/api/logs/statistics', {
      params
    })
    return response.data
  }
}
