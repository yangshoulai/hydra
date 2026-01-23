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
                :columns="createColumns(group.channel_id)"
                :data="group.models"
                :bordered="true"
                :single-line="false"
                size="small"
                :pagination="false"
                :row-key="(row) => row.config_id"
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
import {
  type DataTableColumns,
  NAlert,
  NButton,
  NCard,
  NDataTable,
  NDrawer,
  NDrawerContent,
  NIcon,
  NPopconfirm,
  NSpace,
  NSpin,
  NTag,
  NText,
  useMessage
} from 'naive-ui'
import {CheckmarkCircleOutline, CloseCircleOutline, InformationCircleOutline, PlayCircleOutline} from '@vicons/ionicons5'
import {channelApi} from '../services/channelService'
import EndpointTags from './EndpointTags.vue'

interface ModelConfig {
  id: number
  config_id: number
  config_status: string
  upstream_model: string
  endpoint_types: string[]
  status: 'disabled' | 'active' | 'non_exist'
}

interface ChannelInfo {
  config_id: number
  config_status: 'disabled' | 'active' | 'non_exist'
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
const message = useMessage()

const loading = ref(false)
const channels = ref<ChannelInfo[]>([])
const groupedChannels = ref<ChannelGroup[]>([])
const testingConfigs = ref<Set<number>>(new Set())

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
      id: channel.config_id,
      config_id: channel.config_id,
      config_status: channel.config_status,
      upstream_model: channel.upstream_model,
      endpoint_types: channel.endpoint_types,
      status: channel.config_status
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
    message.error('加载渠道列表失败')
  } finally {
    loading.value = false
  }
}

async function handleToggleStatus(row: ModelConfig) {
  try {
    await channelApi.toggleChannelModelStatus(row.config_id)
    message.success(`已${row.status === 'active' ? '禁用' : '启用'}该模型配置`)
    await loadChannels()
  } catch (error: any) {
    console.error('Failed to toggle status:', error)
    message.error('切换状态失败')
  }
}

async function handleTest(row: ModelConfig, channelId: number) {
  const configId = row.config_id
  testingConfigs.value.add(configId)

  try {
    // 对每个端点类型进行测试
    const testPromises = row.endpoint_types.map(async (endpointType) => {
      try {
        await channelApi.testModel(channelId, row.upstream_model, props.modelName, endpointType)
        return {endpointType, success: true}
      } catch (error) {
        return {endpointType, success: false, error}
      }
    })

    const results = await Promise.all(testPromises)
    const allSuccess = results.every(r => r.success)
    const failedEndpoints = results.filter(r => !r.success).map(r => r.endpointType)

    if (allSuccess) {
      message.success('所有端点类型测试通过')
    } else {
      message.error(`以下端点类型测试失败: ${failedEndpoints.join(', ')}`)
    }
  } catch (error: any) {
    console.error('Failed to test model:', error)
    message.error('测试失败')
  } finally {
    testingConfigs.value.delete(configId)
  }
}

watch(() => props.show, (newVal) => {
  if (newVal) {
    loadChannels()
  }
})

function createColumns(channelId: number): DataTableColumns<ModelConfig> {
  return [
    {
      title: '配置 ID',
      key: 'config_id',
      width: 80
    },
    {
      title: '上游模型',
      key: 'upstream_model',
      width: 200
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
      width: 80,
      align: 'center',
      render: (row: ModelConfig) => {
        const statusConfig = {
          active: {type: 'success' as const, text: '启用'},
          disabled: {type: 'default' as const, text: '禁用'},
          non_exist: {type: 'error' as const, text: '失效'}
        }
        const config = statusConfig[row.status] || statusConfig.disabled
        return h(
            NTag,
            {
              type: config.type,
              size: 'small',
              bordered: false
            },
            {default: () => config.text}
        )
      }
    },
    {
      title: '操作',
      key: 'actions',
      width: 120,
      align: 'center',
      render: (row: ModelConfig) => {
        const isTesting = testingConfigs.value.has(row.config_id)

        return h(NSpace, {size: 8, justify: 'center'}, {
          default: () => row.status === 'non_exist' ? [] : [
            // 测试按钮
            h(
                NButton,
                {
                  size: 'tiny',
                  type: 'info',
                  secondary: true,
                  loading: isTesting,
                  onClick: () => handleTest(row, channelId)
                },
                {
                  default: () => '测试',
                  icon: () => h(NIcon, null, {default: () => h(PlayCircleOutline)})
                }
            ),
            // 启用/禁用按钮
            h(
                NPopconfirm,
                {
                  onPositiveClick: () => handleToggleStatus(row)
                },
                {
                  default: () => `确定要${row.status === 'active' ? '禁用' : '启用'}该模型配置吗？`,
                  trigger: () => h(
                      NButton,
                      {
                        size: 'tiny',
                        type: row.status === 'active' ? 'warning' : 'success',
                        secondary: true
                      },
                      {
                        default: () => row.status === 'active' ? '禁用' : '启用',
                        icon: () => h(NIcon, null, {
                          default: () => row.status === 'active'
                              ? h(CloseCircleOutline)
                              : h(CheckmarkCircleOutline)
                        })
                      }
                  )
                }
            )
          ]
        })
      }
    }
  ]
}
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
