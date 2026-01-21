<template>
  <n-drawer v-model:show="visible" :width="1000" placement="right" class="model-channels-drawer">
    <n-drawer-content :title="title" closable>
      <template v-if="!loading">
        <n-space vertical :size="16">
          <n-alert v-if="groupedChannels.length === 0" type="info" :bordered="false">
            <template #icon>
              <n-icon>
                <InformationCircleOutline/>
              </n-icon>
            </template>
            该模型暂未关联任何渠道
          </n-alert>

          <n-card
              v-for="group in groupedChannels"
              :key="group.channel_id"
              size="small"
              :bordered="true"
          >
            <template #header>
              <n-space align="center" justify="space-between">
                <n-space align="center">
                  <n-text strong>{{ group.channel_name }}</n-text>
                  <n-tag :type="group.channel_status === 'active' ? 'success' : 'warning'" size="small" :bordered="false">
                    {{ group.channel_status === 'active' ? '启用' : '禁用' }}
                  </n-tag>
                  <n-text depth="3" style="font-size: 12px;">ID: {{ group.channel_id }}</n-text>
                </n-space>
                <n-tag size="small" :bordered="true" type="info">
                  {{ group.models.length }} 个模型
                </n-tag>
              </n-space>
            </template>

            <n-data-table
                :columns="columns"
                :data="group.models"
                :bordered="true"
                :single-line="false"
                size="small"
                :pagination="false"
                :row-key="(row) => row.id"
            />
          </n-card>
        </n-space>
      </template>

      <n-space v-else justify="center" style="padding: 40px 0">
        <n-spin size="medium"/>
      </n-space>
    </n-drawer-content>
  </n-drawer>
</template>

<script setup lang="ts">
import {computed, h, ref, watch} from 'vue'
import {type DataTableColumns, NAlert, NCard, NDataTable, NDrawer, NDrawerContent, NIcon, NSpace, NSpin, NTag, NText} from 'naive-ui'
import {InformationCircleOutline} from '@vicons/ionicons5'
import {channelApi} from '../services/channelService'
import EndpointTags from './EndpointTags.vue'

interface ModelConfig {
  id: number
  upstream_model: string
  endpoint_types: string[]
  status: string
}

interface ChannelInfo {
  channel_id: number
  channel_name: string
  channel_status: string
  upstream_model: string
  endpoint_types: string[]
}

interface ChannelGroup {
  channel_id: number
  channel_name: string
  channel_status: string
  models: ModelConfig[]
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
const groupedChannels = ref<ChannelGroup[]>([])

const visible = computed({
  get: () => props.show,
  set: (value) => emit('update:show', value)
})

const title = computed(() => {
  return `模型渠道列表 - ${props.modelName}`
})

function groupChannelsByChannel() {
  const groupMap = new Map<number, ChannelGroup>()

  channels.value.forEach((channel) => {
    const groupId = channel.channel_id
    const modelConfig: ModelConfig = {
      id: channel.channel_id, // 使用 channel_id 作为唯一标识
      upstream_model: channel.upstream_model,
      endpoint_types: channel.endpoint_types,
      status: 'active' // 默认为启用，如果有状态字段可以修改
    }

    if (!groupMap.has(groupId)) {
      groupMap.set(groupId, {
        channel_id: groupId,
        channel_name: channel.channel_name,
        channel_status: channel.channel_status,
        models: []
      })
    }

    groupMap.get(groupId)!.models.push(modelConfig)
  })

  groupedChannels.value = Array.from(groupMap.values())
}

async function loadChannels() {
  if (!props.modelId) return

  loading.value = true
  try {
    const channelsData = await channelApi.getChannelsByModel(props.modelId)
    channels.value = channelsData
    groupChannelsByChannel()
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

const columns: DataTableColumns<ModelConfig> = [
  {
    title: '上游模型',
    key: 'upstream_model',
    width: 240
  },
  {
    title: '端点类型',
    key: 'endpoint_types',
    width: 200,
    render: (row: ModelConfig) => {
      return h(EndpointTags, {types: row.endpoint_types})
    }
  },
  {
    title: '状态',
    key: 'status',
    width: 120,
    align: 'center',
    render: (row: ModelConfig) => {
      return h(
          NTag,
          {
            type: row.status === 'active' ? 'success' : 'default',
            size: 'small',
            bordered: false
          },
          {default: () => (row.status === 'active' ? '启用' : '禁用')}
      )
    }
  }
]
</script>

<style scoped>
.model-channels-drawer :deep(.n-drawer-content) {
  padding: 0;
}

.model-channels-drawer :deep(.n-drawer-header__main) {
  padding: 20px 24px;
  border-bottom: 1px solid #e2e8f0;
}

.model-channels-drawer :deep(.n-card) {
  margin-bottom: 0;
}

.model-channels-drawer :deep(.n-card__header) {
  padding: 12px 16px;
  border-bottom: 1px solid #e5e7eb;
  background: #f8fafc;
}

.model-channels-drawer :deep(.n-card__content) {
  padding: 0;
}

:deep(.n-data-table) {
  font-size: 13px;
}

:deep(.n-data-table-th) {
  background: #f1f5f9;
  font-weight: 600;
  color: #475569;
}

:deep(.n-data-table-td) {
  padding: 8px 16px;
}

:deep(.n-data-table-tr:hover) {
  background: #f8fafc;
}
</style>
