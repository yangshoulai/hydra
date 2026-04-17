<template>
  <n-drawer
    :show="show"
    :width="drawerWidth"
    placement="right"
    @update:show="handleShowUpdate"
  >
    <n-drawer-content
      :native-scrollbar="false"
      closable
      :title="drawerTitle"
    >
      <div v-if="loading" class="drawer-state">
        <n-spin size="medium" />
      </div>
      <div v-else-if="loadError" class="drawer-state">
        <n-alert type="error" :bordered="false">{{ loadError }}</n-alert>
      </div>
      <template v-else-if="full">
        <section class="summary-card">
          <div class="summary-card__head">
            <n-tag :type="successType" :bordered="false">
              {{ statusText }}
            </n-tag>
            <span class="summary-card__title">{{ full.log.method }} {{ full.log.path }}</span>
          </div>
          <div class="summary-card__grid">
            <div class="kv-cell">
              <span class="kv-cell__label">时间</span>
              <span class="kv-cell__value">{{ formatTime(full.log.created_at) }}</span>
            </div>
            <div class="kv-cell">
              <span class="kv-cell__label">耗时</span>
              <span class="kv-cell__value">{{ formatDuration(full.log.duration_ms) }}</span>
            </div>
            <div class="kv-cell">
              <span class="kv-cell__label">状态码</span>
              <span class="kv-cell__value">{{ full.log.status_code || '—' }}</span>
            </div>
            <div class="kv-cell">
              <span class="kv-cell__label">类型</span>
              <span class="kv-cell__value">{{ full.log.is_stream ? '流式' : '非流式' }}</span>
            </div>
            <div class="kv-cell">
              <span class="kv-cell__label">模型</span>
              <span class="kv-cell__value">{{ full.log.model || '—' }}</span>
            </div>
            <div class="kv-cell">
              <span class="kv-cell__label">Tokens</span>
              <span class="kv-cell__value">{{ full.log.prompt_tokens.toLocaleString('en-US') }} / {{ full.log.completion_tokens.toLocaleString('en-US') }}</span>
            </div>
            <div class="kv-cell">
              <span class="kv-cell__label">令牌</span>
              <span class="kv-cell__value">{{ full.log.access_token_name || '—' }}</span>
            </div>
            <div class="kv-cell">
              <span class="kv-cell__label">客户端 IP</span>
              <span class="kv-cell__value">{{ full.log.client_ip || '—' }}</span>
            </div>
            <div class="kv-cell kv-cell--wide">
              <span class="kv-cell__label">最终渠道</span>
              <span class="kv-cell__value">
                {{ full.log.final_channel_name || '—' }}
                <span v-if="full.log.final_channel_model" class="muted"> · {{ full.log.final_channel_model }}</span>
                <span v-if="full.log.route_attempts" class="muted"> · 尝试 {{ full.log.route_attempts }} 次</span>
                <span v-if="full.log.retry_count > 0" class="muted"> · 重试 {{ full.log.retry_count }}</span>
              </span>
            </div>
            <div class="kv-cell kv-cell--wide">
              <span class="kv-cell__label">Trace</span>
              <n-text code class="kv-cell__value" style="word-break: break-all">{{ full.log.trace_id }}</n-text>
            </div>
          </div>
          <div v-if="full.log.error_message" class="summary-card__error">
            <span class="kv-cell__label">错误</span>
            <span>{{ full.log.error_message }}</span>
          </div>
          <div v-if="full.log.failure_type || full.log.failure_stage" class="summary-card__tags">
            <n-tag v-if="full.log.failure_stage" size="small" :bordered="false">
              stage: {{ full.log.failure_stage }}
            </n-tag>
            <n-tag v-if="full.log.failure_type" size="small" :bordered="false">
              type: {{ full.log.failure_type }}
            </n-tag>
            <n-tag v-if="full.log.failure_scope" size="small" :bordered="false">
              scope: {{ full.log.failure_scope }}
            </n-tag>
          </div>
        </section>

        <n-tabs type="line" animated>
          <n-tab-pane name="client" tab="客户端">
            <template v-if="full.detail">
              <BodySection
                title="请求"
                :method="full.log.method"
                :url="full.log.path"
                :headers-json="full.detail.request_headers_json"
                :body="full.detail.request_body"
                :body-size="full.detail.request_body_size"
                :show-curl="true"
                :filename-prefix="`${full.log.trace_id}_client_request`"
              />
              <BodySection
                title="响应"
                :headers-json="full.detail.response_headers_json"
                :body="full.detail.response_body"
                :body-size="full.detail.response_body_size"
                :filename-prefix="`${full.log.trace_id}_client_response`"
              />
            </template>
            <n-empty v-else description="未启用调试模式，无客户端请求/响应明细" />
          </n-tab-pane>

          <n-tab-pane name="upstream" :tab="`上游调用 (${full.attempts.length})`">
            <template v-if="full.attempts.length > 0">
              <div class="attempts">
                <div
                  v-for="a in full.attempts"
                  :key="a.id"
                  class="attempt-card"
                  :class="{ 'attempt-card--success': a.success, 'attempt-card--failed': !a.success }"
                >
                  <div class="attempt-card__head">
                    <span class="attempt-index">#{{ a.attempt_num }}</span>
                    <n-tag :type="a.success ? 'success' : 'error'" size="small" :bordered="false">
                      {{ a.upstream_status_code || (a.success ? 200 : '—') }}
                    </n-tag>
                    <span class="attempt-title">
                      {{ a.channel_name }}
                      <span v-if="a.channel_model" class="muted"> · {{ a.channel_model }}</span>
                    </span>
                    <span class="muted">
                      · 密钥 {{ keyDisplay(a) }} ({{ a.key_masked }})
                    </span>
                    <span class="muted">· {{ formatDuration(a.duration_ms) }}</span>
                    <span v-if="a.failure_stage" class="muted">· {{ a.failure_stage }}</span>
                    <span v-if="a === lastAttempt && a.success" class="final-marker">最终</span>
                  </div>
                  <div v-if="a.error_message" class="attempt-card__error">
                    {{ a.error_message }}
                  </div>
                  <BodySection
                    title="请求"
                    :method="full.log.method"
                    :url="a.upstream_url"
                    :headers-json="a.upstream_request_headers_json"
                    :body="a.upstream_request_body"
                    :body-size="a.upstream_request_body_size"
                    :show-curl="true"
                    :filename-prefix="`${full.log.trace_id}_upstream_${a.attempt_num}_request`"
                    :default-closed="true"
                  />
                  <BodySection
                    title="响应"
                    :headers-json="a.upstream_response_headers_json"
                    :body="a.upstream_response_body"
                    :body-size="a.upstream_response_body_size"
                    :filename-prefix="`${full.log.trace_id}_upstream_${a.attempt_num}_response`"
                    :default-closed="true"
                  />
                </div>
              </div>
            </template>
            <n-empty v-else description="未启用调试模式，无上游调用明细" />
          </n-tab-pane>
        </n-tabs>
      </template>
    </n-drawer-content>
  </n-drawer>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  NAlert,
  NDrawer,
  NDrawerContent,
  NEmpty,
  NSpin,
  NTabs,
  NTabPane,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import { requestLogService, type RequestLogAttempt, type RequestLogFull } from '@/services/requestLogService'
