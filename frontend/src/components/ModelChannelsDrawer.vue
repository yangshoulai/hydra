<template>
  <n-drawer v-model:show="visible" :width="drawerWidth" placement="right" class="model-channels-drawer">
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

    <ImageTestDialog
      v-model:show="showImageTestDialog"
      :initial-prompt="imageTestInitialPrompt"
      :initial-text-prompt="imageTestInitialTextPrompt"
      :initial-generation-prompt="imageTestInitialGenerationPrompt"
      :initial-edit-prompt="imageTestInitialEditPrompt"
      :mode="imageTestMode"
      :endpoint-types="imageTestEndpointTypes"
      :loading="pendingImageTestContext ? testingConfigs.has(pendingImageTestContext.row.config_id) : false"
      @submit="handleImageTestSubmit"
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
  NInputNumber,
  NPopconfirm,
  NSpace,
  NSpin,
  NTag,
  NTooltip,
  useMessage,
} from 'naive-ui'
import {CheckmarkCircleOutline, CloseCircleOutline, PlayCircleOutline} from '@vicons/ionicons5'
import {channelApi} from '../services/channelService'
import ModelTestResultDialog from './ModelTestResultDialog.vue'
import ImageTestDialog from './ImageTestDialog.vue'
import EndpointTags from './EndpointTags.vue'
import type {ModelTestResultItem} from '../types/modelTest'
import type {ModelRelatedChannelInfo} from '../types/channel'
import {createModelTestResultItem} from '../utils/modelTest'
import {getErrorMessage, toastApiError} from '@/utils/error'

interface ModelConfig {
  id: number
  config_id: number
  config_status: string
  weight: number
  channel_model: string
  endpoint_types: string[]
  status: 'active' | 'inactive'
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
const channels = ref<ModelRelatedChannelInfo[]>([])
const groupedChannels = ref<ChannelGroup[]>([])
const testingConfigs = ref<Set<number>>(new Set())
const savingWeightConfigs = ref<Set<number>>(new Set())
const weightEditMap = ref<Record<number, number>>({})
const showTestResultDialog = ref(false)
const testResultTitle = ref('模型测试结果')
const testResultItems = ref<ModelTestResultItem[]>([])
const showImageTestDialog = ref(false)
const pendingImageTestContext = ref<{ row: ModelConfig; channelId: number } | null>(null)
const imageTestMode = ref<'generation' | 'edit' | 'mixed'>('edit')
const imageTestEndpointTypes = ref<string[]>([])
const imageTestInitialPrompt = ref('')
const imageTestInitialTextPrompt = ref('')
const imageTestInitialGenerationPrompt = ref('')
const imageTestInitialEditPrompt = ref('')
const weightSaveTimers = new Map<number, number>()
const drawerWidth = 'min(1080px, calc(100vw - 32px))'
const IMAGE_GENERATION_DEFAULT_PROMPT = '请生成一只戴着耳机的柯基'
const IMAGE_EDIT_DEFAULT_PROMPT = '请将图片中的背景替换为星空，并保持主体不变'

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
      weight: channel.weight,
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

  groupMap.forEach((group) => {
    group.models.sort((a, b) => {
      if (b.weight !== a.weight) {
        return b.weight - a.weight
      }
      return a.channel_model.localeCompare(b.channel_model)
    })
  })

  groupedChannels.value = Array.from(groupMap.values())
}

async function loadChannels() {
  if (!props.modelId) return

  loading.value = true
  try {
    const channelsData = await channelApi.getChannelsByModel(props.modelId)
    channels.value = channelsData
    weightEditMap.value = Object.fromEntries(channelsData.map((item) => [item.config_id, item.weight]))
    groupChannelsByChannel()
  } catch (err) {
    toastApiError(err, '加载渠道列表失败')
  } finally {
    loading.value = false
  }
}

function getEditableWeight(row: ModelConfig) {
  return weightEditMap.value[row.config_id] ?? row.weight
}

