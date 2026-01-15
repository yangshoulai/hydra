<template>
  <div class="key-health-table">
    <n-data-table
      :columns="columns"
      :data="displayData"
      :loading="loading"
      :pagination="pagination"
      :row-key="(row: KeyHealthRow) => row.key_id"
      size="small"
    />

<!--    &lt;!&ndash; 健康状态统计 &ndash;&gt;-->
<!--    <n-space v-if="healthResult" style="margin-top: 16px">-->
<!--      <n-statistic label="总Key数" :value="healthResult.total_keys" />-->
<!--      <n-statistic label="健康Key数">-->
<!--        <template #default>-->
<!--          <n-text type="success" strong>{{ healthResult.healthy_keys }}</n-text>-->
<!--        </template>-->
<!--      </n-statistic>-->
<!--      <n-statistic label="异常Key数">-->
<!--        <template #default>-->
<!--          <n-text type="error" strong>{{ healthResult.total_keys - healthResult.healthy_keys }}</n-text>-->
<!--        </template>-->
<!--      </n-statistic>-->
<!--    </n-space>-->
  </div>
</template>

<script setup lang="ts">
import { ref, computed, h, onMounted } from 'vue'
import {
  NDataTable,
  NSpace,
  NStatistic,
  NText,
  NTag,
  NButton,
  NIcon,
  NProgress,
  NSpin,
  type DataTableColumns
} from 'naive-ui'
import { RefreshOutline, CheckmarkCircle, CloseCircle, AlertCircle, CopyOutline, PulseOutline } from '@vicons/ionicons5'
import { channelApi } from '../services/channelService'
import type { ChannelHealthCheckResult, Channel, Key, SingleKeyHealthResult } from '../types/channel'

interface Props {
  channelId: number
  healthResult?: ChannelHealthCheckResult
}

const props = defineProps<Props>()

const emit = defineEmits<{
  refresh: []
}>()

interface KeyHealthRow {
  key_id: number
  key_remark: string
  key_value: string
  key_preview: string
  status: 'healthy' | 'unhealthy' | 'error' | 'testing'
  message: string
  latency: string
  key?: Key
}

// State
const loading = ref(false)
const channel = ref<Channel | null>(null)
const testingKeys = ref<Set<number>>(new Set())
const keyStatusMap = ref<Map<number, SingleKeyHealthResult>>(new Map())

// 脱敏key函数
function maskKey(key: string): string {
  if (!key || key.length < 10) return key || ''
  return key.substring(0, 6) + '**********' + key.substring(key.length - 4)
}

// 计算显示数据
const displayData = computed<KeyHealthRow[]>(() => {
  if (props.healthResult) {
    return props.healthResult.key_results.map(row => ({
      ...row,
      key_value: row.key_value || '',
      key_preview: row.key_value ? maskKey(row.key_value) : '***',
      // 如果有测试结果，使用测试结果的状态
      status: testingKeys.value.has(row.key_id)
        ? 'testing'
        : keyStatusMap.value.get(row.key_id)?.status || row.status,
      message: keyStatusMap.value.get(row.key_id)?.message || row.message,
      latency: keyStatusMap.value.get(row.key_id)?.latency || row.latency
    }))
  }
  return channel.value?.keys?.map((key) => {
    const testResult = keyStatusMap.value.get(key.id)
    return {
      key_id: key.id,
      key_remark: key.remark,
      key_value: key.key_value,
      key_preview: key.key_preview || maskKey(key.key_value),
      status: testingKeys.value.has(key.id)
        ? 'testing'
        : testResult?.status || (key.status === 'active' ? 'healthy' : 'unhealthy'),
      message: testResult?.message || (key.status === 'active' ? 'Key is active' : 'Key is not active'),
      latency: testResult?.latency || '-',
      key
    }
  }) || []
})

// 分页
const pagination = {
  pageSize: 10
}

