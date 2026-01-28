<template>
  <div class="space-y-4">
    <!-- 页面头部 -->
    <n-card :bordered="false" class="page-header-card">
      <n-space justify="space-between" align="center">
        <n-space vertical :size="4">
          <n-text class="page-title">渠道管理</n-text>
          <n-text depth="3" class="page-subtitle">
            管理 AI 服务渠道，配置接入地址、密钥和模型映射
          </n-text>
        </n-space>
        <n-button type="primary" @click="handleCreate" size="large" strong>
          <template #icon>
            <n-icon>
              <AddOutline/>
            </n-icon>
          </template>
          新建渠道
        </n-button>
      </n-space>
    </n-card>

    <!-- 过滤表单 -->
    <n-form inline :label-width="50" :model="filters" :label-placement="'left'" :label-align="'left'" :show-feedback="false">
      <n-grid :cols="24" :x-gap="24" responsive="screen">
        <n-form-item-gi :span="6" label="名称">
          <n-input
              v-model:value="filters.name"
              placeholder="输入渠道名称"
              clearable
              @update:value="handleFilterChange"
          />
        </n-form-item-gi>
        <n-form-item-gi :span="6" label="BASE URL" :label-width="80">
          <n-input
              v-model:value="filters.base_url"
              placeholder="输入 Base URL"
              clearable
              @update:value="handleFilterChange"
          />
        </n-form-item-gi>
        <n-form-item-gi :span="6" label="状态">
          <n-select
              v-model:value="filters.status"
              placeholder="选择状态"
              clearable
              :options="statusOptions"
              @update:value="handleFilterChange"
          />
        </n-form-item-gi>
        <n-form-item-gi :span="6">
          <n-space>
            <n-button type="primary" @click="handleSearch">
              <template #icon>
                <n-icon>
                  <SearchOutline/>
                </n-icon>
              </template>
              查询
            </n-button>
            <n-button @click="handleReset">
              <template #icon>
                <n-icon>
                  <RefreshOutline/>
                </n-icon>
              </template>
              重置
            </n-button>
          </n-space>
        </n-form-item-gi>
      </n-grid>
    </n-form>

    <!-- 数据表格 -->
    <n-data-table
        :columns="columns"
        :data="channels"
        :loading="loading"
        :pagination="false"
        :single-line="false"
        bordered
        :scroll-x="1000"
        striped
        :row-key="(row: Channel) => row.id"
        @update:sorter="handleSorterChange"
    />

    <div class="flex justify-end">
      <n-pagination
          :page="pagination.page"
          :on-update-page="pagination.onChange"
          @update:page-size="pagination.onUpdatePageSize"
          :page-size="pagination.pageSize"
          :item-count="pagination.total"
          :page-sizes="pagination.pageSizes"
          :show-size-picker="pagination.showSizePicker"
      />
    </div>

    <!-- 渠道对话框 -->
    <ChannelDialog
        v-if="showDialog"
        :channel="editingChannel"
        @confirm="handleDialogConfirm"
        @cancel="handleDialogCancel"
    />

    <!-- 密钥管理对话框 -->
    <KeyManagementDialog
        v-if="selectedChannel"
        v-model="showKeyManagement"
        :channel-id="selectedChannel.id"
        :channel-name="selectedChannel.name"
        @refresh="fetchChannels"
    />

    <!-- 模型管理对话框 -->
    <ModelManagementDialog
        v-model="showModelManagement"
        :channel-id="selectedChannel?.id || 0"
        :channel-name="selectedChannel?.name || ''"
        @refresh="fetchChannels"
    />
  </div>


</template>

<script setup lang="ts">
import {computed, h, onMounted, reactive, ref} from 'vue'
import {
  type DataTableColumns,
  NButton,
  NCard,
  NDataTable,
  NForm,
  NFormItemGi,
  NGrid,
  NIcon,
  NInput,
  NPagination,
  NSelect,
  NSpace,
  NTag,
  NText,
  NTooltip
} from 'naive-ui'
import {AddOutline, CreateOutline, GridOutline, KeyOutline, RefreshOutline, SearchOutline, TrashOutline} from '@vicons/ionicons5'
import {channelApi} from '../services/channelService'
import type {Channel, ChannelListParams} from '../types/channel'
import ChannelDialog from '../components/ChannelDialog.vue'
import KeyManagementDialog from '../components/KeyManagementDialog.vue'
import ModelManagementDialog from '../components/ModelManagementDialog.vue'

// State
const channels = ref<Channel[]>([])
const loading = ref(false)
const showDialog = ref(false)
const editingChannel = ref<Channel | null>(null)
const showKeyManagement = ref(false)
const showModelManagement = ref(false)
const selectedChannel = ref<Channel | null>(null)

// 过滤条件
const filters = reactive<ChannelListParams>({
  name: '',
  base_url: '',
  status: null
})

// 排序状态
const sortState = reactive({
  columnKey: '' as 'id' | 'name' | 'priority' | 'weight' | 'status' | '',
  order: false as boolean | 'asc' | 'desc' // false: 无排序, 'asc': 升序, 'desc': 降序
})

