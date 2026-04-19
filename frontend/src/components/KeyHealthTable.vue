<template>
  <div class="key-health-table">
    <n-data-table
      :columns="columns"
      :data="displayData"
      :loading="loading"
      :pagination="false"
      :row-key="(row: KeyHealthRow) => row.channel_key_id"
      size="small"
      :striped="true"
      :single-line="false"
    />
  </div>
</template>

<script setup lang="ts">
import {computed, h, onMounted, ref} from 'vue'
import {type DataTableColumns, NButton, NDataTable, NIcon, NSpace, NTag, NText, NTooltip} from 'naive-ui'
import {ContrastOutline, CopyOutline, TrashOutline} from '@vicons/ionicons5'
import {channelApi} from '../services/channelService'
import type {Channel, ChannelHealthCheckResult, ChannelKey} from '../types/channel'
import {toastApiError} from '@/utils/error'

interface Props {
  channelId: number
  healthResult?: ChannelHealthCheckResult
}

const props = defineProps<Props>()

const emit = defineEmits<{
  refresh: []
}>()

interface KeyHealthRow {
  channel_key_id: number
  channel_key_remark: string
  channel_key_value: string
  channel_key_preview: string
  channel_key_group: string
  status: 'active' | 'inactive'
  test_status?: 'healthy' | 'unhealthy' | 'error' | 'testing'
  test_message?: string
  message: string
  latency: string
  channelKey?: ChannelKey
}

const loading = ref(false)
const channel = ref<Channel | null>(null)
const testingKeys = ref<Set<number>>(new Set())
const testResults = ref<Map<number, { status: 'healthy' | 'unhealthy' | 'error', message: string, latency: string }>>(new Map())

function maskKey(key: string): string {
  if (!key || key.length < 10) return key || ''
  return key.substring(0, 6) + '**********' + key.substring(key.length - 4)
}

const displayData = computed<KeyHealthRow[]>(() => {
  const rows = channel.value?.channel_keys?.map((channelKey) => {
    const testResult = testResults.value.get(channelKey.id)
    const testStatus = (testingKeys.value.has(channelKey.id) ? 'testing' : testResult?.status) as KeyHealthRow['test_status']
    return {
      channel_key_id: channelKey.id,
      channel_key_remark: channelKey.remark,
      channel_key_value: channelKey.channel_key_value,
      channel_key_preview: channelKey.channel_key_preview || maskKey(channelKey.channel_key_value),
      channel_key_group: channelKey.channel_key_group || 'Default',
      status: channelKey.status,
      test_status: testStatus,
      test_message: testResult?.message,
      message: '',
      latency: testResult?.latency || '',
      channelKey,
    }
  }) || []

  return rows.sort((a, b) => {
    const groupDiff = a.channel_key_group.localeCompare(b.channel_key_group)
    if (groupDiff !== 0) return groupDiff
    return a.channel_key_id - b.channel_key_id
  })
})

