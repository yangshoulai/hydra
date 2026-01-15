<template>
  <div class="space-y-4 animate-fade-in">
    <!-- 顶部操作按钮 -->
    <n-space>
      <n-button @click="handleBack">
        <template #icon>
          <n-icon>
            <ArrowBack/>
          </n-icon>
        </template>
        返回
      </n-button>
    </n-space>

    <n-grid :cols="2" :x-gap="16" responsive="screen">
      <!-- 渠道信息面板 -->
      <n-grid-item span="2">
        <n-card size="small">
          <template #header>
            <n-space justify="space-between" align="center">
              <n-text strong>渠道信息</n-text>
              <n-button @click="handleEdit">
                <template #icon>
                  <n-icon>
                    <Pencil/>
                  </n-icon>
                </template>
                编辑
              </n-button>
            </n-space>
          </template>
          <n-descriptions :column="3" bordered>
            <n-descriptions-item label="ID">{{ channel?.id }}</n-descriptions-item>
            <n-descriptions-item label="名称">{{ channel?.name }}</n-descriptions-item>
            <n-descriptions-item label="Base URL" :span="2">
              <n-text code>{{ channel?.base_url }}</n-text>
            </n-descriptions-item>
            <n-descriptions-item label="优先级">{{ channel?.priority }}</n-descriptions-item>
            <n-descriptions-item label="权重">{{ channel?.weight }}</n-descriptions-item>
            <n-descriptions-item label="状态">
              <n-tag :type="channel?.status === 'active' ? 'success' : 'default'">
                {{ channel?.status === 'active' ? '激活' : '禁用' }}
              </n-tag>
            </n-descriptions-item>
            <n-descriptions-item label="描述" :span="3">
              {{ channel?.description || '-' }}
            </n-descriptions-item>
          </n-descriptions>
        </n-card>
      </n-grid-item>
    </n-grid>

    <!-- 编辑渠道对话框 -->
    <ChannelDialog
        v-if="showEditDialog"
        :channel="channel"
        @confirm="handleEditConfirm"
        @cancel="showEditDialog = false"
    />
  </div>
</template>

<script setup lang="ts">
import {onMounted, ref} from 'vue'
import {useRoute, useRouter} from 'vue-router'
import {
  NButton,
  NCard,
  NDescriptions,
  NDescriptionsItem,
  NGrid,
  NGridItem,
  NIcon,
  NSpace,
  NTag,
  NText
} from 'naive-ui'
import {ArrowBack, Pencil} from '@vicons/ionicons5'
import {channelApi} from '../services/channelService'
import type {Channel} from '../types/channel'
import ChannelDialog from '../components/ChannelDialog.vue'

const route = useRoute()
const router = useRouter()

const channelId = parseInt(route.params.id as string)

// State
const channel = ref<Channel | null>(null)
const showEditDialog = ref(false)

// 获取渠道详情
async function fetchChannel() {
  try {
    channel.value = await channelApi.get(channelId)
  } catch (error: any) {
    console.error('Failed to fetch channel:', error)
    window.$message?.error(error.response?.data?.error || '获取渠道详情失败')
  }
}

// 返回
function handleBack() {
  router.back()
}

// 编辑渠道
function handleEdit() {
  showEditDialog.value = true
}

// 编辑确认
async function handleEditConfirm(data: any) {
  try {
    await channelApi.update(channelId, data)
    window.$message?.success('更新成功')
    showEditDialog.value = false
    await fetchChannel()
  } catch (error: any) {
    console.error('Failed to update channel:', error)
    window.$message?.error(error.response?.data?.error || '更新失败')
  }
}

// 初始化
onMounted(() => {
  fetchChannel()
})
</script>

<style scoped>
/* ===================
   渠道详情容器
   =================== */
.channel-detail-container {
  animation: fadeIn 0.4s ease-out;
}

/* ===================
   操作按钮区域
   =================== */
.action-bar {
  display: flex;
  gap: 12px;
  margin-bottom: 24px;
  flex-wrap: wrap;
}