// 状态选项
const statusOptions = [
  {label: '激活', value: 'active'},
  {label: '禁用', value: 'disabled'}
]

// 分页
const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0,
  showSizePicker: true,
  pageSizes: [10, 20, 50],
  onChange: (page: number) => {
    pagination.page = page
    fetchChannels()
  },
  onUpdatePageSize: (pageSize: number) => {
    pagination.pageSize = pageSize
    pagination.page = 1
    fetchChannels()
  }
})

function getModelStats(channel: Channel) {
  const stats = channel.model_stats
  if (stats) {
    return {
      active: stats.active || 0,
      cooling: stats.cooling || 0,
      disabled: stats.disabled || 0,
      nonExist: stats.non_exist || 0
    }
  }

  if (channel.model_configs && channel.model_configs.length > 0) {
    const result = {active: 0, cooling: 0, disabled: 0, nonExist: 0}
    channel.model_configs.forEach((config) => {
      if (config.status === 'active') {
        result.active += 1
      } else if (config.status === 'cooling') {
        result.cooling += 1
      } else if (config.status === 'disabled') {
        result.disabled += 1
      } else if (config.status === 'non_exist') {
        result.nonExist += 1
      }
    })
    return result
  }

  return {active: 0, cooling: 0, disabled: 0, nonExist: 0}
}

// 获取渠道列表
async function fetchChannels() {
  loading.value = true
  try {
    const params: ChannelListParams = {
      page: pagination.page,
      page_size: pagination.pageSize,
      name: filters.name || undefined,
      base_url: filters.base_url || undefined,
      status: filters.status || undefined
    }

    // 添加排序参数
    if (sortState.columnKey && sortState.order) {
      params.sort_by = sortState.columnKey
      params.sort_order = sortState.order as 'asc' | 'desc'
    }

    const result = await channelApi.list(params)
    channels.value = result.items
    pagination.total = result.total
  } catch (error: any) {
    console.error('Failed to fetch channels:', error)
    window.$message?.error(error.response?.data?.error || '获取渠道列表失败')
  } finally {
    loading.value = false
  }
}

// 处理排序变化
function handleSorterChange(sorter: { columnKey: string; order: 'ascend' | 'descend' | false }) {
  if (sorter.columnKey) {
    sortState.columnKey = sorter.columnKey as 'id' | 'name' | 'priority' | 'weight' | 'status'
    sortState.order = sorter.order === 'ascend' ? 'asc' : sorter.order === 'descend' ? 'desc' : false
  } else {
    sortState.columnKey = '' as 'id' | 'name' | 'priority' | 'weight' | 'status'
    sortState.order = false
  }

  pagination.page = 1
  fetchChannels()
}

// 过滤条件变化时重置到第一页
function handleFilterChange() {
  pagination.page = 1
}

// 搜索
function handleSearch() {
  pagination.page = 1
  fetchChannels()
}

// 重置
function handleReset() {
  filters.name = ''
  filters.base_url = ''
  filters.status = null
  pagination.page = 1
  fetchChannels()
}

