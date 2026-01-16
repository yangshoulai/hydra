<template>
  <div class="space-y-4 animate-fade-in channel-list-page">
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

    <!-- 数据表格 -->
    <n-data-table
        :columns="columns"
        :data="channels"
        :loading="loading"
        :pagination="pagination"
        :single-line="false"
        bordered
        :scroll-x="1600"
        striped
        :row-key="(row: Channel) => row.id"
    />

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
import {h, onMounted, reactive, ref} from 'vue'
import {type DataTableColumns, NButton, NCard, NDataTable, NIcon, NSpace, NTag, NText} from 'naive-ui'
import {AddOutline, GridOutline, KeyOutline} from '@vicons/ionicons5'
import {channelApi} from '../services/channelService'
import type {Channel} from '../types/channel'
import ChannelDialog from '../components/ChannelDialog.vue'
import KeyManagementDialog from '../components/KeyManagementDialog.vue'
import ModelManagementDialog from '../components/ModelManagementDialog.vue' // State

// State
const channels = ref<Channel[]>([])
const loading = ref(false)
const showDialog = ref(false)
const editingChannel = ref<Channel | null>(null)
const showKeyManagement = ref(false)
const showModelManagement = ref(false)
const selectedChannel = ref<Channel | null>(null)

// 分页
const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0,
  showSizePicker: true,
  pageSizes: [10, 20, 50, 100],
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

// 表格列定义
const columns: DataTableColumns<Channel> = [
  {
    title: 'ID',
    key: 'id',
    width: 80,
    align: 'left'
  },
  {
    title: '名称',
    key: 'name',
    width: 240
  },
  {
    title: 'BASE URL',
    key: 'base_url',
    width: 320,
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
      return h(
          NTag,
          {
            type: row.status === 'active' ? 'success' : 'default',
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
    width: 160,
    align: 'right'
  },
  {
    title: '权重',
    key: 'weight',
    width: 160,
    align: 'right'
  },
  {
    title: '密钥数量',
    key: 'keys_count',
    width: 120,
    render(row) {
      return row.keys?.length || 0
    },
    align: 'right'
  },
  {
    title: '操作',
    key: 'actions',
    width: 360,
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
                  size: 'small',
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
                  size: 'small',
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
                  size: 'small',
                  onClick: () => handleEdit(row)
                },
                {default: () => '编辑'}
            ),
            h(
                NButton,
                {
                  size: 'small',
                  type: 'error',
                  onClick: () => handleDelete(row)
                },
                {default: () => '删除'}
            )
          ]
      )
    }
  }
]

// 获取渠道列表
async function fetchChannels() {
  loading.value = true
  try {
    const result = await channelApi.list(pagination.page, pagination.pageSize)
    channels.value = result.items
    pagination.total = result.total
  } catch (error: any) {
    console.error('Failed to fetch channels:', error)
    window.$message?.error(error.response?.data?.error || '获取渠道列表失败')
  } finally {
    loading.value = false
  }
}

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
  const confirmed = await window.$dialog?.warning({
    title: '确认删除',
    content: `确定要删除渠道 "${channel.name}" 吗？`,
    positiveText: '删除',
    negativeText: '取消'
  })

  if (!confirmed) return

  try {
    await channelApi.delete(channel.id)
    window.$message?.success('删除成功')
    await fetchChannels()
  } catch (error: any) {
    console.error('Failed to delete channel:', error)
    window.$message?.error(error.response?.data?.error || '删除失败')
  }
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
/* 页面样式 */
.channel-list-page {
  --primary-color: #18a058;
  --primary-color-hover: #36ad6a;
  --info-color: #2080f0;
  --warning-color: #f0a020;
  --success-color: #18a058;
  --error-color: #d03050;
}

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


/* 响应式调整 */
@media (max-width: 768px) {
  .page-header-card {
    padding: 16px;
  }

  .page-title {
    font-size: 20px;
  }

  .channel-list-page :deep(.n-button) {
    font-size: 14px;
  }
}
</style>