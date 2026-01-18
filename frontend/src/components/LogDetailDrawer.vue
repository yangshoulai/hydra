<template>
  <n-drawer v-model:show="visible" :width="1200" placement="right" class="log-detail-drawer">
    <n-drawer-content :title="title" closable>
      <template v-if="log">
        <n-space vertical :size="24">
          <!-- 基本信息卡片 -->
          <n-card title="基本信息" size="small" :bordered="false" class="detail-section">
            <n-descriptions bordered :column="3" size="small">
              <n-descriptions-item label="日志ID" :span="1">
                <n-text strong>#{{ log.id }}</n-text>
              </n-descriptions-item>
              <n-descriptions-item label="Trace ID" :span="2">
                <n-space align="center">
                  <n-text code style="font-size: 13px">{{ log.trace_id }}</n-text>
                  <TraceIdCopy :trace-id="log.trace_id"/>
                </n-space>
              </n-descriptions-item>

              <n-descriptions-item label="创建时间" :span="3">
                <n-space align="center">
                  <n-icon>
                    <TimeIcon/>
                  </n-icon>
                  <n-text>{{ formatTime(log.created_at) }}</n-text>
                </n-space>
              </n-descriptions-item>
            </n-descriptions>
          </n-card>

          <!-- 请求信息卡片 -->
          <n-card title="请求信息" size="small" :bordered="false" class="detail-section">
            <n-descriptions bordered :column="2" size="small">
              <n-descriptions-item label="请求方法">
                <n-tag :type="getMethodType(log.request_method)" size="medium" :bordered="false">
                  {{ log.request_method }}
                </n-tag>
              </n-descriptions-item>

              <n-descriptions-item label="请求路径">
                <n-text code style="font-size: 12px">{{ log.request_path }}</n-text>
              </n-descriptions-item>

              <n-descriptions-item label="访问令牌" :span="2">
                <n-space align="center">
                  <n-icon size="16" color="#10b981">
                    <KeyIcon/>
                  </n-icon>
                  <n-text code style="font-size: 13px">{{ log.access_token }}</n-text>
                </n-space>
              </n-descriptions-item>

              <n-descriptions-item v-if="log.request_headers" label="请求头" :span="2">
                <template #default>
                  <div class="code-block-wrapper">
                    <n-button
                        size="tiny"
                        quaternary
                        circle
                        class="copy-button"
                        @click="copyToClipboard(formatHeaders(log.request_headers), '请求头')"
                    >
                      <template #icon>
                        <n-icon><CopyIcon/></n-icon>
                      </template>
                    </n-button>
                    <n-code :code="formatHeaders(log.request_headers)" language="json" :word-wrap="true" :hljs="hljs"/>
                  </div>
                </template>
              </n-descriptions-item>

              <n-descriptions-item v-if="log.request_body" label="请求体" :span="2">
                <template #default>
                  <div class="code-block-wrapper">
                    <n-button
                        size="tiny"
                        quaternary
                        circle
                        class="copy-button"
                        @click="copyToClipboard(formatJSON(log.request_body), '请求体')"
                    >
                      <template #icon>
                        <n-icon><CopyIcon/></n-icon>
                      </template>
                    </n-button>
                    <n-code :code="formatJSON(log.request_body)" language="json" :word-wrap="true" :hljs="hljs"/>
                  </div>
                </template>
              </n-descriptions-item>
            </n-descriptions>
          </n-card>

          <!-- 模型与渠道卡片 -->
          <n-card title="模型与渠道" size="small" :bordered="false" class="detail-section">
            <n-descriptions bordered :column="3" size="small">
              <n-descriptions-item label="统一模型">
                <n-tag v-if="log.unified_model" type="info" size="small">
                  {{ log.unified_model }}
                </n-tag>
                <n-text v-else depth="3">-</n-text>
              </n-descriptions-item>

              <n-descriptions-item label="上游模型">
                <n-tag v-if="log.upstream_model" type="success" size="small">
                  {{ log.upstream_model }}
                </n-tag>
                <n-text v-else depth="3">-</n-text>
              </n-descriptions-item>

              <n-descriptions-item label="渠道">
                <n-tag v-if="log.channel_name" type="warning" size="small">
                  {{ log.channel_name }}
                </n-tag>
                <n-text v-else depth="3">-</n-text>
              </n-descriptions-item>

              <n-descriptions-item label="Key ID">
                <n-text v-if="log.key_id" strong>{{ log.key_id }}</n-text>
                <n-text v-else depth="3">-</n-text>
              </n-descriptions-item>

              <n-descriptions-item label="重试次数">
                <n-tag v-if="log.retry_count > 0" type="warning" size="small" :bordered="false">
                  {{ log.retry_count }}
                </n-tag>
                <n-text v-else depth="3">{{ log.retry_count }}</n-text>
              </n-descriptions-item>

              <n-descriptions-item label="流式">
                <n-tag :type="log.is_stream ? 'info' : 'default'" size="small" :bordered="false">
                  {{ log.is_stream ? '是' : '否' }}
                </n-tag>
              </n-descriptions-item>
            </n-descriptions>
          </n-card>

          <!-- 响应信息卡片 -->
          <n-card title="响应信息" size="small" :bordered="false" class="detail-section">
            <n-descriptions bordered :column="3" size="small">
              <n-descriptions-item label="状态码">
                <n-tag :type="getStatusCodeType(log.status_code)" size="medium" :bordered="false">
                  {{ log.status_code }}
                </n-tag>
              </n-descriptions-item>

              <n-descriptions-item label="请求状态">
                <n-tag :type="log.is_success ? 'success' : 'error'" size="medium" :bordered="false">
                  {{ log.is_success ? '成功' : '失败' }}
                </n-tag>
              </n-descriptions-item>

              <n-descriptions-item label="响应时间">
                <n-tag
                    :type="getResponseType(log.response_time)"
                    size="medium"
                    :bordered="false"
                >
                  {{ log.response_time }} ms
                </n-tag>
              </n-descriptions-item>

              <n-descriptions-item v-if="log.error_message" label="错误信息" :span="3">
                <n-alert type="error" :show-icon="true">
                  {{ log.error_message }}
                </n-alert>
              </n-descriptions-item>

              <n-descriptions-item v-if="log.response_headers" label="响应头" :span="3">
                <template #default>
                  <div class="code-block-wrapper">
                    <n-button
                        size="tiny"
                        quaternary
                        circle
                        class="copy-button"
                        @click="copyToClipboard(formatHeaders(log.response_headers), '响应头')"
                    >
                      <template #icon>
                        <n-icon><CopyIcon/></n-icon>
                      </template>
                    </n-button>
                    <n-code :code="formatHeaders(log.response_headers)" language="json" :word-wrap="true" :hljs="hljs"/>
                  </div>
                </template>
              </n-descriptions-item>

              <n-descriptions-item v-if="log.response_body" label="响应体" :span="3">
                <template #default>
                  <div class="code-block-wrapper">
                    <n-button
                        size="tiny"
                        quaternary
                        circle
                        class="copy-button"
                        @click="copyToClipboard(getFormattedResponseBody(), '响应体')"
                    >
                      <template #icon>
                        <n-icon><CopyIcon/></n-icon>
                      </template>
                    </n-button>
                    <n-code
                        :code="getFormattedResponseBody()"
                        :language="getResponseLanguage()"
                        :word-wrap="true"
                        :hljs="hljs"
                    />
                  </div>
                </template>
              </n-descriptions-item>
            </n-descriptions>
          </n-card>

          <!-- 调用过程卡片 -->
          <n-card title="调用过程" size="small" :bordered="false" class="detail-section">
            <n-data-table
                :columns="timelineColumns"
                :data="timelineData"
                :pagination="false"
                :bordered="true"
                size="small"
                :row-key="(row: RequestLog) => row.id"
                :row-props="getRowProps"
            />
          </n-card>

          <!-- 客户端信息卡片 -->
          <n-card title="客户端信息" size="small" :bordered="false" class="detail-section">
            <n-descriptions bordered :column="2" size="small">
              <n-descriptions-item label="客户端IP">
                <n-space align="center">
                  <n-icon size="16" color="#60a5fa">
                    <GlobeIcon/>
                  </n-icon>
                  <n-text code style="font-size: 13px">{{ log.client_ip || '-' }}</n-text>
                </n-space>
              </n-descriptions-item>

              <n-descriptions-item label="User Agent">
                <n-text depth="2" style="font-size: 12px; word-break: break-all">
                  {{ log.user_agent || '-' }}
                </n-text>
              </n-descriptions-item>
            </n-descriptions>
          </n-card>
        </n-space>
      </template>

      <n-empty v-else description="加载中..."/>
    </n-drawer-content>
  </n-drawer>