import BodySection from '@/components/BodySection.vue'

const props = defineProps<{
  show: boolean
  traceId: string
}>()

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
}>()

const message = useMessage()

const loading = ref(false)
const loadError = ref('')
const full = ref<RequestLogFull | null>(null)

const drawerWidth = computed(() => Math.min(960, window.innerWidth * 0.95))
const drawerTitle = computed(() => {
  if (!full.value) return '请求详情'
  return `请求详情 · ${full.value.log.trace_id.slice(0, 12)}…`
})

const lastAttempt = computed(() => {
  if (!full.value || full.value.attempts.length === 0) return null
  return full.value.attempts[full.value.attempts.length - 1]
})

const successType = computed<'success' | 'warning' | 'error'>(() => {
  if (!full.value) return 'warning'
  if (full.value.log.success) return 'success'
  if (full.value.log.status_code === 499) return 'warning'
  return 'error'
})

const statusText = computed(() => {
  if (!full.value) return '—'
  if (full.value.log.success) return '成功'
  if (full.value.log.status_code === 499) return '客户端取消'
  return '失败'
})

function handleShowUpdate(value: boolean) {
  emit('update:show', value)
}

function keyDisplay(a: RequestLogAttempt): string {
  if (a.key_name) return a.key_name
  if (a.key_id) return `#${a.key_id}`
  return '—'
}

