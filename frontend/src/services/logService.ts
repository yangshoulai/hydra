/**
 * 新版日志 API 服务（使用主表和明细表）
 */
import {apiClient} from './api'
import type {LogDetailResponse, LogQueryRequest, LogQueryResponse, LogStatistics} from '../types/log'

export const logApi = {
    /**
     * 查询日志列表
     */
    async list(params: LogQueryRequest): Promise<LogQueryResponse> {
        const response = await apiClient.get('/admin/api/logs', {params})
        return response.data.data
    },

    /**
     * 根据 TraceID 获取日志详情（包含所有重试记录）
     */
    async getByTraceId(traceId: string): Promise<LogDetailResponse> {
        const response = await apiClient.get(`/admin/api/logs/${traceId}`)
        return response.data.data
    },

    /**
     * 获取统计信息
     */
    async getStatistics(params?: { start_time?: string; end_time?: string }): Promise<LogStatistics> {
        const response = await apiClient.get('/admin/api/logs/statistics', {params})
        return response.data.data
    }
}