const columns: DataTableColumns<KeyHealthRow> = [
  {
    title: 'ID',
    key: 'channel_key_id',
    width: 60,
    align: 'right',
  },
  {
    title: '密钥',
    key: 'channel_key_preview',
    minWidth: 240,
    render(row) {
      const metaParts: any[] = [
        h(NTag, {size: 'small', bordered: false}, {default: () => row.channel_key_group}),
      ]
      if (row.channel_key_remark) {
        metaParts.push(
          h(NText, {depth: 3, style: 'font-size: 11px'}, {default: () => row.channel_key_remark}),
        )
      }

      return h('div', {class: 'key-cell'}, [
        h('div', {class: 'key-cell__main'}, [
          h(NText, {code: true, style: 'font-size: 12px'}, {default: () => row.channel_key_preview}),
          h(
            NTooltip,
            null,
            {
              trigger: () =>
                h(
                  NButton,
                  {
                    class: 'table-action-btn',
                    size: 'tiny',
                    quaternary: true,
                    circle: true,
                    'aria-label': `复制密钥 ${row.channel_key_remark || row.channel_key_id}`,
                    onClick: () => handleCopyKey(row.channel_key_value),
                  },
                  {icon: () => h(NIcon, null, {default: () => h(CopyOutline)})},
                ),
              default: () => '复制密钥',
            },
          ),
        ]),
        h('div', {class: 'key-cell__meta'}, metaParts),
      ])
    },
  },
  {
    title: '状态',
    key: 'status',
    align: 'center',
    width: 80,
    render(row) {
      const type = row.status === 'active' ? 'success' : 'default'
      const text = row.status === 'active' ? '启用' : '停用'
      return h(NTag, {type, size: 'small', bordered: false}, {default: () => text})
    },
  },
  {
    title: '测试',
    key: 'test_status',
    align: 'center',
    width: 96,
    render(row) {
      if (testingKeys.value.has(row.channel_key_id)) {
        return h(
          NButton,
          {
            size: 'tiny',
            quaternary: true,
            loading: true,
            disabled: true,
            'aria-label': `密钥 ${row.channel_key_id} 测试中`,
          },
          {default: () => '测试中'},
        )
      }

      if (row.test_status) {
        const config = {
          healthy: {type: 'success' as const, text: '健康'},
          unhealthy: {type: 'error' as const, text: '异常'},
          error: {type: 'warning' as const, text: '错误'},
          testing: {type: 'info' as const, text: '测试中'},
        }
        const status = config[row.test_status]

        return h(
          NButton,
          {
            size: 'tiny',
            type: status.type,
            quaternary: true,
            title: row.test_message || '',
            'aria-label': `测试密钥 ${row.channel_key_id}`,
            onClick: () => handleTestChannelKey(row.channel_key_id),
          },
          {default: () => status.text},
        )
      }

      return h(
        NButton,
        {
          size: 'tiny',
          quaternary: true,
          'aria-label': `测试密钥 ${row.channel_key_id}`,
          onClick: () => handleTestChannelKey(row.channel_key_id),
        },
        {default: () => '测试'},
      )
    },
  },
  {
    title: '延迟',
    key: 'latency',
    width: 72,
    align: 'right',
    render(row) {
      if (!row.latency || row.latency === '-') {
        return h(NText, {depth: 3}, {default: () => '-'})
      }
      const match = row.latency.match(/^([\d.]+)(.*)$/)
      if (match) {
        const value = parseFloat(match[1] || '')
        const unit = match[2]
        return h(NText, {}, {default: () => `${value.toFixed(2)}${unit}`})
      }
      return h(NText, {}, {default: () => row.latency})
    },
  },
  {
    title: '操作',
    key: 'actions',
    align: 'center',
    width: 100,
    fixed: 'right',
    render(row) {
      return h(
        NSpace,
        {
          justify: 'center',
          size: 4,
          class: 'table-action-group',
        },
        {
          default: () => [
            row.channelKey && row.channelKey.status !== 'inactive' && h(
              NTooltip,
              null,
              {
                trigger: () => h(
                  NButton,
                  {
                    class: 'table-action-btn',
                    size: 'tiny',
                    type: 'warning',
                    quaternary: true,
                    circle: true,
                    'aria-label': `停用密钥 ${row.channel_key_id}`,
                    onClick: () => handleToggleChannelKeyStatus(row.channel_key_id, 'inactive'),
                  },
                  {icon: () => h(NIcon, null, {default: () => h(ContrastOutline)})},
                ),
                default: () => '停用',
              },
            ),
            row.channelKey && row.channelKey.status === 'inactive' && h(
              NTooltip,
              null,
              {
                trigger: () => h(
                  NButton,
                  {
                    class: 'table-action-btn',
                    size: 'tiny',
                    type: 'success',
                    quaternary: true,
                    circle: true,
                    'aria-label': `启用密钥 ${row.channel_key_id}`,
                    onClick: () => handleToggleChannelKeyStatus(row.channel_key_id, 'active'),
                  },
                  {icon: () => h(NIcon, null, {default: () => h(ContrastOutline)})},
                ),
                default: () => '启用',
              },
            ),
            row.channelKey && h(
              NTooltip,
              null,
              {
                trigger: () => h(
                  NButton,
                  {
                    class: 'table-action-btn',
                    size: 'tiny',
                    type: 'error',
                    quaternary: true,
                    circle: true,
                    'aria-label': `删除密钥 ${row.channel_key_id}`,
                    onClick: () => handleDeleteChannelKey(row.channel_key_id),
                  },
                  {icon: () => h(NIcon, null, {default: () => h(TrashOutline)})},
                ),
                default: () => '删除',
              },
            ),
          ],
        },
      )
    },
  },
]