function formatTime(s: string): string {
  if (!s) return '-'
  const d = new Date(s)
  if (isNaN(d.getTime())) return s
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms} ms`
  return `${(ms / 1000).toFixed(2)} s`
}

async function loadDetail(traceID: string) {
  if (!traceID) return
  loading.value = true
  loadError.value = ''
  full.value = null
  try {
    full.value = await requestLogService.get(traceID)
  } catch (err) {
    loadError.value = err instanceof Error ? err.message : '加载请求详情失败'
    message.error(loadError.value)
  } finally {
    loading.value = false
  }
}

watch(
  () => [props.show, props.traceId],
  ([show, id]) => {
    if (show && id) {
      loadDetail(id as string)
    }
  },
  { immediate: true },
)
</script>

<style scoped>
.drawer-state {
  display: flex;
  justify-content: center;
  padding: 40px 0;
}

.summary-card {
  padding: 16px 18px;
  margin-bottom: 20px;
  background: var(--n-color-target, rgba(0, 0, 0, 0.02));
  border: 1px solid var(--n-border-color, rgba(0, 0, 0, 0.06));
  border-radius: 8px;
}

.summary-card__head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 14px;
  padding-bottom: 12px;
  border-bottom: 1px dashed var(--n-border-color, rgba(0, 0, 0, 0.08));
}

.summary-card__title {
  font-size: 14px;
  font-weight: 500;
  font-family: ui-monospace, monospace;
}

.summary-card__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px 20px;
  font-size: 13px;
}

.kv-cell {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  min-width: 0;
}

.kv-cell--wide {
  grid-column: span 2;
}

.kv-cell__label {
  width: 72px;
  color: var(--n-text-color-3, #888);
  font-size: 12px;
  flex-shrink: 0;
}

.kv-cell__value {
  flex: 1;
  min-width: 0;
  word-break: break-all;
}

.summary-card__error {
  margin-top: 12px;
  padding: 8px 10px;
  background: rgba(208, 48, 80, 0.08);
  border-radius: 4px;
  font-size: 13px;
  color: var(--n-color-error, #d03050);
  display: flex;
  gap: 10px;
}

.summary-card__tags {
  margin-top: 12px;
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.muted {
  color: var(--n-text-color-3, #888);
}

.attempts {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.attempt-card {
  padding: 12px 14px;
  border: 1px solid var(--n-border-color, rgba(0, 0, 0, 0.08));
  border-radius: 8px;
  background: var(--n-color, #fff);
}

.attempt-card--success {
  border-left: 3px solid var(--n-color-success, #18a058);
}

.attempt-card--failed {
  border-left: 3px solid var(--n-color-error, #d03050);
}

.attempt-card__head {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  font-size: 13px;
  margin-bottom: 8px;
}

.attempt-index {
  font-weight: 600;
  font-family: ui-monospace, monospace;
}

.attempt-title {
  font-weight: 500;
}

.attempt-card__error {
  margin: 6px 0 10px;
  padding: 6px 10px;
  background: rgba(208, 48, 80, 0.08);
  border-radius: 4px;
  color: var(--n-color-error, #d03050);
  font-size: 12px;
}

.final-marker {
  display: inline-flex;
  align-items: center;
  padding: 1px 6px;
  margin-left: 4px;
  background: var(--n-color-success, #18a058);
  color: #fff;
  border-radius: 3px;
  font-size: 11px;
}
</style>
