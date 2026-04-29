<template>
  <div class="app-page">
    <n-alert v-if="listError" type="error" :bordered="false">
      <n-space align="center" justify="space-between" style="width: 100%">
        <span>{{ listError }}</span>
        <n-button text type="error" @click="fetchChannels">重试</n-button>
      </n-space>
    </n-alert>

    <section class="panel-card">
      <header class="panel-card__header table-toolbar">
        <div class="table-toolbar__title">
          <h3 class="panel-card__title">渠道列表</h3>
        </div>
        <div class="table-toolbar__actions">
          <n-button size="small" type="primary" @click="handleCreate">
            <template #icon>
              <n-icon>
                <AddOutline />
              </n-icon>
            </template>
            新建渠道
          </n-button>
        </div>
      </header>
      <div class="panel-card__body">
        <div class="table-inline-search">
          <n-input
            v-model:value="filters.name"
            class="table-filter-input"
            size="small"
            clearable
            placeholder="渠道名称"
            @keyup.enter="handleSearch"
          >
            <template #prefix>
              <n-icon><SearchOutline /></n-icon>
            </template>
          </n-input>
          <n-input
            v-model:value="filters.base_url"
            class="table-filter-input"
            size="small"
            clearable
            placeholder="Base URL"
            @keyup.enter="handleSearch"
          >
            <template #prefix>
              <n-icon><SearchOutline /></n-icon>
            </template>
          </n-input>
          <n-select
            v-model:value="filters.status"
            class="table-filter-select"
            size="small"
            clearable
            :options="statusOptions"
            placeholder="状态"
          />
          <n-button size="small" type="primary" @click="handleSearch">查询</n-button>
          <n-button size="small" quaternary @click="handleReset">重置</n-button>
        </div>

        <n-data-table
          :columns="columns"
          :data="channels"
          :loading="loading"
          :locale="tableLocale"
          :pagination="false"
          :single-line="false"
          :scroll-x="1030"
          :row-key="(row: Channel) => row.id"
          @update:sorter="handleSorterChange"
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

    <ChannelEditorDrawer
      v-model="showEditor"
      :channel="editingChannel"
      @saved="handleSaved"
    />

    <ModelManagementDialog
      v-model="showModelManagement"
      :channel-id="selectedChannel?.id || 0"
      :channel-name="selectedChannel?.name || ''"
      @refresh="fetchChannels"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import {
  type DataTableColumns,
  NAlert,
  NButton,
  NDataTable,
  NIcon,
  NInput,
  NPagination,
  NSelect,
  NSpace,
  NTag,
  NTooltip,
} from 'naive-ui'
import { AddOutline, LayersOutline, PowerOutline, SearchOutline, SettingsOutline, TrashOutline } from '@vicons/ionicons5'
import type { Channel, ChannelListParams } from '@/types/channel'
import { channelApi } from '@/services/channelService'
import ChannelEditorDrawer from '@/components/ChannelEditorDrawer.vue'
import ModelManagementDialog from '@/components/ModelManagementDialog.vue'
import { getErrorMessage, toastApiError } from '@/utils/error'
import { feedback } from '@/services/feedback'

const channels = ref<Channel[]>([])
const loading = ref(false)
const listError = ref('')
const showEditor = ref(false)
const showModelManagement = ref(false)
const editingChannel = ref<Channel | null>(null)
const selectedChannel = ref<Channel | null>(null)
const togglingStatusMap = ref<Record<number, boolean>>({})

const filters = reactive<ChannelListParams>({
  name: '',
  base_url: '',
  status: null,
})

const sortState = reactive({
  columnKey: 'id' as 'id' | 'name' | 'weight' | 'status' | '',
  order: 'asc' as boolean | 'asc' | 'desc',
})

const statusOptions = [
  { label: '启用', value: 'active' },
  { label: '停用', value: 'inactive' },
]

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
  },
})

const tableLocale = computed(() => ({
  emptyText: loading.value ? '加载中...' : '暂无渠道数据',
}))

function getSortOrder(key: string) {
  if (sortState.columnKey === key) {
    return sortState.order === 'asc' ? 'ascend' : sortState.order === 'desc' ? 'descend' : false
  }
  return false
}

