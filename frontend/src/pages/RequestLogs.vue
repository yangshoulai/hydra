<template>
  <div class="app-page">
    <n-alert v-if="listError" type="error" :bordered="false">
      <n-space align="center" justify="space-between" style="width: 100%">
        <span>{{ listError }}</span>
        <n-button text type="error" @click="loadLogs">重试</n-button>
      </n-space>
    </n-alert>

    <section class="panel-card">
      <header class="panel-card__header table-toolbar">
        <div class="table-toolbar__title">
          <h3 class="panel-card__title">请求日志</h3>
          <span class="panel-card__subtitle">记录每一次代理请求；开启调试模式后会保留请求/响应明细</span>
        </div>
        <div class="table-toolbar__actions">
          <n-button size="small" @click="handleReset">重置</n-button>
          <n-button size="small" type="primary" @click="handleSearch">刷新</n-button>
        </div>
      </header>

      <div class="panel-card__body">
        <div class="table-inline-search">
          <n-date-picker
            v-model:value="filters.range"
            type="datetimerange"
            clearable
            size="small"
            style="width: 340px"
            :time-picker-props="{ format: 'HH:mm:ss' }"
            @update:value="handleSearch"
          />
          <n-select
            v-model:value="filters.status"
            class="table-filter-select"
            size="small"
            clearable
            placeholder="状态"
            :options="statusOptions"
            @update:value="handleSearch"
          />
          <n-select
            v-model:value="filters.hasRetry"
            class="table-filter-select"
            size="small"
            clearable
            placeholder="是否有重试"
            :options="retryOptions"
            @update:value="handleSearch"
          />
          <n-select
            v-model:value="filters.model"
            class="table-filter-select"
            size="small"
            clearable
            filterable
            placeholder="模型"
            :options="modelOptions"
            :loading="loadingModels"
            @update:value="handleSearch"
          />
          <n-select
            v-model:value="filters.channelID"
            class="table-filter-select"
            size="small"
            clearable
            filterable
            placeholder="渠道"
            :options="channelOptions"
            :loading="loadingChannels"
            @update:value="handleSearch"
          />
          <n-input
            v-model:value="filters.traceID"
            class="table-filter-input"
            size="small"
            clearable
            placeholder="Trace ID 前缀"
            @keyup.enter="handleSearch"
          >
            <template #prefix>
              <n-icon>
                <SearchOutline />
              </n-icon>
            </template>
          </n-input>
          <n-button size="small" type="primary" @click="handleSearch">查询</n-button>
        </div>

        <n-data-table
          :columns="columns"
          :data="logs"
          :loading="isLoading"
          :locale="tableLocale"
          :pagination="false"
          :single-line="false"
          :scroll-x="1540"
          :row-key="(row: RequestLog) => row.id"
          :row-props="rowProps"
        />

        <div style="display: flex; justify-content: flex-end; margin-top: 16px">
          <n-pagination
            :page="pagination.page"
            :on-update-page="pagination.onChange"
            :page-size="pagination.pageSize"
            :on-update:page-size="pagination.onUpdatePageSize"
            :item-count="pagination.total"
            :page-sizes="pagination.pageSizes"
            :show-size-picker="pagination.showSizePicker"
          />
        </div>
      </div>
    </section>

    <RequestLogDetailDrawer
      v-model:show="detailDrawerShow"
      :trace-id="selectedTraceID"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import {
  NAlert,
  NButton,
  NDataTable,
  NDatePicker,
  NIcon,
  NInput,
  NPagination,
  NSelect,
  NSpace,
  NTag,
  NTooltip,
  type DataTableColumns,
} from 'naive-ui'
import { SearchOutline } from '@vicons/ionicons5'
import { requestLogService, type RequestLog, type RequestLogListParams } from '@/services/requestLogService'
import { modelApi } from '@/services/modelService'
import { channelApi } from '@/services/channelService'
import RequestLogDetailDrawer from '@/components/RequestLogDetailDrawer.vue'
import { COL_WIDTH } from '@/constants/tableColumns'

const isLoading = ref(false)
const listError = ref('')
const logs = ref<RequestLog[]>([])

const detailDrawerShow = ref(false)
const selectedTraceID = ref('')

const statusOptions = [
  { label: '成功', value: 'success' },
  { label: '失败', value: 'failed' },
]
const retryOptions = [
  { label: '有重试', value: 'true' },
  { label: '无重试', value: 'false' },
]

const modelOptions = ref<Array<{ label: string; value: string }>>([])
const channelOptions = ref<Array<{ label: string; value: number }>>([])
const loadingModels = ref(false)
const loadingChannels = ref(false)

interface FilterState {
  range: [number, number] | null
  status: 'success' | 'failed' | null
  hasRetry: 'true' | 'false' | null
  model: string | null
  channelID: number | null
  traceID: string
}

const filters = reactive<FilterState>({
  range: null,
  status: null,
  hasRetry: null,
  model: null,
  channelID: null,
  traceID: '',
})

const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0,
  showSizePicker: true,
  pageSizes: [20, 50, 100],
  onChange: (page: number) => {
    pagination.page = page
    loadLogs()
  },
  onUpdatePageSize: (size: number) => {
    pagination.pageSize = size
    pagination.page = 1
    loadLogs()
  },
})

const tableLocale = computed(() => ({
  emptyText: isLoading.value ? '加载中...' : '暂无请求日志',
}))

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

function renderStatusCode(row: RequestLog) {
  const type = row.success ? 'success' : row.status_code === 499 ? 'warning' : 'error'
  const text = row.status_code || (row.success ? '200' : '—')
  return h(NTag, { type, bordered: false, size: 'small' }, { default: () => text })
}