function isWeightDirty(row: ModelConfig) {
  const matchedChannel = channels.value.find((item) => item.config_id === row.config_id)
  if (!matchedChannel) {
    return false
  }
  return getEditableWeight(row) !== matchedChannel.weight
}

function clearWeightSaveTimer(configId: number) {
  const timerId = weightSaveTimers.get(configId)
  if (timerId) {
    window.clearTimeout(timerId)
    weightSaveTimers.delete(configId)
  }
}

function scheduleWeightSave(configId: number) {
  clearWeightSaveTimer(configId)
  const timerId = window.setTimeout(() => {
    weightSaveTimers.delete(configId)
    void persistWeight(configId)
  }, 450)
  weightSaveTimers.set(configId, timerId)
}

function handleWeightDraftChange(row: ModelConfig, value: number | null) {
  const matchedChannel = channels.value.find((item) => item.config_id === row.config_id)
  const fallbackWeight = matchedChannel?.weight ?? row.weight
  weightEditMap.value[row.config_id] = value && value > 0 ? value : fallbackWeight

  if (weightEditMap.value[row.config_id] === fallbackWeight) {
    clearWeightSaveTimer(row.config_id)
    return
  }

  scheduleWeightSave(row.config_id)
}

function flushWeightSave(configId: number) {
  clearWeightSaveTimer(configId)
  void persistWeight(configId)
}