async function fetchChannel() {
  if (!props.channelId || props.healthResult) return

  loading.value = true
  try {
    channel.value = await channelApi.get(props.channelId)
  } catch (err) {
    toastApiError(err, '加载渠道信息失败')
  } finally {
    loading.value = false
  }
}

async function handleCopyKey(keyValue: string) {
  if (!keyValue) return

  try {
    await navigator.clipboard.writeText(keyValue)
    window.$message?.success('Key 已复制到剪贴板')
  } catch (error) {
    console.error('Failed to copy key:', error)
    window.$message?.error('复制失败，请手动复制')
  }
}

async function handleDeleteChannelKey(channelKeyId: number) {
  window.$dialog?.warning({
    title: '确认删除',
    content: '确定要删除此渠道密钥吗？',
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await channelApi.deleteChannelKey(channelKeyId)
        window.$message?.success('删除成功')
        await fetchChannel()
        emit('refresh')
      } catch (err) {
        toastApiError(err, '删除失败')
      }
    },
  })
}

async function handleTestChannelKey(channelKeyId: number) {
  testingKeys.value.add(channelKeyId)

  try {
    const result = await channelApi.testSingleChannelKey(channelKeyId)

    testResults.value.set(channelKeyId, {
      status: result.status,
      message: result.message,
      latency: result.latency,
    })

    if (result.status === 'healthy') {
      window.$message?.success(`测试成功: ${result.message}`)
    } else {
      window.$message?.error(`测试失败: ${result.message}`)
    }
    await fetchChannel()
  } catch (err) {
    toastApiError(err, '测试失败')

    testResults.value.set(channelKeyId, {
      status: 'error',
      message: (err as any)?.response?.data?.error || '测试失败',
      latency: '-',
    })
  } finally {
    testingKeys.value.delete(channelKeyId)
  }
}

async function handleToggleChannelKeyStatus(channelKeyId: number, targetStatus: 'active' | 'inactive') {
  const action = targetStatus === 'inactive' ? '停用' : '启用'

  window.$dialog?.warning({
    title: `确认${action}`,
    content: `确定要${action}此密钥吗？`,
    positiveText: '确定',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await channelApi.resetChannelKeyStatus(channelKeyId, targetStatus)
        window.$message?.success(`${action}成功`)
        await fetchChannel()
        emit('refresh')
      } catch (err) {
        toastApiError(err, `${action}失败`)
      }
    },
  })
}

async function refresh() {
  await fetchChannel()
}

function setTestingAll() {
  testingKeys.value.clear()
  displayData.value.forEach((row) => {
    testingKeys.value.add(row.channel_key_id)
  })
}

function updateTestResults(results: ChannelHealthCheckResult) {
  testingKeys.value.clear()
  results.channel_key_results.forEach((result) => {
    testResults.value.set(result.channel_key_id, {
      status: result.status,
      message: result.message,
      latency: result.latency,
    })
  })
}

defineExpose({
  refresh,
  setTestingAll,
  updateTestResults,
})

onMounted(() => {
  fetchChannel()
})
</script>

<style scoped>
.key-health-table {
  width: 100%;
}

.key-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.key-cell__main {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.key-cell__meta {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  flex-wrap: wrap;
}
</style>
