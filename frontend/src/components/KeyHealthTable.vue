<template>
  <div class="key-health-table">
    <n-data-table
        :columns="columns"
        :data="displayData"
        :loading="loading"
        :pagination="pagination"
        :row-key="(row: KeyHealthRow) => row.key_id"
        size="small"
        :bordered="true"
        :striped="true"
        :single-line="false"
    />
  </div>
</template>

<script setup lang="ts">
import {computed, h, onMounted, ref} from 'vue'
import {type DataTableColumns, NButton, NDataTable, NIcon, NSpace, NTag, NText} from 'naive-ui'
import {AlertCircle, CheckmarkCircle, CloseCircle, ContrastOutline, CopyOutline, PlayCircleOutline, TrashOutline} from '@vicons/ionicons5'
import {channelApi} from '../services/channelService'
import type {Channel, ChannelHealthCheckResult, Key} from '../types/channel'

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
  status: 'active' | 'cooling' | 'disabled' | 'dead'
  test_status?: 'healthy' | 'unhealthy' | 'error' | 'testing'
  test_message?: string
  message: string
  latency: string
  cooling_time?: string
  key?: Key
}

// State
const loading = ref(false)
const channel = ref<Channel | null>(null)
const testingKeys = ref<Set<number>>(new Set())
const testResults = ref<Map<number, { status: 'healthy' | 'unhealthy' | 'error', message: string, latency: string }>>(new Map())
const currentTime = ref(Date.now())

// 脱敏key函数
function maskKey(key: string): string {
  if (!key || key.length < 10) return key || ''
  return key.substring(0, 6) + '**********' + key.substring(key.length - 4)
}

// 计算冷却时间
function calculateCoolingTime(coolingAt?: string): string | undefined {
  if (!coolingAt) return undefined

  const coolingTime = new Date(coolingAt).getTime()
  const diff = currentTime.value - coolingTime

  if (diff < 0) return undefined

  const seconds = Math.floor(diff / 1000)
  const minutes = Math.floor(seconds / 60)
  const remainingSeconds = seconds % 60

  if (minutes > 0) {
    return `${minutes}分${remainingSeconds}秒`
  } else {
    return `${remainingSeconds}秒`
  }
}