async function persistWeight(configId: number) {
  const matchedChannel = channels.value.find((item) => item.config_id === configId)
  const nextWeight = weightEditMap.value[configId]
  if (!matchedChannel || !nextWeight || nextWeight === matchedChannel.weight) {
    return
  }

  if (savingWeightConfigs.value.has(configId)) {
    scheduleWeightSave(configId)
    return
  }

  savingWeightConfigs.value.add(configId)
  let saved = false
  try {
    await channelApi.updateModelConfig(configId, { weight: nextWeight })
    matchedChannel.weight = nextWeight
    weightEditMap.value[configId] = nextWeight
    groupChannelsByChannel()
    saved = true
  } catch (err) {
    toastApiError(err, '更新权重失败')
  } finally {
    savingWeightConfigs.value.delete(configId)
    const latestWeight = weightEditMap.value[configId]
    if (saved && latestWeight && latestWeight !== matchedChannel.weight) {
      scheduleWeightSave(configId)
    }
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
  const endpointTypes = row.endpoint_types || []
  const includesImageGeneration = endpointTypes.includes('OpenAIImagesGenerations')
  const includesImageEdit = endpointTypes.includes('OpenAIImagesEdits')
  if (includesImageGeneration || includesImageEdit) {
    pendingImageTestContext.value = {row, channelId}
    imageTestMode.value = includesImageGeneration && includesImageEdit
      ? 'mixed'
      : includesImageGeneration
        ? 'generation'
        : 'edit'
    imageTestEndpointTypes.value = [...endpointTypes]
    imageTestInitialPrompt.value = ''
    imageTestInitialTextPrompt.value = ''
    imageTestInitialGenerationPrompt.value = IMAGE_GENERATION_DEFAULT_PROMPT
    imageTestInitialEditPrompt.value = IMAGE_EDIT_DEFAULT_PROMPT
    showImageTestDialog.value = true
    return
  }

  await executeChannelModelTest(row, channelId)
}

async function handleImageTestSubmit(payload: {
  prompt: string
  textPrompt: string
  generationPrompt: string
  editPrompt: string
  imageData: string
  size: string
  quality: string
}) {
  const context = pendingImageTestContext.value
  if (!context) return
  showImageTestDialog.value = false
  await executeChannelModelTest(context.row, context.channelId, {
    prompt: payload.prompt,
    textPrompt: payload.textPrompt,
    imageGenerationPrompt: payload.generationPrompt,
    imageEditPrompt: payload.editPrompt,
    imageData: payload.imageData,
    imageSize: payload.size,
    imageQuality: payload.quality,
  })
  pendingImageTestContext.value = null
}

async function executeChannelModelTest(
  row: ModelConfig,
  channelId: number,
  options?: {
    prompt?: string
    textPrompt?: string
    imageGenerationPrompt?: string
    imageEditPrompt?: string
    imageData?: string
    imageSize?: string
    imageQuality?: string
  },
) {
  const configId = row.config_id
  testingConfigs.value.add(configId)

  try {
    const channelName = groupedChannels.value.find((group) => group.channel_id === channelId)?.channel_name
    const resultItems = await Promise.all(row.endpoint_types.map(async (endpointType, index) => {
      try {
        const isImageEndpoint = endpointType === 'OpenAIImagesGenerations' || endpointType === 'OpenAIImagesEdits'
        const result = await channelApi.testModel(
          channelId,
          row.channel_model,
          props.modelName,
          endpointType,
          undefined,
          {
            testPrompt: endpointType === 'OpenAIImagesGenerations'
              ? (options?.imageGenerationPrompt || options?.prompt)
              : endpointType === 'OpenAIImagesEdits'
                ? (options?.imageEditPrompt || options?.prompt)
                : options?.textPrompt,
            imageData: endpointType === 'OpenAIImagesEdits' ? (options?.imageData || '') : '',
            imageSize: isImageEndpoint ? options?.imageSize : undefined,
            imageQuality: isImageEndpoint ? options?.imageQuality : undefined,
          },
        )
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
  savingWeightConfigs.value.clear()
  weightSaveTimers.forEach((timerId) => window.clearTimeout(timerId))
  weightSaveTimers.clear()
  weightEditMap.value = {}
})

function createColumns(channelId: number): DataTableColumns<ModelConfig> {
  return [
    {
      title: '配置 ID',
      key: 'config_id',
      width: 80,
      align: 'right',
    },
    {
      title: '渠道模型',
      key: 'channel_model',
      width: 160,
      ellipsis: {tooltip: true},
    },
    {
      title: '端点类型',
      key: 'endpoint_types',
      minWidth: 160,
      render: (row: ModelConfig) => {
        return h(EndpointTags, {types: row.endpoint_types})
      }
    },
    {
      title: '权重',
      key: 'weight',
      width: 120,
      align: 'center',
      render: (row: ModelConfig) => {
        const isSaving = savingWeightConfigs.value.has(row.config_id)

        return h('div', {class: ['weight-editor', isSaving ? 'weight-editor--saving' : '', isWeightDirty(row) ? 'weight-editor--dirty' : '']}, [
          h(NInputNumber, {
            value: getEditableWeight(row),
            size: 'small',
            min: 1,
            max: 1000,
            disabled: isSaving,
            class: 'weight-editor__input',
            buttonPlacement: 'both',
            style: {width: '100%'},
            placeholder: '1-1000',
            onUpdateValue: (value: number | null) => handleWeightDraftChange(row, value),
            onBlur: () => flushWeightSave(row.config_id),
          }),
        ])
      }
    },
    {
      title: '状态',
      key: 'status',
      width: 100,
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
      width: 100,
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

.weight-editor {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 34px;
  width: 100%;
}

.weight-editor__input {
  width: 128px;
}

.weight-editor--saving :deep(.n-input-number) {
  opacity: 0.72;
}

.weight-editor--dirty :deep(.n-input-number) {
  box-shadow: 0 0 0 1px rgba(17, 17, 17, 0.12);
  border-radius: 999px;
}

.weight-editor :deep(.n-input-number) {
  border-radius: 999px;
  overflow: hidden;
}

.weight-editor :deep(.n-input-number .n-input__input-el) {
  text-align: center;
  font-variant-numeric: tabular-nums;
}

.weight-editor :deep(.n-input-number-button) {
  width: 30px;
  min-width: 30px;
}
</style>