// 表格列定义（使用 computed 响应式更新排序状态）
const columns = computed<DataTableColumns<Channel>>(() => {
  const getSortOrder = (key: string) => {
    if (sortState.columnKey === key) {
      return sortState.order === 'asc' ? 'ascend' : sortState.order === 'desc' ? 'descend' : false
    }
    return false
  }

  return [
    {
      title: 'ID',
      key: 'id',
      width: 80,
      align: 'left',
      sortable: true,
      sorter: 'default',
      sortOrder: getSortOrder('id')
    },
    {
      title: '名称',
      key: 'name',
      width: 160,
      ellipsis: {
        tooltip: true
      },
      render(row) {
        return h(
            NButton,
            {
              text: true,
              tag: 'a',
              size: "small",
              href: row.base_url,
              type: "primary",
              target: "_blank"
            },
            {
              default: () => (row.name)
            }
        )
      }
    },
    {
      title: '状态',
      key: 'status',
      align: 'center',
      width: 80,
      sortable: true,
      sorter: 'default',
      sortOrder: getSortOrder('status'),
      render(row) {
        return h(
            NTag,
            {
              type: row.status === 'active' ? 'success' : 'warning',
              size: "small"
            },
            {
              default: () => (row.status === 'active' ? '激活' : '禁用')
            }
        )
      }
    },
    {
      title: '优先级',
      key: 'priority',
      width: 80,
      align: 'right',
      sortable: true,
      sorter: 'default',
      sortOrder: getSortOrder('priority')
    },
    {
      title: '权重',
      key: 'weight',
      width: 80,
      align: 'right',
      sortable: true,
      sorter: 'default',
      sortOrder: getSortOrder('weight')
    },
    {
      title: '模型',
      key: 'model_count',
      width: 120,
      align: 'right',
      render(row) {
        const stats = getModelStats(row)
        const color = stats.active > 0
            ? '#10b981'
            : stats.cooling > 0
                ? '#f59e0b'
                : stats.disabled > 0
                    ? '#f59e0b'
                    : '#ef4444'

        return h(
            NTooltip,
            {},
            {
              trigger: () =>
                  h(NText, {style: {color: color, fontWeight: 500}}, {
                    default: () => `${stats.active} / ${stats.cooling} / ${stats.disabled} / ${stats.nonExist}`
                  }),
              default: () =>
                  h('div', {style: {lineHeight: '1.8'}}, [
                    h('div', {}, `正常：${stats.active}`),
                    h('div', {}, `冷却：${stats.cooling}`),
                    h('div', {}, `禁用：${stats.disabled}`),
                    h('div', {}, `失效：${stats.nonExist}`)
                  ])
            }
        )
      }
    },
    {
      title: '密钥',
      key: 'keys_count',
      width: 120,
      align: 'right',
      render(row) {
        const stats = row.key_stats
        if (!stats) {
          return h(NText, {depth: 3}, {default: () => '0 / 0 / 0'})
        }
        const color = stats.active > 0 ? '#10b981' : stats.cooling > 0 ? '#f59e0b' : '#ef4444'

        return h(
            NTooltip,
            {},
            {
              trigger: () =>
                  h(NText, {style: {color: color, fontWeight: 500}}, {
                    default: () => `${stats.active} / ${stats.cooling} / ${stats.disabled}`
                  }),
              default: () =>
                  h('div', {style: {lineHeight: '1.8'}}, [
                    h('div', {}, `健康：${stats.active}`),
                    h('div', {}, `冷却：${stats.cooling}`),
                    h('div', {}, `失效：${stats.disabled}`)
                  ])
            }
        )
      }
    },
    {
      title: '操作',
      key: 'actions',
      width: 280,
      align: 'center',
      fixed: 'right',
      render(row) {
        return h(
            'div',
            {class: 'flex gap-2 justify-center'},
            [
              h(
                  NButton,
                  {
                    size: 'tiny',
                    type: 'info',
                    onClick: () => handleKeyManagement(row)
                  },
                  {
                    default: () => '密钥管理',
                    icon: () => h(NIcon, {}, {default: () => h(KeyOutline)})
                  }
              ),
              h(
                  NButton,
                  {
                    size: 'tiny',
                    type: 'primary',
                    onClick: () => handleModelManagement(row)
                  },
                  {
                    default: () => '模型管理',
                    icon: () => h(NIcon, {}, {default: () => h(GridOutline)})
                  }
              ),
              h(
                  NButton,
                  {
                    size: 'tiny',
                    type: 'warning',
                    onClick: () => handleEdit(row)
                  },
                  {
                    default: () => '编辑',
                    icon: () => h(NIcon, {}, {default: () => h(CreateOutline)})
                  }
              ),
              h(
                  NButton,
                  {
                    size: 'tiny',
                    type: 'error',
                    onClick: () => handleDelete(row)
                  },
                  {
                    default: () => '删除',
                    icon: () => h(NIcon, {}, {default: () => h(TrashOutline)})
                  }
              )
            ]
        )
      }
    }
  ]
})


// 密钥管理
function handleKeyManagement(channel: Channel) {
  selectedChannel.value = channel
  showKeyManagement.value = true
}

// 模型管理
function handleModelManagement(channel: Channel) {
  selectedChannel.value = channel
  showModelManagement.value = true
}

// 创建渠道
function handleCreate() {
  editingChannel.value = null
  showDialog.value = true
}

// 编辑渠道
function handleEdit(channel: Channel) {
  editingChannel.value = channel
  showDialog.value = true
}

// 删除渠道
async function handleDelete(channel: Channel) {
  await window.$dialog?.warning({
    title: '确认删除',
    content: `确定要删除渠道 "${channel.name}" 吗？`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await channelApi.delete(channel.id)
        window.$message?.success('删除成功')
        await fetchChannels()
      } catch (error: any) {
        console.error('Failed to delete channel:', error)
        window.$message?.error(error.response?.data?.error || '删除失败')
      }
    }
  })

}

// 对话框确认
async function handleDialogConfirm(data: any) {
  try {
    if (editingChannel.value) {
      // 更新
      await channelApi.update(editingChannel.value.id, data)
      window.$message?.success('更新成功')
    } else {
      // 创建
      await channelApi.create(data)
      window.$message?.success('创建成功')
    }

    showDialog.value = false
    await fetchChannels()
  } catch (error: any) {
    console.error('Failed to save channel:', error)
    window.$message?.error(error.response?.data?.error || '保存失败')
  }
}

// 对话框取消
function handleDialogCancel() {
  showDialog.value = false
  editingChannel.value = null
}

// 初始化
onMounted(() => {
  fetchChannels()
})
</script>

<style scoped>
/* 页面头部卡片 */
.page-header-card {
  background: linear-gradient(135deg, #f5f7fa 0%, #ffffff 100%);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
  border-radius: 12px;
  padding: 24px;
}

.page-title {
  font-size: 24px;
  font-weight: 700;
  color: #333;
  line-height: 1.4;
}

.page-subtitle {
  font-size: 14px;
  line-height: 1.6;
}

</style>
