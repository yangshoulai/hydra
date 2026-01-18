/**
 * 日志查询 API 客户端
 */

import { apiClient } from './api'
import type { LogQueryRequest, LogQueryResponse, LogStatistics, RequestLog } from '../types/log'

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
   * 根据TraceID获取所有相关日志（包括重试记录）
   */
  async getTimelineByTraceID(traceId: string): Promise<RequestLog[]> {
    const response = await apiClient.get<RequestLog[]>(`/admin/api/logs/${traceId}/timeline`)
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
