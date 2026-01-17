<template>
  <n-drawer v-model:show="visible" :width="900" placement="right" class="log-detail-drawer">
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

              <n-descriptions-item v-if="log.request_body" label="请求体" :span="2">
                <n-code :code="log.request_body" language="json" :word-wrap="true"/>
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

              <n-descriptions-item v-if="log.response_body" label="响应体" :span="3">
                <n-code :code="log.response_body" language="json" :word-wrap="true"/>
              </n-descriptions-item>
            </n-descriptions>
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
import {computed} from 'vue'
import {NAlert, NCard, NCode, NDescriptions, NDescriptionsItem, NDrawer, NDrawerContent, NEmpty, NIcon, NSpace, NTag, NText} from 'naive-ui'
import {GlobeOutline, KeyOutline, TimeOutline} from '@vicons/ionicons5'
import type {RequestLog} from '../types/log'
import TraceIdCopy from './TraceIdCopy.vue'

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

// 控制显示状态
const visible = computed({
  get: () => props.show,
  set: (value) => emit('update:show', value)
})

// 标题
const title = computed(() => {
  return props.log ? `日志详情 - #${props.log.id}` : '日志详情'
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
</style>
