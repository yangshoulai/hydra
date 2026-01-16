<template>
  <div class="space-y-6 animate-fade-in">
    <!-- 筛选器 -->
    <LogFilter ref="filterRef" @filter="handleFilter"/>

    <!-- 日志列表 -->
    <n-data-table
        :columns="columns"
        :data="logs"
        :loading="loading"
        :pagination="pagination"
        :row-key="(row: RequestLog) => row.id"
        :row-props="rowProps"
        :scroll-x="1680"
        :single-line="false"
        striped
    />
  </div>

  <!-- 日志详情抽屉 -->
  <LogDetailDrawer
      v-model:show="showDetailDrawer"
      :log="selectedLog"
  />
</template>

<script setup lang="ts">
import {computed, h, onMounted, reactive, ref} from 'vue'
import {type DataTableColumns, type DataTableRowProps, NButton, NDataTable, NTag, NText, NTime} from 'naive-ui'
import {EyeOutline} from '@vicons/ionicons5'
import {logApi} from '../services/logService'
import type {LogQueryRequest, RequestLog} from '../types/log'
import LogFilter from '../components/LogFilter.vue'
import LogDetailDrawer from '../components/LogDetailDrawer.vue'

// State
const logs = ref<RequestLog[]>([])
const loading = ref(false)
const selectedLog = ref<RequestLog | null>(null)
const showDetailDrawer = ref(false)
const filterRef = ref()

// 当前筛选条件
const currentFilters = ref<LogQueryRequest>({})

// 是否有激活的筛选条件
const hasActiveFilters = computed(() => {
  return Object.values(currentFilters.value).some(
      (value) => value !== undefined && value !== '' && value !== null
  )
})

// 分页
const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0,
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
const rowProps = (row: RequestLog): DataTableRowProps => {
  return {
    style: 'cursor: pointer;',
    onClick: () => {
      handleViewDetail(row)
    }
  }
}

// 表格列定义
const columns: DataTableColumns<RequestLog> = [
  {
    title: 'ID',
    key: 'id',
    width: 120,
    align: 'left',
    render(row) {
      return h(NText, {depth: 3}, {default: () => `#${row.id}`})
    }
  },
  {
    title: 'Trace ID',
    key: 'trace_id',
    width: 160,
    align: 'center',
    render(row) {
      return h(NText, {code: true}, {default: () => row.trace_id.substring(0, 16)})
    }
  },
  {
    title: '请求时间',
    key: 'created_at',
    width: 200,
    align: 'center',
    render(row) {
      return h(NTime, {time: new Date(row.created_at), format: 'yyyy-MM-dd HH:mm:ss'})
    }
  },
  {
    title: '状态',
    key: 'is_success',
    width: 120,
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
    title: '响应时间（毫秒）',
    key: 'response_time',
    width: 160,
    align: 'right',
    render(row) {
      const color = row.response_time < 1000 ? 'success' : row.response_time < 3000 ? 'warning' : 'error'
      return h(NTag, {type: color, size: 'small'}, {default: () => `${row.response_time}`})
    }
  },
  {
    title: '方法',
    key: 'request_method',
    width: 120,
    align: 'center',
    render(row) {
      const types: Record<string, any> = {
        GET: 'info',
        POST: 'success',
        PUT: 'warning',
        DELETE: 'error'
      }
      return h(NTag, {type: types[row.request_method] || 'default', size: 'small'}, {
        default: () => row.request_method
      })
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
    title: '上游模型',
    key: 'upstream_model',
    width: 240,
    ellipsis: {
      tooltip: true
    },
    render(row) {
      return h(NText, {}, {default: () => row.upstream_model || '-'})
    }
  },
  {
    title: '渠道',
    key: 'channel_name',
    width: 160,
    ellipsis: {
      tooltip: true
    },
    render(row) {
      return h(NText, {}, {default: () => row.channel_name || '-'})
    }
  },
  {
    title: '状态码',
    key: 'status_code',
    width: 120,
    align: 'center',
    render(row) {
      const type = row.status_code >= 200 && row.status_code < 300 ? 'success' : 'error'
      return h(NTag, {type, size: 'small'}, {default: () => row.status_code})
    }
  },


  {
    title: '操作',
    key: 'actions',
    width: 80,
    align: 'center',
    fixed: 'right',
    render(row) {
      return h(
          NButton,
          {
            size: 'small',
            text: true,
            type: 'primary',
            onClick: (e: Event) => {
              e.stopPropagation()
              handleViewDetail(row)
            }
          },
          {
            icon: () => h(EyeOutline),
            default: () => '查看'
          }
      )
    }
  }
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

    const result = await logApi.query(params)
    logs.value = result.logs
    pagination.total = result.total
  } catch (error: any) {
    console.error('Failed to fetch logs:', error)
    window.$message?.error(error.response?.data?.error || '获取日志失败')
  } finally {
    loading.value = false
  }
}

// 处理筛选条件变化
function handleFilter(filters: LogQueryRequest) {
  currentFilters.value = {...filters}
  pagination.page = 1
  fetchLogs()
}

// 查看详情
function handleViewDetail(log: RequestLog) {
  selectedLog.value = log
  showDetailDrawer.value = true
}

// 初始化
onMounted(() => {
  fetchLogs()
})
</script>

<style scoped>


</style>
