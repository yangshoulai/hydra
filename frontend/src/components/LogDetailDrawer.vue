<template>
  <n-drawer v-model:show="show" :width="1000" placement="right">
    <n-drawer-content :title="`日志详情 - ${log?.trace_id || ''}`" closable>
      <n-spin :show="loading">
        <n-space vertical :size="20" v-if="log">
          <!-- 统计卡片 -->
          <n-grid :cols="5" :x-gap="12" responsive="screen">
            <n-gi>
              <n-statistic label="总耗时" :value="(log.duration / 1000).toFixed(2) + 's'">
                <template #prefix>
                  <n-icon color="#60a5fa">
                    <TimeIcon/>
                  </n-icon>
                </template>
              </n-statistic>
            </n-gi>
            <n-gi>
              <n-statistic label="重试次数" :value="log.retry_count">
                <template #prefix>
                  <n-icon :color="log.retry_count > 0 ? '#f59e0b' : '#10b981'">
                    <RefreshIcon/>
                  </n-icon>
                </template>
              </n-statistic>
            </n-gi>
            <n-gi>
              <n-statistic label="请求大小" :value="formatBytes(totalRequestSize)">
                <template #prefix>
                  <n-icon color="#8b5cf6">
                    <UploadIcon/>
                  </n-icon>
                </template>
              </n-statistic>
            </n-gi>
            <n-gi>
              <n-statistic label="响应大小" :value="formatBytes(totalResponseSize)">
                <template #prefix>
                  <n-icon color="#10b981">
                    <DownloadIcon/>
                  </n-icon>
                </template>
              </n-statistic>
            </n-gi>
            <n-gi>
              <n-statistic label="状态">
                <template #prefix>
                  <n-icon color="#10b981">
                    <GlobeOutline/>
                  </n-icon>
                </template>

                <template #default>
                  <n-text :type="log.is_success ? 'success' : 'error'">{{ log.is_success ? '成功' : '失败' }}</n-text>
                </template>
              </n-statistic>
            </n-gi>
          </n-grid>

          <!-- 请求概览表格 -->
          <n-card title="请求概览" size="small" :bordered="false">
            <n-descriptions :column="2" label-placement="left" bordered size="small" :label-style="{width: '100px'}">
              <n-descriptions-item label="Trace ID" :span="2">
                <n-text>
                  <n-text code>
                    <span>{{ log.trace_id }}</span>
                  </n-text>
                  <n-button text size="tiny" @click="copyToClipboard(log.trace_id)">
                    <template #icon>
                      <n-icon>
                        <CopyIcon/>
                      </n-icon>
                    </template>
                  </n-button>
                </n-text>
              </n-descriptions-item>

              <n-descriptions-item label="请求模型">
                <n-tag type="info" size="small">{{ log.requested_model }}</n-tag>
              </n-descriptions-item>

              <n-descriptions-item label="重试次数">
                <n-tag :type="log.retry_count === 0 ? 'success' : (log.retry_count === 1 ?'warning' : 'error')" size="small">{{ log.retry_count }}</n-tag>
              </n-descriptions-item>

              <n-descriptions-item label="最后渠道">
                <n-text type="primary" size="small">{{ log.last_channel_name }}</n-text>
              </n-descriptions-item>

              <n-descriptions-item label="最后模型">
                <n-text type="info" size="small">{{ log.last_model }}</n-text>
              </n-descriptions-item>

              <n-descriptions-item label="端点类型">
                <EndpointTags :types="[log.endpoint_type]"/>
              </n-descriptions-item>

              <n-descriptions-item label="请求路径">
                <n-text type="info" size="small">{{ log.request_path }}</n-text>
              </n-descriptions-item>
              <n-descriptions-item label="请求方法">
                <n-text :type="getMethodTagType(log.request_method)" size="small">
                  {{ log.request_method }}
                </n-text>
              </n-descriptions-item>
              <n-descriptions-item label="状态码">
                <n-text :type="getStatusCodeType(log.status_code)" size="small">
                  {{ log.status_code }}
                </n-text>
              </n-descriptions-item>
              <n-descriptions-item label="开始时间">
                <n-space align="center">
                  <n-text>{{ formatTime(log.start_time) }}</n-text>
                </n-space>
              </n-descriptions-item>
              <n-descriptions-item label="结束时间">
                <n-text>{{ formatTime(log.end_time) }}</n-text>
              </n-descriptions-item>

              <n-descriptions-item label="耗时">
                <n-text :type="log.duration <= 5000 ? 'success': (log.duration <= 10000 ? 'warning' : 'error')">{{ (log.duration / 1000).toFixed(2) }} 秒
                </n-text>
              </n-descriptions-item>
              <n-descriptions-item label="流式">
                <n-text :type="log.is_stream ? 'info' : 'default'" size="small">
                  {{ log.is_stream ? '是' : '否' }}
                </n-text>
              </n-descriptions-item>
              <n-descriptions-item label="客户端 IP" :span="2">{{ log.client_ip }}</n-descriptions-item>
              <n-descriptions-item label="User Agent" :span="2">
                <n-text style="font-size: 12px" type="default">{{ log.user_agent }}</n-text>
              </n-descriptions-item>
              <n-descriptions-item label="错误" :span="2" v-if="log.error_message">
                <n-text style="font-size: 12px;" type="error">{{ log.error_message }}</n-text>
              </n-descriptions-item>
            </n-descriptions>
          </n-card>

          <!-- Timeline 展示重试记录 -->
          <n-card title="请求流程" size="small" :bordered="false">
            <n-timeline>
              <n-timeline-item
                  v-for="(detail, _) in log.details"
                  :key="detail.id"
                  :type="detail.is_success ? 'success' : 'error'"
                  :title="`${detail.retry_index === 0 ? '首次尝试' : `第 ${detail.retry_index} 次重试`}`"
                  :time="formatTime(detail.start_time)"
              >
                <template #icon>
                  <n-icon :color="detail.is_success ? '#18a058' : '#d03050'">
                    <component :is="detail.is_success ? CheckmarkCircleIcon : CloseCircleIcon"/>
                  </n-icon>
                </template>

                <n-space vertical :size="16">
                  <!-- 渠道和模型信息 -->
                  <n-space align="center" :size="8">
                    <n-tag type="info" size="small" :bordered="false">
                      <template #icon>
                        <n-icon>
                          <ServerIcon/>
                        </n-icon>
                      </template>
                      {{ detail.channel_name }}
                    </n-tag>
                    <n-tag type="primary" size="small" :bordered="false">
                      <template #icon>
                        <n-icon>
                          <CubeIcon/>
                        </n-icon>
                      </template>
                      {{ detail.model }}
                    </n-tag>
                    <n-tag :type="detail.is_success ? 'success' : 'error'" size="small" :bordered="false">
                      <template #icon>
                        <n-icon>
                          <GlobeOutline/>
                        </n-icon>
                      </template>
                      {{ detail.status_code }}
                    </n-tag>
                    <n-divider vertical/>
                    <n-text depth="3">
                      <n-icon>
                        <TimeIcon/>
                      </n-icon>
                      {{ detail.duration }} ms
                    </n-text>
                    <n-text depth="3">
                      <n-icon>
                        <UploadIcon/>
                      </n-icon>
                      {{ formatBytes(detail.request_body_size) }}
                    </n-text>
                    <n-text depth="3">
                      <n-icon>
                        <DownloadIcon/>
                      </n-icon>
                      {{ formatBytes(detail.response_body_size) }}
                    </n-text>
                  </n-space>

                  <!-- 错误信息 -->
                  <n-alert v-if="detail.error_message" type="error" :bordered="false" size="small">
                    {{ detail.error_message }}
                  </n-alert>

                  <!-- 详细信息折叠面板 -->
                  <n-collapse v-if="detail.request_headers || detail.request_body || detail.response_headers || detail.response_body">
                    <n-collapse-item v-if="detail.request_headers" name="request_headers">
                      <template #header>
                        <n-space align="center">
                          <n-text>
                            <n-icon color="#60a5fa">
                              <DocumentTextIcon/>
                            </n-icon>
                            <span>请求头</span>
                          </n-text>
                        </n-space>
                      </template>
                      <div class="code-block-wrapper">
                        <n-button
                            size="tiny"
                            text
                            class="copy-button"
                            @click="copyToClipboard(formatHeaders(detail.request_headers))">
                          <template #icon>
                            <n-icon>
                              <CopyIcon/>
                            </n-icon>
                          </template>
                        </n-button>
                        <n-code
                            :code="formatHeaders(detail.request_headers)"
                            language="json"
                            :show-line-numbers="true"
                            :word-wrap="true"
                            :hljs="hljs"
                        />
                      </div>
                    </n-collapse-item>

                    <n-collapse-item v-if="detail.request_body" name="request_body">
                      <template #header>
                        <n-space align="center">
                          <n-text>
                            <n-icon color="#10b981">
                              <UploadIcon/>
                            </n-icon>
                            <span>请求体</span>
                            <n-tag size="tiny" type="info" :bordered="false">{{ formatBytes(detail.request_body_size) }}</n-tag>
                          </n-text>
                        </n-space>
                      </template>
                      <div class="code-block-wrapper">
                        <n-button
                            size="tiny"
                            text
                            class="copy-button"
                            @click="copyToClipboard(formatJSON(detail.request_body))"
                        >
                          <template #icon>
                            <n-icon>
                              <CopyIcon/>
                            </n-icon>
                          </template>
                        </n-button>
                        <n-code
                            :code="formatJSON(detail.request_body)"
                            language="json"
                            :word-wrap="true"
                            :hljs="hljs"
                        />
                      </div>
                    </n-collapse-item>

                    <n-collapse-item v-if="detail.response_headers" name="response_headers">
                      <template #header>
                        <n-space align="center">
                          <n-text>
                            <n-icon color="#f59e0b">
                              <DocumentTextIcon/>
                            </n-icon>
                            <span>响应头</span>
                          </n-text>
                        </n-space>
                      </template>
                      <div class="code-block-wrapper">
                        <n-button
                            size="tiny"
                            text
                            class="copy-button"
                            @click="copyToClipboard(formatHeaders(detail.response_headers))"
                        >
                          <template #icon>
                            <n-icon>
                              <CopyIcon/>
                            </n-icon>
                          </template>
                        </n-button>
                        <n-code
                            :code="formatHeaders(detail.response_headers)"
                            language="json"
                            :word-wrap="true"
                            :hljs="hljs"
                        />
                      </div>
                    </n-collapse-item>

                    <n-collapse-item v-if="detail.response_body" name="response_body">
                      <template #header>
                        <n-space align="center">
                          <n-text>
                            <n-icon color="#8b5cf6">
                              <DownloadIcon/>
                            </n-icon>
                            <span>响应体</span>
                            <n-tag size="tiny" type="info" :bordered="false">{{ formatBytes(detail.response_body_size) }}</n-tag>
                          </n-text>
                        </n-space>
                      </template>
                      <div class="code-block-wrapper">
                        <n-button
                            size="tiny"
                            text
                            class="copy-button"
                            @click="copyToClipboard(formatJSON(detail.response_body))"
                        >
                          <template #icon>
                            <n-icon>
                              <CopyIcon/>
                            </n-icon>
                          </template>
                        </n-button>
                        <n-code
                            :code="formatJSON(detail.response_body)"
                            language="json"
                            :word-wrap="true"
                            :hljs="hljs"
                        />
                      </div>
                    </n-collapse-item>
                  </n-collapse>
                </n-space>
              </n-timeline-item>
            </n-timeline>
          </n-card>
        </n-space>
      </n-spin>
    </n-drawer-content>
  </n-drawer>