const columns = computed<DataTableColumns<Channel>>(() => [
  {
    title: 'ID',
    key: 'id',
    width: 70,
    sortable: true,
    sorter: 'default',
    sortOrder: getSortOrder('id'),
  },
  {
    title: '渠道',
    key: 'name',
    minWidth: 210,
    ellipsis: { tooltip: true },
    render: (row) =>
      h('div', { class: 'channel-cell' }, [
        h('div', { class: 'channel-cell__name', title: row.name }, row.name),
        row.base_url
          ? h('a', {
              href: buildBaseUrlLink(row.base_url),
              target: '_blank',
              rel: 'noopener noreferrer',
              class: 'channel-cell__url',
              title: row.base_url,
            }, row.base_url)
          : h('span', { class: 'channel-cell__url channel-cell__url--empty' }, '—'),
      ]),
  },
  {
    title: '状态',
    key: 'status',
    width: 120,
    align: 'center',
    sortable: true,
    sorter: 'default',
    sortOrder: getSortOrder('status'),
    render: (row) =>
      h(
        NTag,
        { type: row.status === 'active' ? 'success' : 'default', size: 'small', bordered: false },
        { default: () => (row.status === 'active' ? '启用' : '停用') },
      ),
  },
  {
    title: '权重',
    key: 'weight',
    width: 120,
    align: 'right',
    sortable: true,
    sorter: 'default',
    sortOrder: getSortOrder('weight'),
  },
  {
    title: '模型数',
    key: 'model_count',
    width: 120,
    align: 'right',
    render: (row) => `${row.model_stats?.active || 0} / ${row.model_stats?.inactive || 0}`,
  },
  {
    title: '密钥数',
    key: 'key_stats',
    width: 120,
    align: 'right',
    render: (row) => `${row.key_stats?.active || 0} / ${row.key_stats?.inactive || 0}`,
  },
  {
    title: '更新时间',
    key: 'updated_at',
    width: 180,
    align: 'center',
    render: (row) => new Date(row.updated_at).toLocaleString('zh-CN'),
  },
  {
    title: '操作',
    key: 'actions',
    width: 180,
    fixed: 'right',
    align: 'center',
    render: (row) =>
      h(NSpace, { size: 4, justify: 'center', class: 'table-action-group' }, {
        default: () => [
          renderActionIcon({
            tooltip: '基础/密钥',
            ariaLabel: `编辑渠道 ${row.name} 的基础信息和密钥`,
            icon: SettingsOutline,
            type: 'primary',
            onClick: () => handleEdit(row),
          }),
          renderActionIcon({
            tooltip: '模型配置',
            ariaLabel: `配置渠道 ${row.name} 的模型`,
            icon: LayersOutline,
            type: 'info',
            onClick: () => handleModelManagement(row),
          }),
          renderActionIcon({
            tooltip: row.status === 'active' ? '禁用' : '启用',
            ariaLabel: `${row.status === 'active' ? '禁用' : '启用'}渠道 ${row.name}`,
            icon: PowerOutline,
            type: row.status === 'active' ? 'warning' : 'success',
            loading: !!togglingStatusMap.value[row.id],
            onClick: () => handleToggleStatus(row),
          }),
          renderActionIcon({
            tooltip: '删除',
            ariaLabel: `删除渠道 ${row.name}`,
            icon: TrashOutline,
            type: 'error',
            onClick: () => handleDelete(row),
          }),
        ],
      }),
  },
])

function renderActionIcon(options: {
  tooltip: string
  ariaLabel: string
  icon: any
  type?: 'default' | 'primary' | 'info' | 'warning' | 'error' | 'success'
  loading?: boolean
  onClick: () => void
}) {
  return h(
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
            type: options.type,
            circle: true,
            loading: options.loading,
            'aria-label': options.ariaLabel,
            onClick: options.onClick,
          },
          {
            icon: () => h(NIcon, null, { default: () => h(options.icon) }),
          },
        ),
      default: () => options.tooltip,
    },
  )
}