</template>

<script setup lang="ts">
import {computed, onMounted, ref, watch, h} from 'vue'
import {useMessage, type DataTableColumns} from 'naive-ui'
import {NAlert, NButton, NCard, NCode, NDataTable, NDescriptions, NDescriptionsItem, NDrawer, NDrawerContent, NEmpty, NIcon, NSpace, NTag, NText} from 'naive-ui'
import {CopyOutline, GlobeOutline, KeyOutline, TimeOutline} from '@vicons/ionicons5'
import type {RequestLog} from '../types/log'
import TraceIdCopy from './TraceIdCopy.vue'
import {logApi} from '../services/logService'
import hljs from 'highlight.js/lib/core'
import json from 'highlight.js/lib/languages/json'

// 注册 JSON 语言
hljs.registerLanguage('json', json)

const message = useMessage()

interface Props {
  log: RequestLog | null
  show: boolean
}

interface Emits {
  (e: 'update:show', value: boolean): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const TimeIcon = TimeOutline
const KeyIcon = KeyOutline
const GlobeIcon = GlobeOutline
const CopyIcon = CopyOutline

// 调用过程数据
const timelineData = ref<RequestLog[]>([])

// 控制显示状态
const visible = computed({
  get: () => props.show,
  set: (value) => emit('update:show', value)
})

// 标题
const title = computed(() => {
  return props.log ? `日志详情 - #${props.log.id}` : '日志详情'
})

// 调用过程表格列定义
const timelineColumns = computed<DataTableColumns<RequestLog>>(() => [
  {
    title: '时间',
    key: 'created_at',
    width: 160,
    render: (row) => formatTime(row.created_at)
  },
  {
    title: '渠道',
    key: 'channel_name',
    width: 160,
    render: (row) => row.channel_name || '-'
  },
  {
    title: '渠道模型',
    key: 'upstream_model',
    width: 240,
    render: (row) => row.upstream_model || '-'
  },
  {
    title: '状态',
    key: 'status',
    width: 80,
    align:'center',
    render: (row) => {
      const type = row.is_success ? 'success' : 'error'
      const label = row.is_success ? '成功' : '失败'
      return h(NTag, {type, size: 'small', bordered: false}, {default: () => label})
    }
  },
  {
    title: '状态码',
    key: 'status_code',
    width: 80,
    align:'center',
    render: (row) => {
      const type = getStatusCodeType(row.status_code)
      return h(NTag, {type, size: 'small', bordered: false}, {default: () => row.status_code})
    }
  },
  {
    title: '响应时间',
    key: 'response_time',
    width: 80,
    align:'right',
    render: (row) => {
      const seconds = (row.response_time / 1000).toFixed(2)
      const timeType = getResponseTimeStatusType(row.response_time)
      return h('div', {class: 'flex items-center gap-2 justify-end'}, [
        h(NTag, {type: timeType, size: 'small', bordered: false}, {default: () => `${seconds}s`})
      ])
    }
  }
])

// 获取响应时间状态类型（用于 timeline）
function getResponseTimeStatusType(time: number): 'success' | 'warning' | 'error' {
  const seconds = time / 1000
  if (seconds < 5) return 'success'
  if (seconds < 10) return 'warning'
  return 'error'
}

// 获取行属性，用于高亮当前日志
function getRowProps(row: RequestLog) {
  if (props.log && row.id === props.log.id) {
    return {
      style: 'background-color: #e6f7ff; font-weight: 500;'
    }
  }
  return {}
}

// 加载调用过程数据
async function loadTimelineData() {
  if (!props.log?.trace_id) {
    timelineData.value = []
    return
  }

  try {
    const data = await logApi.getTimelineByTraceID(props.log.trace_id)
    timelineData.value = data
  } catch (error) {
    console.error('Failed to load timeline data:', error)
    message.error('加载调用过程失败')
  }
}

// 监听 log 变化，重新加载调用过程
watch(() => props.log, () => {
  loadTimelineData()
}, {immediate: true})

// 组件挂载时加载数据
onMounted(() => {
  loadTimelineData()
})

// 获取HTTP方法类型
function getMethodType(method: string) {
  const types: Record<string, any> = {
    GET: 'info',
    POST: 'success',
    PUT: 'warning',
    DELETE: 'error',
    PATCH: 'default'
  }
  return types[method] || 'default'
}

// 获取状态码类型
function getStatusCodeType(code: number) {
  if (code >= 200 && code < 300) return 'success'
  if (code >= 300 && code < 400) return 'info'
  if (code >= 400 && code < 500) return 'warning'
  if (code >= 500) return 'error'
  return 'default'
}

// 获取响应时间类型
function getResponseType(time: number) {
  if (time < 500) return 'success'
  if (time < 1000) return 'info'
  if (time < 3000) return 'warning'
  return 'error'
}

// 格式化时间
function formatTime(timeStr: string) {
  const date = new Date(timeStr)
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

// 格式化请求头/响应头
function formatHeaders(headersStr: string): string {
  try {
    const headers = JSON.parse(headersStr)
    return JSON.stringify(headers, null, 2)
  } catch {
    return headersStr
  }
}

// 格式化 JSON
function formatJSON(jsonStr: string): string {
  try {
    const obj = JSON.parse(jsonStr)
    return JSON.stringify(obj, null, 2)
  } catch {
    return jsonStr
  }
}

// 获取格式化后的响应体
function getFormattedResponseBody(): string {
  if (!props.log?.response_body) return ''
  // 如果是流式响应，直接返回原始内容（可能是 SSE 格式）
  if (props.log.is_stream) {
    return props.log.response_body
  }
  // 非流式响应尝试格式化 JSON
  return formatJSON(props.log.response_body)
}

// 获取响应体的语言类型
function getResponseLanguage(): string {
  if (!props.log?.response_body) return 'text'
  // 如果是流式响应，使用文本模式
  if (props.log.is_stream) {
    return 'text'
  }
  // 非流式响应尝试检测是否为 JSON
  try {
    JSON.parse(props.log.response_body)
    return 'json'
  } catch {
    return 'text'
  }
}

// 复制到剪贴板
function copyToClipboard(text: string, label: string) {
  navigator.clipboard.writeText(text).then(() => {
    message.success(`${label}已复制到剪贴板`)
  }).catch(() => {
    message.error('复制失败')
  })
}
</script>

<style scoped>
.log-detail-drawer :deep(.n-drawer-content) {
  padding: 0;
}

.log-detail-drawer :deep(.n-drawer-header__main) {
  padding: 20px 24px;
  border-bottom: 1px solid #e2e8f0;
}

.detail-section {
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  transition: all 0.2s ease;
}

.detail-section:hover {
  border-color: #cbd5e1;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
}

:deep(.n-card__header) {
  font-weight: 600;
  font-size: 14px;
  color: #1e293b;
  border-bottom: 1px solid #e2e8f0;
  padding: 12px 16px;
  background: #f1f5f9;
}

:deep(.n-card__content) {
  padding: 16px;
}

:deep(.n-descriptions) {
  font-size: 13px;
}

:deep(.n-descriptions-table-content__label) {
  font-weight: 500;
  color: #64748b;
  background: #f8fafc;
}

:deep(.n-descriptions-table-content__content) {
  background: #ffffff;
}

.code-block-wrapper {
  position: relative;
  width: 100%;
}

.copy-button {
  position: absolute;
  top: 8px;
  right: 8px;
  z-index: 10;
  opacity: 0;
  transition: opacity 0.2s;
}

.code-block-wrapper:hover .copy-button {
  opacity: 1;
}
</style>