</template>

<script setup lang="ts">
import {computed, ref, watch} from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NCode,
  NCollapse,
  NCollapseItem,
  NDescriptions,
  NDescriptionsItem,
  NDivider,
  NDrawer,
  NDrawerContent,
  NGi,
  NGrid,
  NIcon,
  NSpace,
  NSpin,
  NStatistic,
  NTag,
  NText,
  NTimeline,
  NTimelineItem
} from 'naive-ui'
import {
  CheckmarkCircle as CheckmarkCircleIcon,
  CloseCircle as CloseCircleIcon,
  CloudDownloadOutline as DownloadIcon,
  CloudUploadOutline as UploadIcon,
  Copy as CopyIcon,
  CubeOutline as CubeIcon,
  DocumentTextOutline as DocumentTextIcon,
  GlobeOutline,
  RefreshOutline as RefreshIcon,
  ServerOutline as ServerIcon,
  TimeOutline as TimeIcon,
} from '@vicons/ionicons5'
import {logApi} from '../services/logService'
import type {LogDetailResponse} from '../types/log'
import EndpointTags from './EndpointTags.vue'

// highlight.js 配置
import hljs from 'highlight.js/lib/core'
import javascript from 'highlight.js/lib/languages/javascript'
import json from 'highlight.js/lib/languages/json'
import sql from 'highlight.js/lib/languages/sql'
import xml from 'highlight.js/lib/languages/xml'
import 'highlight.js/styles/atom-one-dark.css'