.action-bar :deep(.n-button) {
  min-width: 100px;
  height: 40px;
  font-size: 14px;
  font-weight: 600;
  border-radius: var(--radius-md);
  padding: 0 20px;
  transition: all 200ms cubic-bezier(0.4, 0, 0.2, 1);
  color: white;
}

.action-bar :deep(.n-button:not(.n-button--primary-type):not(.n-button--info-type)) {
  color: #1f2937;
}

.action-bar :deep(.n-button:hover) {
  transform: translateY(-1px);
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

/* ===================
   卡片样式
   =================== */
:deep(.n-card) {
  background: #ffffff;
  border-radius: var(--radius-xl);
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  border: 1px solid #e5e7eb;
  overflow: hidden;
  transition: all 200ms cubic-bezier(0.4, 0, 0.2, 1);
  margin-bottom: 24px;
}

/* ===================
   描述列表样式
   =================== */
:deep(.n-descriptions) {
  border-radius: var(--radius-lg);
  overflow: hidden;
}

:deep(.n-descriptions .n-descriptions-table-wrapper) {
  border-radius: var(--radius-lg);
}

:deep(.n-descriptions .n-descriptions-th) {
  background: #f3f4f6;
  font-weight: 600;
  font-size: 13px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: #1f2937;
  width: 150px;
}

:deep(.n-descriptions .n-descriptions-td) {
  color: #4b5563;
  font-size: 14px;
}

/* ===================
   表格样式
   =================== */
:deep(.n-data-table) {
  border: none;
  border-radius: 0;
}

:deep(.n-data-table .n-data-table-th) {
  background: #f3f4f6;
  font-weight: 600;
  font-size: 13px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  padding: 14px 16px;
  color: #1f2937;
  border-bottom: 2px solid #e5e7eb;
}

:deep(.n-data-table .n-data-table-td) {
  padding: 14px 16px;
  border-bottom: 1px solid #e5e7eb;
  color: #4b5563;
  font-size: 14px;
}

:deep(.n-data-table .n-data-table-tr:hover .n-data-table-td) {
  background: #f9fafb;
}

/* ===================
   标签样式
   =================== */
:deep(.n-tag) {
  border-radius: var(--radius-md);
  padding: 6px 14px;
  font-weight: 600;
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.3px;
  border: none;
}

:deep(.n-tag--success) {
  background: var(--success-light);
  color: var(--success-color);
}

:deep(.n-tag--default) {
  background: var(--gray-200);
  color: var(--gray-600);
}

/* ===================
   按钮组
   =================== */
.action-buttons {
  display: flex;
  gap: 8px;
}

.action-buttons :deep(.n-button) {
  min-width: auto;
  padding: 6px 14px;
  height: 32px;
  font-size: 13px;
  font-weight: 500;
  border-radius: var(--radius-md);
  transition: all 200ms cubic-bezier(0.4, 0, 0.2, 1);
  color: #1f2937;
}

.action-buttons :deep(.n-button--error) {
  background: var(--error-color);
  border-color: var(--error-color);
  color: white;
}

.action-buttons :deep(.n-button--error:hover) {
  background: #dc2626;
  border-color: #dc2626;
  box-shadow: 0 4px 12px rgba(239, 68, 68, 0.3);
  color: white;
}

/* ===================
   模态框样式
   =================== */
:deep(.n-modal) {
  border-radius: var(--radius-xl);
  box-shadow: 0 25px 50px rgba(0, 0, 0, 0.25);
}

:deep(.n-modal .n-card) {
  margin-bottom: 0;
}

/* ===================
   响应式设计
   =================== */
@media (max-width: 768px) {
  .action-bar {
    flex-direction: column;
  }

  .action-bar :deep(.n-button) {
    width: 100%;
  }

  :deep(.n-descriptions .n-descriptions-th) {
    width: 120px;
  }

  :deep(.n-data-table .n-data-table-th),
  :deep(.n-data-table .n-data-table-td) {
    padding: 10px 8px;
    font-size: 13px;
  }
}
</style>
