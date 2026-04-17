<template>
  <n-drawer v-model:show="visible" :width="960" placement="right" class="model-channels-drawer">
    <n-drawer-content :title="title" closable>
      <template v-if="!loading">
        <n-space vertical :size="12">
          <n-empty v-if="groupedChannels.length === 0" description="该模型暂未关联任何渠道" />

          <section
            v-for="group in groupedChannels"
            :key="group.channel_id"
            class="panel-card"
          >
            <header class="panel-card__header">
              <div class="channel-group__title">
                <span class="channel-group__name">{{ group.channel_name }}</span>
                <n-tag
                  :type="group.channel_status === 'active' ? 'success' : 'default'"
                  size="small"
                  :bordered="false"
                >
                  {{ group.channel_status === 'active' ? '启用' : '停用' }}
                </n-tag>
                <span class="muted">ID: {{ group.channel_id }}</span>
              </div>
              <span class="muted">共 {{ group.models.length }} 个模型</span>
            </header>

            <div class="panel-card__body">
              <n-data-table
                :columns="createColumns(group.channel_id)"
                :data="group.models"
                :single-line="false"
                :pagination="false"
                :scroll-x="680"
                :row-key="(row) => row.config_id"
              />
            </div>
          </section>
        </n-space>
      </template>

      <n-space v-else justify="center" style="padding: 40px 0">
        <n-spin size="medium"/>
      </n-space>
    </n-drawer-content>

    <ModelTestResultDialog
      v-model:show="showTestResultDialog"
      :title="testResultTitle"
      :items="testResultItems"
    />
  </n-drawer>
</template>

<script setup lang="ts">
import {computed, h, ref, watch} from 'vue'
import {
  type DataTableColumns,
  NButton,
  NDataTable,
  NDrawer,
  NDrawerContent,
  NEmpty,
  NIcon,
  NPopconfirm,
  NSpace,
  NSpin,
  NTag,
  NTooltip,
  useMessage
} from 'naive-ui'
import {CheckmarkCircleOutline, CloseCircleOutline, PlayCircleOutline} from '@vicons/ionicons5'
import {channelApi} from '../services/channelService'
import ModelTestResultDialog from './ModelTestResultDialog.vue'
import EndpointTags from './EndpointTags.vue'
import type {ModelTestResultItem} from '../types/modelTest'
import {createModelTestResultItem} from '../utils/modelTest'
import {getErrorMessage, toastApiError} from '@/utils/error'

interface ModelConfig {
  id: number
  config_id: number
  config_status: string
  channel_model: string
  endpoint_types: string[]
  status: 'active' | 'inactive'
}

interface ChannelInfo {
  config_id: number
  config_status: 'active' | 'inactive'
  channel_id: number
  channel_name: string
  channel_status: 'active' | 'inactive'
  channel_model: string
  endpoint_types: string[]
}

