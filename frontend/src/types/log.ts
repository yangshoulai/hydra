/**
 * 新版日志类型定义（主表和明细表）
 */

// 主日志记录
export interface RequestLogMain {
    id: number
    created_at: string
    updated_at: string
    trace_id: string

    // 请求基本信息
    endpoint_type: string
    request_path: string
    request_method: string
    requested_model: string
    unified_model: string

    // 客户端信息
    access_token: string
    client_ip: string
    user_agent: string

    // 时间信息
    start_time: string
    end_time: string
    duration: number // 总耗时（毫秒）

    // 最终结果
    is_success: boolean
    status_code: number
    retry_count: number
    is_stream: boolean

    // 最后成功/失败的渠道信息
    last_channel_id?: number
    last_channel_name?: string
    last_model?: string

    // 错误信息
    error_message?: string
}

// 日志明细记录
export interface RequestLogDetail {
    id: number
    created_at: string

    // 关联主表
    main_log_id: number

    // 渠道和模型信息
    channel_id?: number
    channel_name: string
    model: string

    // 密钥信息
    key_id?: number

    // 时间信息
    start_time: string
    end_time: string
    duration: number // 本次尝试耗时（毫秒）

    // 请求和响应信息
    request_body_size: number
    response_body_size: number

    // 状态信息
    status_code: number
    is_success: boolean
    status: string // success, failed, timeout, etc.
    retry_index: number // 第几次重试（0表示首次尝试）

    // 流式响应信息
    is_stream: boolean
    stream_chunks: number
    stream_first_chunk_time?: number

    // 详细信息
    request_headers?: string
    request_body?: string
    response_headers?: string
    response_body?: string

    // 错误信息
    error_message?: string
}

// 日志详情（主表 + 明细）
export interface LogDetailResponse extends RequestLogMain {
    details: RequestLogDetail[]
}

// 查询请求
export interface LogQueryRequest {
    page?: number
    page_size?: number
    trace_id?: string
    access_token?: string
    requested_model?: string
    status_code?: number
    is_success?: boolean | undefined
    endpoint_type?: string
    channel_id?: number
    start_time?: string
    end_time?: string
}

// 查询响应
export interface LogQueryResponse {
    total: number
    page: number
    page_size: number
    items: RequestLogMain[]
}

// 统计信息
export interface LogStatistics {
    total_requests: number
    success_requests: number
    failed_requests: number
    success_rate: number
    avg_response_time: number
    model_stats: ModelStat[]
    channel_stats: ChannelStat[]
}

// 模型统计
export interface ModelStat {
    model: string
    total: number
    success: number
}

// 渠道统计
export interface ChannelStat {
    channel_name: string
    total: number
    success: number
    avg_duration: number
}