// 注册语言
hljs.registerLanguage('javascript', javascript)
hljs.registerLanguage('json', json)
hljs.registerLanguage('sql', sql)
hljs.registerLanguage('xml', xml)

interface Props {
  traceId?: string
  modelValue: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const show = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit('update:modelValue', value)
})

const loading = ref(false)
const log = ref<LogDetailResponse | null>(null)

// 计算总请求大小
const totalRequestSize = computed(() => {
  return log.value?.details.reduce((sum, d) => sum + d.request_body_size, 0) || 0
})

// 计算总响应大小
const totalResponseSize = computed(() => {
  return log.value?.details.reduce((sum, d) => sum + d.response_body_size, 0) || 0
})

// 格式化时间
function formatTime(time: string) {
  return new Date(time).toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false
  })
}

// 格式化字节大小
function formatBytes(bytes: number) {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return (bytes / Math.pow(k, i)).toFixed(2) + ' ' + sizes[i]
}

// 格式化 JSON
function formatJSON(json: string) {
  try {
    const parsed = JSON.parse(json)
    return JSON.stringify(parsed, null, 2)
  } catch {
    return json
  }
}

// 格式化请求头
function formatHeaders(headers: string) {
  try {
    const parsed = JSON.parse(headers)
    const formatted: Record<string, string> = {}
    for (const [key, value] of Object.entries(parsed)) {
      if (typeof value === 'object') {
        formatted[key] = JSON.stringify(value)
      } else {
        formatted[key] = String(value)
      }
    }
    return JSON.stringify(formatted, null, 2)
  } catch {
    return headers
  }
}