interface ChannelGroup {
  channel_id: number
  channel_name: string
  channel_status: 'active' | 'inactive'
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
const showTestResultDialog = ref(false)
const testResultTitle = ref('模型测试结果')
const testResultItems = ref<ModelTestResultItem[]>([])

const visible = computed({
  get: () => props.show,
  set: (value) => emit('update:show', value)
})

const title = computed(() => {
  return `关联渠道 · ${props.modelName}`
})

function groupChannelsByChannel() {
  const groupMap = new Map<number, ChannelGroup>()

  channels.value.forEach((channel) => {
    const groupId = channel.channel_id
    const modelConfig: ModelConfig = {
      id: channel.config_id,
      config_id: channel.config_id,
      config_status: channel.config_status,
      channel_model: channel.channel_model,
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
  } catch (err) {
    toastApiError(err, '加载渠道列表失败')
  } finally {
    loading.value = false
  }
}

async function handleToggleStatus(row: ModelConfig) {
  try {
    const nextStatus: 'active' | 'inactive' = row.status === 'active' ? 'inactive' : 'active'
    await channelApi.updateModelConfig(row.config_id, {status: nextStatus})
    message.success(`已${nextStatus === 'active' ? '启用' : '停用'}该模型配置`)
    row.status = nextStatus
  } catch (err) {
    toastApiError(err, '切换状态失败')
  }
}

async function handleTest(row: ModelConfig, channelId: number) {
  const configId = row.config_id
  testingConfigs.value.add(configId)

  try {
    const channelName = groupedChannels.value.find((group) => group.channel_id === channelId)?.channel_name
    const resultItems = await Promise.all(row.endpoint_types.map(async (endpointType, index) => {
      try {
        const result = await channelApi.testModel(channelId, row.channel_model, props.modelName, endpointType)
        return createModelTestResultItem({
          id: `${configId}:${endpointType}:${index}`,
          channelName,
          modelName: props.modelName,
          channelModel: row.channel_model,
          endpointType,
          result,
        })
      } catch (err) {
        const errorMessage = getErrorMessage(err, '测试请求失败')
        return createModelTestResultItem({
          id: `${configId}:${endpointType}:${index}`,
          channelName,
          modelName: props.modelName,
          channelModel: row.channel_model,
          endpointType,
          errorMessage,
        })
      }
    }))

    testResultTitle.value = `模型测试结果 · ${channelName || props.modelName} / ${row.channel_model}`
    testResultItems.value = resultItems
    showTestResultDialog.value = true
  } catch (err) {
    toastApiError(err, '测试失败')
  } finally {
    testingConfigs.value.delete(configId)
  }
}

watch(() => props.show, (newVal) => {
  if (newVal) {
    loadChannels()
    return
  }

  showTestResultDialog.value = false
  testResultItems.value = []
})

function createColumns(channelId: number): DataTableColumns<ModelConfig> {
  return [
    {
      title: '配置 ID',
      key: 'config_id',
      width: 90,
      align: 'right',
    },
    {
      title: '渠道模型',
      key: 'channel_model',
      minWidth: 200,
      ellipsis: {tooltip: true},
    },
    {
      title: '端点类型',
      key: 'endpoint_types',
      minWidth: 200,
      render: (row: ModelConfig) => {
        return h(EndpointTags, {types: row.endpoint_types})
      }
    },
    {
      title: '状态',
      key: 'status',
      width: 84,
      align: 'center',
      render: (row: ModelConfig) => {
        const type = row.status === 'active' ? 'success' : 'default'
        const text = row.status === 'active' ? '启用' : '停用'
        return h(
          NTag,
          {type, size: 'small', bordered: false},
          {default: () => text}
        )
      }
    },
    {
      title: '操作',
      key: 'actions',
      width: 110,
      fixed: 'right',
      align: 'center',
      render: (row: ModelConfig) => {
        const isTesting = testingConfigs.value.has(row.config_id)

        return h(NSpace, {size: 4, justify: 'center', class: 'table-action-group'}, {
          default: () => [
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
                      type: 'info',
                      quaternary: true,
                      circle: true,
                      loading: isTesting,
                      'aria-label': `测试配置 ${row.config_id}`,
                      onClick: () => handleTest(row, channelId)
                    },
                    {
                      icon: () => h(NIcon, null, {default: () => h(PlayCircleOutline)})
                    }
                  ),
                default: () => '测试'
              }
            ),
            h(
              NPopconfirm,
              {
                onPositiveClick: () => handleToggleStatus(row)
              },
              {
                default: () => `确定要${row.status === 'active' ? '停用' : '启用'}该模型配置吗？`,
                trigger: () =>
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
                            type: row.status === 'active' ? 'warning' : 'success',
                            quaternary: true,
                            circle: true,
                            'aria-label': `${row.status === 'active' ? '停用' : '启用'}配置 ${row.config_id}`
                          },
                          {
                            icon: () => h(NIcon, null, {
                              default: () => row.status === 'active'
                                ? h(CloseCircleOutline)
                                : h(CheckmarkCircleOutline)
                            })
                          }
                        ),
                      default: () => row.status === 'active' ? '停用' : '启用'
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
.channel-group__title {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.channel-group__name {
  font-size: 13px;
  font-weight: 650;
  color: var(--hydra-text);
}

.muted {
  font-size: 12px;
  color: var(--hydra-text-tertiary);
}
</style>
