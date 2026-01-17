<template>
  <n-drawer v-model:show="visible" :width="900" placement="right" class="model-channels-drawer">
    <n-drawer-content :title="title" closable>
      <template v-if="!loading">
        <n-space vertical :size="16">
          <n-alert v-if="channels.length === 0" type="info" :bordered="false">
            <template #icon>
              <n-icon><InformationCircleOutline /></n-icon>
            </template>
            该模型暂未关联任何渠道
          </n-alert>

          <n-card
            v-for="channel in channels"
            :key="channel.channel_id"
            size="small"
            :bordered="false"
            class="channel-card"
          >
            <template #header>
              <n-space align="center" justify="space-between">
                <n-space align="center">
                  <n-text strong style="font-size: 15px">{{ channel.channel_name }}</n-text>
                  <n-tag :type="channel.channel_status === 'active' ? 'success' : 'default'" size="small" :bordered="false">
                    {{ channel.channel_status === 'active' ? '启用' : '禁用' }}
                  </n-tag>
                </n-space>
              </n-space>
            </template>

            <n-descriptions bordered :column="2" size="small">
              <n-descriptions-item label="渠道ID">
                <n-text strong>#{{ channel.channel_id }}</n-text>
              </n-descriptions-item>

              <n-descriptions-item label="上游模型">
                <n-text code style="font-size: 12px">{{ channel.upstream_model }}</n-text>
              </n-descriptions-item>

              <n-descriptions-item label="端点类型" :span="2">
                <n-space :size="4">
                  <n-tag
                    v-for="type in channel.endpoint_types"
                    :key="type"
                    :type="getEndpointTypeColor(type)"
                    size="small"
                  >
                    {{ getEndpointTypeLabel(type) }}
                  </n-tag>
                </n-space>
              </n-descriptions-item>
            </n-descriptions>
          </n-card>
        </n-space>
      </template>

      <n-space v-else justify="center" style="padding: 40px 0">
        <n-spin size="medium" />
      </n-space>
    </n-drawer-content>
  </n-drawer>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  NDrawer,
  NDrawerContent,
  NCard,
  NDescriptions,
  NDescriptionsItem,
  NSpace,
  NText,
  NTag,
  NAlert,
  NIcon,
  NSpin
} from 'naive-ui'
import { InformationCircleOutline } from '@vicons/ionicons5'
import { channelApi } from '../services/channelService'

interface ChannelInfo {
  channel_id: number
  channel_name: string
  channel_status: string
  upstream_model: string
  endpoint_types: string[]
}

interface Props {
  modelId: number
  modelName: string
  show: boolean
}

interface Emits {
  (e: 'update:show', value: boolean): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const loading = ref(false)
const channels = ref<ChannelInfo[]>([])

const visible = computed({
  get: () => props.show,
  set: (value) => emit('update:show', value)
})

const title = computed(() => {
  return `模型渠道列表 - ${props.modelName}`
})

function getEndpointTypeLabel(type: string) {
  const labels: Record<string, string> = {
    'openai': 'OpenAI',
    'openai-response': 'Response',
    'anthropic': 'Anthropic'
  }
  return labels[type] || type
}

function getEndpointTypeColor(type: string) {
  const colors: Record<string, any> = {
    'openai': 'success',
    'openai-response': 'info',
    'anthropic': 'warning'
  }
  return colors[type] || 'default'
}

async function loadChannels() {
  if (!props.modelId) return

  loading.value = true
  try {
    const channelsData = await channelApi.getChannelsByModel(props.modelId)
    channels.value = channelsData
  } catch (error: any) {
    console.error('Failed to load channels:', error)
    window.$message?.error('加载渠道列表失败')
  } finally {
    loading.value = false
  }
}

watch(() => props.show, (newVal) => {
  if (newVal) {
    loadChannels()
  }
})
</script>

<style scoped>
.model-channels-drawer :deep(.n-drawer-content) {
  padding: 0;
}

.model-channels-drawer :deep(.n-drawer-header__main) {
  padding: 20px 24px;
  border-bottom: 1px solid #e2e8f0;
}

.channel-card {
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  transition: all 0.2s ease;
}

.channel-card:hover {
  border-color: #cbd5e1;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
}

:deep(.n-card__header) {
  font-weight: 600;
  font-size: 14px;
  color: #1e293b;
  border-bottom: 1px solid #e2e8f0;
  padding: 12px 16px;
  background: #f1f5f9;
}

:deep(.n-card__content) {
  padding: 16px;
}

:deep(.n-descriptions) {
  font-size: 13px;
}

:deep(.n-descriptions-table-content__label) {
  font-weight: 500;
  color: #64748b;
  background: #f8fafc;
}

:deep(.n-descriptions-table-content__content) {
  background: #ffffff;
}
</style>