// 计算显示数据
const displayData = computed<KeyHealthRow[]>(() => {
  return channel.value?.keys?.map((key) => {
    const testResult = testResults.value.get(key.id)
    return {
      key_id: key.id,
      key_remark: key.remark,
      key_value: key.key_value,
      key_preview: key.key_preview || maskKey(key.key_value),
      status: key.status,
      test_status: testingKeys.value.has(key.id) ? 'testing' : testResult?.status,
      test_message: testResult?.message,
      message: '',
      latency: testResult?.latency || '',
      cooling_time: key.status === 'cooling' ? calculateCoolingTime(key.cooling_at) : undefined,
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
    title: '密钥',
    key: 'key_preview',
    width: 200,
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
    title: '状态',
    key: 'status',
    align: 'center',
    width: 80,
    render(row) {
      const config = {
        active: {type: 'success' as const, text: '正常', icon: CheckmarkCircle},
        cooling: {type: 'warning' as const, text: '冷却中', icon: AlertCircle},
        disabled: {type: 'error' as const, text: '禁用', icon: CloseCircle},
        dead: {type: 'error' as const, text: '失效', icon: CloseCircle}
      }
      const status = config[row.status] || config.disabled
      return h(
          'div',
          {style: {display: 'flex', alignItems: 'center', gap: '8px', 'justifyContent': 'center'}},
          [
            h(NTag, {type: status.type, size: 'small'}, {default: () => status.text})
          ]
      )
    }
  },
  {
    title: '已冷却',
    key: 'cooling_time',
    align: 'center',
    width: 120,
    render(row) {
      if (row.status !== 'cooling' || !row.cooling_time) {
        return h(NText, {depth: 3}, {default: () => '-'})
      }
      return h(NText, {}, {default: () => row.cooling_time})
    }
  },
  {
    title: '备注',
    key: 'key_remark',
    width: 120,
    ellipsis: {
      tooltip: true
    }
  },
  {
    title: '测试状态',
    key: 'test_status',
    align: 'center',
    width: 120,
    render(row) {
      // 如果正在测试
      if (testingKeys.value.has(row.key_id)) {
        return h(
            NButton,
            {
              size: 'tiny',
              type: 'info',
              loading: true,
              disabled: true
            },
            {
              default: () => '测试中',
              icon: () => h(NIcon, {}, {default: () => h(PlayCircleOutline)})
            }
        )
      }

      // 有测试结果时显示为按钮
      if (row.test_status) {
        const config = {
          healthy: {type: 'success' as const, text: '健康'},
          unhealthy: {type: 'error' as const, text: '异常'},
          error: {type: 'warning' as const, text: '错误'},
          testing: {type: 'info' as const, text: '测试中'}
        }
        const status = config[row.test_status]

        return h(
            NButton,
            {
              size: 'tiny',
              type: status.type,
              title: row.test_message || '',
              onClick: () => handleTestKey(row.key_id)
            },
            {
              default: () => status.text,
              icon: () => h(NIcon, {}, {default: () => h(PlayCircleOutline)})
            }
        )
      }

      // 没有测试结果时显示测试按钮
      return h(
          NButton,
          {
            size: 'tiny',
            type: 'info',
            onClick: () => handleTestKey(row.key_id)
          },
          {
            default: () => '测试',
            icon: () => h(NIcon, {}, {default: () => h(PlayCircleOutline)})
          }
      )
    }
  },
  {
    title: '延迟',
    key: 'latency',
    width: 80,
    align: 'right',
    render(row) {
      if (!row.latency || row.latency === '-') {
        return h(NText, {depth: 3}, {default: () => '-'})
      }
      // 解析延迟字符串，保留最多2位小数
      const match = row.latency.match(/^([\d.]+)(.*)$/)
      if (match) {
        const value = parseFloat(match[1] || '')
        const unit = match[2]
        return h(NText, {}, {default: () => `${value.toFixed(2)}${unit}`})
      }
      return h(NText, {}, {default: () => row.latency})
    }
  },
  {
    title: '操作',
    key: 'actions',
    align: 'center',
    width: 160,
    fixed: 'right',
    render(row) {
      return h(
          NSpace,
          {
            justify: 'center'
          },
          {
            default: () => [
              row.key && row.key.status !== 'disabled' && h(
                  NButton,
                  {
                    size: 'tiny',
                    type: 'warning',
                    onClick: () => handleToggleKeyStatus(row.key_id, 'disabled')
                  },
                  {
                    default: () => '禁用',
                    icon: () => h(NIcon, null, {default: () => h(ContrastOutline)})
                  }
              ),
              row.key && row.key.status === 'disabled' && h(
                  NButton,
                  {
                    size: 'tiny',
                    type: 'success',
                    onClick: () => handleToggleKeyStatus(row.key_id, 'active')
                  },
                  {
                    default: () => '启用',
                    icon: () => h(NIcon, null, {default: () => h(ContrastOutline)})
                  }
              ),
              row.key && h(
                  NButton,
                  {
                    size: 'tiny',
                    type: 'error',
                    onClick: () => handleDeleteKey(row.key_id)
                  },
                  {
                    default: () => '删除',
                    icon: () => h(NIcon, null, {default: () => h(TrashOutline)})
                  }
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
  testingKeys.value.add(keyId)

  try {
    const result = await channelApi.testSingleKey(keyId)

    // 保存测试结果
    testResults.value.set(keyId, {
      status: result.status,
      message: result.message,
      latency: result.latency
    })

    if (result.status === 'healthy') {
      window.$message?.success(`测试成功: ${result.message}`)
    } else {
      window.$message?.error(`测试失败: ${result.message}`)
    }
    await fetchChannel()
  } catch (error: any) {
    console.error('Failed to test key:', error)
    window.$message?.error(error.response?.data?.error || '测试失败')

    // 保存错误结果
    testResults.value.set(keyId, {
      status: 'error',
      message: error.response?.data?.error || '测试失败',
      latency: '-'
    })
  } finally {
    testingKeys.value.delete(keyId)
  }
}

// 切换密钥状态（启用/禁用）
async function handleToggleKeyStatus(keyId: number, targetStatus: 'active' | 'disabled') {
  const action = targetStatus === 'disabled' ? '禁用' : '启用'

  window.$dialog?.warning({
    title: `确认${action}`,
    content: `确定要${action}此密钥吗？`,
    positiveText: '确定',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await channelApi.resetKeyStatus(keyId, targetStatus)
        window.$message?.success(`${action}成功`)
        await fetchChannel()
        emit('refresh')
      } catch (error: any) {
        console.error(`Failed to ${action} key:`, error)
        window.$message?.error(error.response?.data?.error || `${action}失败`)
      }
    }
  })
}

// 刷新列表
async function refresh() {
  await fetchChannel()
}

// 设置批量测试中状态
function setTestingAll() {
  testingKeys.value.clear()
  displayData.value.forEach(row => {
    testingKeys.value.add(row.key_id)
  })
}

// 更新批量测试结果
function updateTestResults(results: ChannelHealthCheckResult) {
  testingKeys.value.clear()
  results.key_results.forEach(result => {
    testResults.value.set(result.key_id, {
      status: result.status,
      message: result.message,
      latency: result.latency
    })
  })
}

// 暴露刷新方法给父组件
defineExpose({
  refresh,
  setTestingAll,
  updateTestResults
})

// 初始化
onMounted(() => {
  fetchChannel()

  // 每秒更新一次当前时间，用于实时更新冷却时间
  setInterval(() => {
    currentTime.value = Date.now()
  }, 1000)
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