// 复制到剪贴板
async function copyToClipboard(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    window.$message?.success('已复制到剪贴板')
  } catch {
    window.$message?.error('复制失败')
  }
}

// 获取请求方法标签类型
function getMethodTagType(method: string): "default" | "info" | "warning" | "error" | "success" | "primary" {
  const types: Record<string, "default" | "info" | "warning" | "error" | "success" | "primary"> = {
    'GET': 'info',
    'POST': 'success',
    'PUT': 'warning',
    'DELETE': 'error',
    'PATCH': 'default'
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

// 加载日志详情
async function loadLogDetail() {
  if (!props.traceId) return

  loading.value = true
  try {
    log.value = await logApi.getByTraceId(props.traceId)
  } catch (error: any) {
    console.error('Failed to load log detail:', error)
    window.$message?.error('加载日志详情失败')
  } finally {
    loading.value = false
  }
}

// 监听 traceId 变化
watch(() => props.traceId, () => {
  if (props.traceId && props.modelValue) {
    loadLogDetail()
  }
}, {immediate: true})

// 监听抽屉打开
watch(() => props.modelValue, (newVal) => {
  if (newVal && props.traceId) {
    loadLogDetail()
  }
})
</script>

<style scoped>
:deep(.n-timeline-item-content) {
  padding-bottom: 24px;
}

:deep(.n-statistic .n-statistic-value) {
  font-size: 20px;
  font-weight: 600;
}

:deep(.n-statistic .n-statistic-label) {
  font-size: 12px;
  color: #64748b;
}

:deep(.n-collapse-item__header) {
  font-weight: 500;
  display: flex;
  align-items: center;
}

:deep(.n-code) {
  font-size: 12px;
  border-radius: 6px;
}

:deep(.n-descriptions) {
  font-size: 13px;
}

:deep(.n-descriptions .n-descriptions-th) {
  font-weight: 500;
  background: #f8fafc;
}

:deep(.n-card) {
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
  border-radius: 8px;
}

:deep(.n-card .n-card__header) {
  font-weight: 600;
  font-size: 14px;
}

:deep(.n-text) {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

/* 代码块包装器样式 */
.code-block-wrapper {
  position: relative;
  background: #ececec;
  border-radius: 8px;
  padding: 16px;
  overflow: hidden;
  margin-left: 24px;
}

/* 复制按钮样式 */
.copy-button {
  position: absolute;
  top: 8px;
  right: 8px;
  z-index: 10;
  opacity: 0.8;
}

.copy-button:hover {
  opacity: 1;
}

/* 代码块样式 */
:deep(.code-block-wrapper .n-code) {
  background: transparent !important;
  padding: 0 !important;
}

:deep(.code-block-wrapper pre) {
  margin: 0 !important;
  background: transparent !important;
}
</style>