// 表格列定义
const columns: DataTableColumns<KeyHealthRow> = [
  {
    title: 'ID',
    key: 'key_id',
    width: 80,
    align: 'right'
  },
  {
    title: 'Key',
    key: 'key_preview',
    width: 280,
    render(row) {
      return h(
        'div',
        {
          style: {
            display: 'flex',
            alignItems: 'center',
            gap: '8px'
          }
        },
        [
          h(
            NText,
            {
              code: true,
              style: {
                fontFamily: 'monospace',
                fontSize: '13px',
                color: '#4b5563'
              }
            },
            {
              default: () => row.key_preview
            }
          ),
          h(
            NIcon,
            {
              size: 16,
              style: {
                cursor: 'pointer',
                color: '#60a5fa'
              },
              onClick: () => handleCopyKey(row.key_value)
            },
            {
              default: () => h(CopyOutline)
            }
          )
        ]
      )
    }
  },
  {
    title: '备注',
    key: 'key_remark',
    width: 200,
    ellipsis: {
      tooltip: true
    }
  },
  {
    title: '状态',
    key: 'status',
    align: 'center',
    width: 160,
    render(row) {
      const config = {
        healthy: { type: 'success' as const, text: '健康', icon: CheckmarkCircle },
        unhealthy: { type: 'error' as const, text: '异常', icon: CloseCircle },
        error: { type: 'warning' as const, text: '错误', icon: AlertCircle },
        testing: { type: 'info' as const, text: '测试中', icon: RefreshOutline }
      }
      const status = config[row.status]
      return h(
        'div',
        { style: { display: 'flex', alignItems: 'center', gap: '8px', 'justifyContent': 'center' } },
        [
          h(NIcon, {
            color: status.type === 'success' ? '#18a058' :
                   status.type === 'error' ? '#d03050' :
                   status.type === 'warning' ? '#f0a020' :
                   '#2080f0',
            class: row.status === 'testing' ? 'spin-icon' : ''
          }, {
            default: () => h(status.icon)
          }),
          h(NTag, { type: status.type, size: 'small' }, { default: () => status.text })
        ]
      )
    }
  },
  {
    title: '延迟',
    key: 'latency',
    width: 160,
    align: 'right',
    render(row) {
      if (row.latency === '-') {
        return h(NText, { depth: 3 }, { default: () => '-' })
      }
      // 解析延迟字符串并显示
      return h(NText, {}, { default: () => row.latency })
    }
  },
  {
    title: '操作',
    key: 'actions',
    align: 'center',
    width: 240,
    fixed: 'right',
    render(row) {
      return h(
        NSpace,
        {
          justify: 'center'
        },
        {
          default: () => [
            h(
              NButton,
              {
                size: 'small',
                type: 'info',
                loading: testingKeys.value.has(row.key_id),
                disabled: testingKeys.value.has(row.key_id),
                onClick: () => handleTestKey(row.key_id)
              },
              {
                default: () => '测试',
                icon: () => h(NIcon, {}, { default: () => h(PulseOutline) })
              }
            ),
            row.key && h(
              NButton,
              {
                size: 'small',
                type: 'error',
                onClick: () => handleDeleteKey(row.key_id)
              },
              { default: () => '删除' }
            )
          ]
        }
      )
    }
  }
]

// 获取渠道详情
async function fetchChannel() {
  if (!props.channelId || props.healthResult) return

  loading.value = true
  try {
    channel.value = await channelApi.get(props.channelId)
  } catch (error: any) {
    console.error('Failed to fetch channel:', error)
  } finally {
    loading.value = false
  }
}

// 复制Key
async function handleCopyKey(keyValue: string) {
  if (!keyValue) return

  try {
    await navigator.clipboard.writeText(keyValue)
    window.$message?.success('Key已复制到剪贴板')
  } catch (error) {
    console.error('Failed to copy key:', error)
    window.$message?.error('复制失败，请手动复制')
  }
}

// 删除Key
async function handleDeleteKey(keyId: number) {
  window.$dialog?.warning({
    title: '确认删除',
    content: '确定要删除此Key吗？',
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await channelApi.deleteKey(keyId)
        window.$message?.success('删除成功')
        await fetchChannel()
        emit('refresh')
      } catch (error: any) {
        console.error('Failed to delete key:', error)
        window.$message?.error(error.response?.data?.error || '删除失败')
      }
    }
  })
}

// 测试单个Key
async function handleTestKey(keyId: number) {
  // 标记为测试中
  testingKeys.value.add(keyId)

  try {
    const result = await channelApi.testSingleKey(keyId)
    // 保存测试结果
    keyStatusMap.value.set(keyId, result)
    window.$message?.success('测试完成')
  } catch (error: any) {
    console.error('Failed to test key:', error)
    window.$message?.error(error.response?.data?.error || '测试失败')
    // 保存错误结果
    keyStatusMap.value.set(keyId, {
      key_id: keyId,
      key_remark: '',
      status: 'error',
      message: error.response?.data?.error || '测试失败',
      latency: '-'
    })
  } finally {
    // 移除测试中状态
    testingKeys.value.delete(keyId)
  }
}

// 刷新列表
async function refresh() {
  await fetchChannel()
}

// 设置 Key 为测试中状态
function setTesting(keyId: number) {
  testingKeys.value.add(keyId)
}

// 更新测试结果
function updateHealthResults(result: ChannelHealthCheckResult) {
  result.key_results.forEach(keyResult => {
    keyStatusMap.value.set(keyResult.key_id, keyResult)
  })
  // 清除所有测试中状态
  testingKeys.value.clear()
}

// 清除所有测试中状态
function clearTesting() {
  testingKeys.value.clear()
}

// 暴露刷新方法给父组件
defineExpose({
  refresh,
  setTesting,
  updateHealthResults,
  clearTesting
})

// 初始化
onMounted(() => {
  fetchChannel()
})
</script>

<style scoped>
.key-health-table {
  width: 100%;
}

.spin-icon {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}
</style>