function buildBaseUrlLink(baseUrl: string): string {
  const trimmed = (baseUrl || '').trim()
  if (!trimmed) return '#'
  if (/^https?:\/\//i.test(trimmed)) return trimmed
  return `https://${trimmed}`
}

async function fetchChannels() {
  loading.value = true
  listError.value = ''
  try {
    const params: ChannelListParams = {
      page: pagination.page,
      page_size: pagination.pageSize,
      name: filters.name || undefined,
      base_url: filters.base_url || undefined,
      status: filters.status || undefined,
    }

    params.sort_by = (sortState.columnKey || 'id') as 'id' | 'name' | 'weight' | 'status'
    params.sort_order = (sortState.order || 'asc') as 'asc' | 'desc'

    const result = await channelApi.list(params)
    channels.value = result.items
    pagination.total = result.total
  } catch (err) {
    listError.value = getErrorMessage(err, '获取渠道列表失败')
    feedback.message?.error(listError.value)
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  pagination.page = 1
  fetchChannels()
}

function handleReset() {
  filters.name = ''
  filters.base_url = ''
  filters.status = null
  sortState.columnKey = 'id'
  sortState.order = 'asc'
  pagination.page = 1
  fetchChannels()
}

function handleSorterChange(sorter: { columnKey: string; order: 'ascend' | 'descend' | false }) {
  if (sorter.columnKey) {
    sortState.columnKey = sorter.columnKey as 'id' | 'name' | 'weight' | 'status'
    sortState.order = sorter.order === 'ascend' ? 'asc' : sorter.order === 'descend' ? 'desc' : 'asc'
  } else {
    sortState.columnKey = 'id'
    sortState.order = 'asc'
  }

  pagination.page = 1
  fetchChannels()
}

function handleCreate() {
  editingChannel.value = null
  showEditor.value = true
}

function handleEdit(channel: Channel) {
  editingChannel.value = channel
  showEditor.value = true
}

function handleModelManagement(channel: Channel) {
  selectedChannel.value = channel
  showModelManagement.value = true
}

async function handleToggleStatus(channel: Channel) {
  const nextStatus: 'active' | 'inactive' = channel.status === 'active' ? 'inactive' : 'active'
  const actionText = nextStatus === 'active' ? '启用' : '禁用'

  await feedback.dialog?.warning({
    title: '确认操作',
    content: `确定要${actionText}渠道“${channel.name}”吗？`,
    positiveText: '确认',
    negativeText: '取消',
    onPositiveClick: async () => {
      togglingStatusMap.value = {
        ...togglingStatusMap.value,
        [channel.id]: true,
      }
      try {
        await channelApi.update(channel.id, { status: nextStatus })
        feedback.message?.success(`渠道已${actionText}`)
        await fetchChannels()
      } catch (err) {
        toastApiError(err, '更新渠道状态失败')
      } finally {
        const { [channel.id]: _removed, ...rest } = togglingStatusMap.value
        togglingStatusMap.value = rest
      }
    },
  })
}

async function handleDelete(channel: Channel) {
  await feedback.dialog?.warning({
    title: '确认删除',
    content: `确定删除渠道“${channel.name}”吗？相关密钥和模型配置会一并删除。`,
    positiveText: '确认删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await channelApi.delete(channel.id)
        feedback.message?.success('删除成功')
        await fetchChannels()
      } catch (err) {
        toastApiError(err, '删除失败')
      }
    },
  })
}

function handleSaved() {
  fetchChannels()
}

onMounted(() => {
  fetchChannels()
})
</script>

<style scoped>
.table-toolbar__actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: auto;
}

:deep(.channel-cell) {
  min-width: 0;
}

:deep(.channel-cell__name) {
  font-weight: 650;
  color: #111111;
  line-height: 1.35;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

:deep(.channel-cell__url) {
  margin-top: 2px;
  display: block;
  max-width: 300px;
  font-size: 12px;
  color: #525252;
  line-height: 1.35;
  text-decoration: none;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

:deep(a.channel-cell__url:hover) {
  color: #111111;
  text-decoration: none;
}

:deep(.channel-cell__url--empty) {
  color: #a3a3a3;
}
</style>
