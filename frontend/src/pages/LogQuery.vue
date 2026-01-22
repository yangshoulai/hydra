<template>
  <div class="space-y-6">
    <!-- 筛选器 -->
    <LogFilter ref="filterRef" @filter="handleFilter"/>

    <!-- 日志列表 -->
    <n-data-table
        :columns="columns"
        :data="logs"
        :loading="loading"
        :pagination="false"
        :row-key="(row: RequestLogMain) => row.id"
        :row-props="rowProps"
        :scroll-x="1840"
        :single-line="false"
        striped
    />
    <div class="flex justify-end">
      <n-pagination
          :page="pagination.page"
          :on-update-page="pagination.onChange"
          @update:page-size="pagination.onUpdatePageSize"
          :page-size="pagination.pageSize"
          :item-count="pagination.itemCount"
          :page-sizes="pagination.pageSizes"
          :show-size-picker="pagination.showSizePicker"
      />
    </div>
  </div>

  <!-- 日志详情抽屉（新版，带 Timeline） -->
  <LogDetailDrawer
      v-model="showDetailDrawer"
      :trace-id="selectedTraceId"
  />
</template>

<script setup lang="ts">
import {h, onMounted, reactive, ref} from 'vue'
import {type DataTableColumns, NDataTable, NPagination, NTag, NText, NTime} from 'naive-ui'
import {logApi} from '../services/logService'
import type {LogQueryRequest, RequestLogMain} from '../types/log'
import LogFilter from '../components/LogFilter.vue'
import LogDetailDrawer from '../components/LogDetailDrawer.vue'
import EndpointTags from '../components/EndpointTags.vue'

// State
const logs = ref<RequestLogMain[]>([])
const loading = ref(false)
const selectedTraceId = ref<string>('')
const showDetailDrawer = ref(false)

// 当前筛选条件
const currentFilters = ref<LogQueryRequest>({})

// 分页
const pagination = reactive({
  page: 1,
  pageSize: 20,
  itemCount: 1,
  showSizePicker: true,
  pageSizes: [10, 20, 50, 100],
  onChange: (page: number) => {
    pagination.page = page
    fetchLogs()
  },
  onUpdatePageSize: (pageSize: number) => {
    pagination.pageSize = pageSize
    pagination.page = 1
    fetchLogs()
  }
})

// 表格行属性（支持点击行查看详情）
const rowProps = (row: RequestLogMain) => {
  return {
    style: 'cursor: pointer;',
    onClick: () => {
      handleViewDetail(row)
    }
  }
}

// 表格列定义
const columns: DataTableColumns<RequestLogMain> = [
  {
    title: 'ID',
    key: 'id',
    width: 80,
    align: 'left',
    fixed: 'left',
    render(row) {
      return h(NText, {depth: 3}, {default: () => `#${row.id}`})
    }
  },
  {
    title: 'Trace ID',
    key: 'trace_id',
    width: 160,
    align: 'center',
    fixed: 'left',
    render(row) {
      return h(NText, {code: true}, {default: () => row.trace_id.substring(0, 13)})
    }
  },
  {
    title: '开始时间',
    key: 'start_time',
    width: 200,
    align: 'center',
    render(row) {
      return h(NTime, {time: new Date(row.start_time), format: 'yyyy-MM-dd HH:mm:ss'})
    }
  },

  {
    title: '状态',
    key: 'is_success',
    width: 80,
    align: 'center',
    render(row) {
      return h(NTag, {
        type: row.is_success ? 'success' : 'error',
        size: 'small'
      }, {
        default: () => row.is_success ? '成功' : '失败'
      })
    }
  },
  {
    title: '耗时',
    key: 'duration',
    width: 80,
    align: 'right',
    render(row) {
      const seconds = (row.duration / 1000).toFixed(2)
      const color = row.duration < 5000 ? 'success' : row.duration < 10000 ? 'warning' : 'error'
      return h(NTag, {type: color, size: 'small'}, {default: () => `${seconds}s`})
    }
  },
  {
    title: '重试',
    key: 'retry_count',
    width: 80,
    align: 'center',
    render(row) {
      const type = row.retry_count === 0 ? 'success' : (row.retry_count === 1 ? 'warning' : 'error')
      return h(NTag, {type: type, size: 'small'}, {default: () => row.retry_count})
    }
  },
  {
    title: '请求模型',
    key: 'requested_model',
    width: 200,
    ellipsis: {
      tooltip: true
    }
  },
  {
    title: '渠道',
    key: 'last_channel_name',
    width: 160,
    ellipsis: {
      tooltip: true
    },
    render(row) {
      return h(NText, {}, {default: () => row.last_channel_name || '-'})
    }
  },
  {
    title: '渠道模型',
    key: 'last_model',
    width: 200,
    ellipsis: {
      tooltip: true
    }
  },
  {
    title: '端点类型',
    key: 'endpoint_type',
    width: 200,
    align: 'center',
    ellipsis: {
      tooltip: true
    },
    render(row) {
      if (!row.endpoint_type) return h(NText, {depth: 3}, {default: () => '-'})
      return h(EndpointTags, {types: [row.endpoint_type]})
    }
  },
  {
    title: '流式',
    key: 'is_stream',
    width: 120,
    align: 'center',
    render(row) {
      return h(NTag, {
        type: row.is_stream ? 'info' : 'default',
        size: 'small'
      }, {
        default: () => row.is_stream ? '流式' : '非流式'
      })
    }
  },
  {
    title: '状态码',
    key: 'status_code',
    width: 80,
    align: 'center',
    render(row) {
      const type = row.status_code >= 200 && row.status_code < 300 ? 'success' : 'error'
      return h(NTag, {type, size: 'small'}, {default: () => row.status_code})
    }
  },
  {
    title: '结束时间',
    key: 'end_time',
    width: 200,
    align: 'center',
    render(row) {
      return h(NTime, {time: new Date(row.end_time), format: 'yyyy-MM-dd HH:mm:ss'})
    }
  },
]

// 获取日志列表
async function fetchLogs() {
  loading.value = true
  try {
    const params: LogQueryRequest = {
      page: pagination.page,
      page_size: pagination.pageSize,
      ...currentFilters.value
    }
    const result = await logApi.list(params)
    logs.value = result.items
    pagination.itemCount = result.total
  } catch (error: any) {
    console.error('Failed to fetch logs:', error)
    window.$message?.error(error.response?.data?.error || '获取日志失败')
  } finally {
    loading.value = false
  }
}

// 处理筛选条件变化
function handleFilter(filters: any) {
  currentFilters.value = {...filters}
  pagination.page = 1
  fetchLogs()
}

// 查看详情
function handleViewDetail(log: RequestLogMain) {
  selectedTraceId.value = log.trace_id
  showDetailDrawer.value = true
}

// 初始化
onMounted(() => {
  fetchLogs()
})
</script>

<style scoped>
</style>