function renderResultText(row: RequestLog) {
  if (row.success) return h('span', { class: 'status-text status-text--success' }, '成功')
  if (row.status_code === 499) return h('span', { class: 'status-text status-text--warning' }, '客户端取消')
  return h('span', { class: 'status-text status-text--error' }, '失败')
}

function renderTraceID(row: RequestLog) {
  const short = row.trace_id.slice(0, 8) + '…'
  return h(
    NTooltip,
    {},
    {
      trigger: () => h('span', { style: 'font-family: ui-monospace, monospace; font-size: 12px' }, short),
      default: () => row.trace_id,
    },
  )
}

function renderErrorIndicator(row: RequestLog) {
  if (row.success || !row.error_message) {
    return '—'
  }
  const preview = row.error_message.length > 60
    ? row.error_message.slice(0, 60) + '…'
    : row.error_message
  return h(
    NTooltip,
    { style: { maxWidth: '400px' } },
    {
      trigger: () => h('span', { class: 'error-cell' }, preview),
      default: () => row.error_message,
    },
  )
}

const columns = computed<DataTableColumns<RequestLog>>(() => [
  { title: '时间', key: 'created_at', width: COL_WIDTH.time, render: (row) => formatTime(row.created_at) },
  { title: 'Trace', key: 'trace_id', width: 110, render: renderTraceID },
  { title: '模型', key: 'model', width: COL_WIDTH.model, ellipsis: { tooltip: true } },
  {
    title: '渠道',
    key: 'final_channel_name',
    width: COL_WIDTH.channel,
    ellipsis: { tooltip: true },
    render: (row) => row.final_channel_name ? `${row.final_channel_name} · ${row.final_channel_model}` : '—',
  },
  { title: '状态', key: 'status_code', width: COL_WIDTH.status, render: renderStatusCode, align: "center" },
  { title: '结果', key: 'result', width: 80, render: renderResultText, align: "center" },
  {
    title: '耗时',
    key: 'duration_ms',
    width: COL_WIDTH.duration,
    render: (row) => formatDuration(row.duration_ms),
    sorter: 'default',
  },
  {
    title: '重试',
    key: 'retry_count',
    width: COL_WIDTH.retry,
    align: 'center',
    render: (row) => row.retry_count > 0
      ? h(NTag, { type: 'warning', bordered: false, size: 'small' }, { default: () => `↻ ${row.retry_count}` })
      : '—',
  },
  {
    title: 'Tokens',
    key: 'tokens',
    width: COL_WIDTH.tokens,
    render: (row) => `${row.prompt_tokens.toLocaleString('en-US')} / ${row.completion_tokens.toLocaleString('en-US')}`,
  },
  {
    title: '错误',
    key: 'error_message',
    ellipsis: { tooltip: true },
    render: renderErrorIndicator,
  },
])

function rowProps(row: RequestLog) {
  return {
    style: 'cursor: pointer',
    onClick: () => {
      selectedTraceID.value = row.trace_id
      detailDrawerShow.value = true
    },
  }
}

function buildQuery(): RequestLogListParams {
  const q: RequestLogListParams = {
    page: pagination.page,
    page_size: pagination.pageSize,
    sort_by: 'created_at',
    sort_order: 'desc',
  }
  if (filters.range) {
    q.start_at_ms = filters.range[0]
    q.end_at_ms = filters.range[1]
  }
  if (filters.status) q.status = filters.status
  if (filters.hasRetry) q.has_retry = filters.hasRetry
  if (filters.model) q.model = filters.model
  if (filters.channelID) q.channel_id = filters.channelID
  if (filters.traceID.trim()) q.trace_id = filters.traceID.trim()
  return q
}

async function loadLogs() {
  isLoading.value = true
  listError.value = ''
  try {
    const res = await requestLogService.list(buildQuery())
    logs.value = res.items
    pagination.total = res.total
  } catch (err) {
    listError.value = err instanceof Error ? err.message : '加载请求日志失败'
  } finally {
    isLoading.value = false
  }
}

async function loadFilterOptions() {
  loadingModels.value = true
  loadingChannels.value = true
  try {
    const [modelsRes, channelsRes] = await Promise.all([
      modelApi.list({ page: 1, page_size: 1000 }).catch(() => null),
      channelApi.list({ page: 1, page_size: 1000 }).catch(() => null),
    ])
    if (modelsRes && Array.isArray(modelsRes.items)) {
      modelOptions.value = modelsRes.items.map((m) => ({
        label: m.name,
        value: m.name,
      }))
    }
    if (channelsRes && Array.isArray(channelsRes.items)) {
      channelOptions.value = channelsRes.items.map((c) => ({
        label: c.name,
        value: c.id,
      }))
    }
  } finally {
    loadingModels.value = false
    loadingChannels.value = false
  }
}

function handleSearch() {
  pagination.page = 1
  loadLogs()
}

function handleReset() {
  filters.range = null
  filters.status = null
  filters.hasRetry = null
  filters.model = null
  filters.channelID = null
  filters.traceID = ''
  pagination.page = 1
  loadLogs()
}

onMounted(() => {
  loadFilterOptions()
  loadLogs()
})
</script>

<style scoped>
.panel-card__subtitle {
  display: block;
  margin-top: 2px;
  font-size: 12px;
  color: var(--n-text-color-3, #888);
}

.error-cell {
  color: var(--n-color-error, #d03050);
}

.status-text {
  font-size: 13px;
}
.status-text--success { color: var(--n-color-success, #18a058); }
.status-text--warning { color: var(--n-color-warning, #f0a020); }
.status-text--error   { color: var(--n-color-error, #d03050); }

.table-inline-search {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
  align-items: center;
}

.table-filter-input {
  width: 180px;
}

.table-filter-select {
  width: 150px;
}
</style>
