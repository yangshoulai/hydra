<template>
  <n-modal
      v-model:show="show"
      preset="card"
      :title="`模型管理 - ${channelName}`"
      style="width: 1400px"
      :mask-closable="false"
      :closable="true"
      @close="handleClose"
      :bordered="false"
      class="model-management-dialog"
  >
    <template #header-extra>
      <n-space class="mr-4">
        <n-button
            type="info"
            @click="handleSyncModels"
            :loading="syncing"
            size="medium"
            secondary
            strong
        >
          <template #icon>
            <n-icon>
              <SyncOutline/>
            </n-icon>
          </template>
          同步上游模型
        </n-button>
        <n-button
            type="primary"
            @click="showAddModelDialog = true"
            size="medium"
            strong
        >
          <template #icon>
            <n-icon>
              <AddOutline/>
            </n-icon>
          </template>
          添加模型
        </n-button>
      </n-space>
    </template>

    <n-space vertical :size="20">
      <!-- 统计信息卡片 -->
      <n-card size="small" :bordered="false" class="stats-card">
        <n-grid :cols="5" :x-gap="20" responsive="screen">
          <n-grid-item>
            <div class="stat-item">
              <div class="stat-icon stat-icon-success">
                <n-icon size="24">
                  <CheckmarkCircleOutline/>
                </n-icon>
              </div>
              <div class="stat-content">
                <div class="stat-label">本地已配置</div>
                <div class="stat-value stat-value-success">{{ stats.localConfigured }}</div>
              </div>
            </div>
          </n-grid-item>
          <n-grid-item>
            <div class="stat-item">
              <div class="stat-icon stat-icon-info">
                <n-icon size="24">
                  <CloudOutline/>
                </n-icon>
              </div>
              <div class="stat-content">
                <div class="stat-label">上游模型总数</div>
                <div class="stat-value stat-value-info">{{ stats.upstreamTotal }}</div>
              </div>
            </div>
          </n-grid-item>
          <n-grid-item>
            <div class="stat-item">
              <div class="stat-icon stat-icon-primary">
                <n-icon size="24">
                  <AddCircleOutline/>
                </n-icon>
              </div>
              <div class="stat-content">
                <div class="stat-label">待添加</div>
                <div class="stat-value stat-value-primary">{{ stats.toAdd }}</div>
              </div>
            </div>
          </n-grid-item>
          <n-grid-item>
            <div class="stat-item">
              <div class="stat-icon stat-icon-warning">
                <n-icon size="24">
                  <RemoveCircleOutline/>
                </n-icon>
              </div>
              <div class="stat-content">
                <div class="stat-label">待删除</div>
                <div class="stat-value stat-value-warning">{{ stats.toRemove }}</div>
              </div>
            </div>
          </n-grid-item>
          <n-grid-item>
            <div class="stat-item">
              <div class="stat-icon stat-icon-default">
                <n-icon size="24">
                  <CheckboxOutline/>
                </n-icon>
              </div>
              <div class="stat-content">
                <div class="stat-label">已选中</div>
                <div class="stat-value">{{ checkedKeys.length }}</div>
              </div>
            </div>
          </n-grid-item>
        </n-grid>
      </n-card>

      <!-- 说明信息 -->
      <n-alert
          v-if="!hasSynced"
          type="info"
          :bordered="false"
          class="info-alert"
      >
        <template #icon>
          <n-icon>
            <InformationCircleOutline/>
          </n-icon>
        </template>
        当前显示本地已配置的模型列表。点击"同步上游模型"按钮可获取渠道的最新模型列表并合并显示。
      </n-alert>

      <n-alert
          v-else-if="syncFailed"
          type="warning"
          :bordered="false"
          class="warning-alert"
      >
        <template #icon>
          <n-icon>
            <WarningOutline/>
          </n-icon>
        </template>
        无法从上游渠道获取模型列表，仅显示本地已配置的模型。
      </n-alert>

      <n-alert
          v-else
          type="success"
          :bordered="false"
          class="success-alert"
      >
        <template #icon>
          <n-icon>
            <CheckmarkCircleOutline/>
          </n-icon>
        </template>
        <n-ul style="margin: 0; padding-left: 20px;">
          <n-li>
            <n-text strong>已配置模型</n-text>
            ：上游和本地都存在的模型，默认选中
          </n-li>
          <n-li>
            <n-text strong>待添加模型</n-text>
            ：仅上游存在的模型，需要手动勾选
          </n-li>
          <n-li>
            <n-text strong>待删除模型</n-text>
            ：仅本地存在的模型，默认选中且禁用（将被删除）
          </n-li>
        </n-ul>
        <n-text depth="3" style="margin-top: 8px; display: block">
          修改统一模型后，点击"保存更改"按钮批量保存选中的模型配置。
        </n-text>
      </n-alert>

      <!-- 模型列表表格 -->
      <n-data-table
          :columns="columns"
          :data="displayModels"
          :loading="loading"
          :pagination="pagination"
          :row-key="(row: ModelDisplayType) => row.key"
          size="small"
          v-model:checked-row-keys="checkedKeys"
          :bordered="true"
          :scroll-x="1260"
      />

      <!-- 操作按钮 -->
      <n-space justify="end" :size="12">
        <n-button @click="handleClose" size="large">取消</n-button>
        <n-button
            type="primary"
            @click="handleSave"
            :loading="saving"
            size="large"
            strong
        >
          <template #icon>
            <n-icon>
              <SaveOutline/>
            </n-icon>
          </template>
          保存更改 ({{ checkedKeys.length }})
        </n-button>
      </n-space>
    </n-space>

    <!-- 添加模型对话框 -->
    <n-modal
        v-model:show="showAddModelDialog"
        preset="card"
        title="添加模型配置"
        style="width: 600px"
        :bordered="false"
        class="add-model-dialog"
    >
      <n-space vertical :size="20">
        <n-alert type="info" :bordered="false">
          <template #icon>
            <n-icon>
              <InformationCircleOutline/>
            </n-icon>
          </template>
          手动添加上游模型配置。请确保上游模型名称正确。
        </n-alert>

        <n-form
            ref="modelFormRef"
            :model="modelForm"
            :rules="modelRules"
            label-placement="top"
            size="large"
        >
          <n-form-item label="上游模型名称" path="upstream_model">
            <n-input
                v-model:value="modelForm.upstream_model"
                placeholder="例如：gpt-4-turbo-preview"
                :input-props="{autocomplete: 'off'}"
            >
              <template #prefix>
                <n-icon>
                  <CloudOutline/>
                </n-icon>
              </template>
            </n-input>
            <template #feedback>
              上游渠道提供的原始模型名称，如 gpt-4、claude-3-opus 等
            </template>
          </n-form-item>

          <n-form-item label="统一模型" path="unified_model">
            <n-select
                v-model:value="modelForm.unified_model"
                :options="modelOptions"
                placeholder="请选择统一模型"
                :loading="loadingModels"
                filterable
                :input-props="{autocomplete: 'off'}"
            >
              <template #prefix>
                <n-icon>
                  <LayersOutline/>
                </n-icon>
              </template>
            </n-select>
            <template #feedback>
              选择系统中的统一模型，用于将不同渠道的相同模型映射到统一名称
            </template>
          </n-form-item>

          <n-form-item label="端点类型" path="endpoint_types">
            <n-select
                v-model:value="modelForm.endpoint_types"
                :options="endpointTypeOptions"
                placeholder="请选择端点类型"
                multiple
                :input-props="{autocomplete: 'off'}"
            />
            <template #feedback>
              选择该模型支持的端点类型，可多选
            </template>
          </n-form-item>

          <n-form-item label="密钥分组" path="key_groups">
            <n-select
                v-model:value="modelForm.key_groups"
                :options="keyGroupOptions"
                placeholder="请选择密钥分组"
                multiple
                :input-props="{autocomplete: 'off'}"
            />
            <template #feedback>
              选择该模型可用的密钥分组
            </template>
          </n-form-item>
        </n-form>
      </n-space>

      <template #footer>
        <n-space justify="end">
          <n-button @click="showAddModelDialog = false" size="large">取消</n-button>
          <n-button type="primary" @click="handleAddModel" size="large" strong>
            <template #icon>
              <n-icon>
                <AddOutline/>
              </n-icon>
            </template>
            确定添加
          </n-button>
        </n-space>
      </template>
    </n-modal>
  </n-modal>
</template>

<script setup lang="ts">
// @ts-nocheck
import {computed, h, nextTick, reactive, ref, watch} from 'vue'
import {
  type DataTableColumns,
  NAlert,
  NButton,
  NCard,
  NDataTable,
  NForm,
  NFormItem,
  NGrid,
  NGridItem,
  NIcon,
  NInput,
  NLi,
  NModal,
  NSelect,
  NSpace,
  NTag,
  NText,
  NUl
} from 'naive-ui'
import {
  AddCircleOutline,
  AddOutline,
  CheckboxOutline,
  CheckmarkCircleOutline,
  CloudOutline,
  InformationCircleOutline,
  LayersOutline,
  PlayCircleOutline,
  RemoveCircleOutline,
  SaveOutline,
  SyncOutline,
  WarningOutline
} from '@vicons/ionicons5'
import {channelApi} from '../services/channelService'
import {modelApi} from '../services/modelService'
import {endpointApi} from '../services/endpointService'
import type {SyncResult} from '../types/channel'
import type {Model} from '../types/model'
import type {EndpointInfo} from '../types/endpoint'

interface Props {
  channelId: number
  channelName: string
  modelValue: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  refresh: []
}>()

const show = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit('update:modelValue', value)
})

// State
const loading = ref(false)
const syncing = ref(false)
const syncRequestId = ref(0)
const saving = ref(false)
const syncFailed = ref(false)
const hasSynced = ref(false)
const showAddModelDialog = ref(false)
const syncResult = ref<SyncResult | null>(null)
const localConfigs = ref<any[]>([])
const unifiedModels = ref<Model[]>([])
const endpoints = ref<EndpointInfo[]>([])
const loadingModels = ref(false)
const testStatus = ref<Record<string, Record<string, 'idle' | 'testing' | 'success' | 'error'>>>({})

// 选中的行
const checkedKeys = ref<string[]>([])

// 编辑状态：key -> unified_model
const editMap = ref<Record<string, string>>({})

// 端点类型编辑状态：key -> endpoint_types
const endpointTypesEditMap = ref<Record<string, string[]>>({})

// 密钥分组编辑状态：key -> key_groups
const keyGroupsEditMap = ref<Record<string, string[]>>({})

const keyGroupOptions = ref<{ label: string; value: string }[]>([
  {label: 'Default', value: 'Default'}
])

// 表单
const modelForm = reactive({
  upstream_model: '',
  unified_model: '',
  endpoint_types: ['openai'],
  key_groups: ['Default']
})
const modelRules = {
  upstream_model: {
    required: true,
    message: '请输入上游模型名称',
    trigger: ['blur', 'input']
  },
  unified_model: {
    required: true,
    message: '请选择统一模型',
    trigger: ['blur', 'change']
  },
  endpoint_types: {
    required: true,
    type: 'array',
    message: '请选择至少一个端点类型',
    trigger: ['blur', 'change']
  },
  key_groups: {
    required: true,
    type: 'array',
    message: '请选择至少一个密钥分组',
    trigger: ['blur', 'change']
  }
}

// 加载端点列表
async function loadEndpoints() {
  try {
    endpoints.value = await endpointApi.list()
  } catch (error: any) {
    console.error('Failed to load endpoints:', error)
    window.$message?.error('加载端点列表失败')
  }
}

// 端点类型选项
const endpointTypeOptions = computed(() => {
  return endpoints.value.map(ep => ({
    label: `${ep.name}`,
    value: ep.type
  }))
})

// 统一模型下拉选项
const modelOptions = computed(() => {
  return unifiedModels.value.map(model => ({
    label: model.provider
        ? `${model.name} (${model.provider.name})`
        : model.name,
    value: model.name
  }))
})

// 统计信息
const stats = computed(() => {
  if (!hasSynced.value || syncFailed.value) {
    return {
      localConfigured: localConfigs.value.length,
      upstreamTotal: 0,
      toAdd: 0,
      toRemove: 0
    }
  }

  const diff = syncResult.value?.diff
  return {
    localConfigured: diff?.existing_count || 0,
    upstreamTotal: diff?.total_upstream_models || 0,
    toAdd: diff?.added_count || 0,
    toRemove: diff?.removed_count || 0
  }
})

// 模型显示类型
interface ModelDisplayType {
  key: string
  upstream_model: string
  unified_model: string
  endpoint_types: string[]
  key_groups: string[]
  status: 'configured' | 'to_add' | 'to_remove'
  disabled: boolean
  channel_status: 'active' | 'disabled' | 'non_exist' | 'unconfigured'
}

// 显示的模型列表
const displayModels = computed<ModelDisplayType[]>(() => {
  if (!hasSynced.value || syncFailed.value || !syncResult.value) {
    // 仅显示本地配置
    return localConfigs.value.map(config => ({
      key: config.upstream_model,
      upstream_model: config.upstream_model,
      unified_model: config.unified_model,
      endpoint_types: config.endpoint_types || ['openai'],
      key_groups: config.key_groups || ['Default'],
      status: 'configured' as const,
      disabled: false,
      channel_status: config.status || 'unconfigured'
    }))
  }

  // 有同步结果，合并显示
  const models: ModelDisplayType[] = []
  const diff = syncResult.value.diff

  // ❌ 不要在计算属性中调用 initEditMap()，会导致无限循环
  // initEditMap() 应该在数据变化时主动调用

  // 安全检查：防止 diff 为 null
  if (!diff || !diff.diffs) {
    console.warn('[displayModels] diff or diff.diffs is null/undefined')
    return []
  }

  diff.diffs.forEach((d) => {
    const unifiedModel = editMap.value[d.upstream_model] || d.upstream_model
    let status: 'configured' | 'to_add' | 'to_remove'
    let disabled = false
    let endpointTypes = ['openai']
    let keyGroups = ['Default']
    let channelStatus: 'active' | 'disabled' | 'non_exist' | 'unconfigured' | 'cooling' = 'unconfigured'

    if (d.type === 'existing') {
      status = 'configured'
      disabled = false
      endpointTypes = d.existing_config?.endpoint_types || ['openai']
      keyGroups = d.key_groups || d.existing_config?.key_groups || ['Default']
      channelStatus = d.existing_config?.status || 'unconfigured'
    } else if (d.type === 'added') {
      status = 'to_add'
      disabled = false
      endpointTypes = ['openai']
      keyGroups = d.key_groups || ['Default']
      channelStatus = 'unconfigured'
    } else {
      status = 'to_remove'
      disabled = true // 待删除的模型默认选中且禁用
      endpointTypes = d.existing_config?.endpoint_types || ['openai']
      keyGroups = d.existing_config?.key_groups || ['Default']
      channelStatus = d.existing_config?.status || 'unconfigured'
    }

    models.push({
      key: d.upstream_model,
      upstream_model: d.upstream_model,
      unified_model: unifiedModel,
      endpoint_types: endpointTypes,
      key_groups: keyGroups,
      status,
      disabled,
      channel_status: channelStatus
    })
  })

  // 排序：已配置 > 待删除 > 待添加，同类型按上游模型名称排序
  const statusOrder = {configured: 1, to_remove: 2, to_add: 3}
  models.sort((a, b) => {
    const statusDiff = statusOrder[a.status] - statusOrder[b.status]
    if (statusDiff !== 0) return statusDiff
    return a.upstream_model.localeCompare(b.upstream_model)
  })

  return models
})

// 分页
const pagination = reactive({
  page: 1,
  pageSize: 10,
  showSizePicker: true,
  pageSizes: [10, 20, 50, 100],
  onChange: (page: number) => {
    pagination.page = page
  },
  onUpdatePageSize: (pageSize: number) => {
    pagination.pageSize = pageSize
    pagination.page = 1
  }
})

// 表格列定义
const columns: DataTableColumns<ModelDisplayType> = [
  {
    type: 'selection'
  },
  {
    title: '上游模型',
    key: 'upstream_model',
    width: 200,
    render(row) {
      return h(NText, {code: true}, {default: () => row.upstream_model})
    }
  },
  {
    title: '统一模型',
    key: 'unified_model',
    width: 240,
    render(row) {
      const value = editMap.value[row.key] || row.unified_model
      return h(NSelect, {
        value: value,
        options: modelOptions.value,
        placeholder: '请选择统一模型',
        loading: loadingModels.value,
        filterable: true,
        size: 'small',
        onUpdateValue: (val: string) => {
          editMap.value[row.key] = val
        }
      })
    }
  },
  {
    title: '端点类型',
    key: 'endpoint_types',
    width: 120,
    render(row) {
      const value = endpointTypesEditMap.value[row.key] || row.endpoint_types
      return h(NSelect, {
        value: value,
        options: endpointTypeOptions.value,
        placeholder: '请选择端点类型',
        multiple: true,
        size: 'small',
        onUpdateValue: (val: string[]) => {
          endpointTypesEditMap.value[row.key] = val
        }
      })
    }
  },
  {
    title: '密钥分组',
    key: 'key_groups',
    width: 160,
    render(row) {
      const value = keyGroupsEditMap.value[row.key] || row.key_groups
      return h(NSelect, {
        value: value,
        options: keyGroupOptions.value,
        placeholder: '请选择密钥分组',
        multiple: true,
        size: 'small',
        onUpdateValue: (val: string[]) => {
          keyGroupsEditMap.value[row.key] = val
        }
      })
    }
  },
  {
    title: '状态',
    key: 'status',
    width: 80,
    align: 'center',
    render(row) {
      const statusConfig = {
        configured: {type: 'success' as const, text: '已配置'},
        to_add: {type: 'info' as const, text: '待添加'},
        to_remove: {type: 'warning' as const, text: '待删除'}
      }
      const config = statusConfig[row.status]
      return h(NTag, {type: config.type, size: 'small'}, {default: () => config.text})
    }
  },
  {
    title: '渠道状态',
    key: 'channel_status',
    width: 80,
    align: 'center',
    render(row) {
      const statusConfig = {
        active: {type: 'success' as const, text: '正常'},
        disabled: {type: 'warning' as const, text: '禁用'},
        non_exist: {type: 'error' as const, text: '失效'},
        cooling: {type: 'warning' as const, text: '冷却中'},
        unconfigured: {type: 'default' as const, text: '未配置'}
      }
      const config = statusConfig[row.channel_status] || statusConfig.unconfigured
      return h(NTag, {type: config.type, size: 'small'}, {default: () => config.text})
    }
  },
  {
    title: '测试状态',
    key: 'test_status',
    width: 160,
    fixed: 'right',
    align: 'left',
    render(row) {
      const endpointTypes = endpointTypesEditMap.value[row.key] || row.endpoint_types
      const statusMap = {
        idle: {type: 'default' as const, text: '未测试'},
        testing: {type: 'info' as const, text: '测试中'},
        success: {type: 'success' as const, text: '成功'},
        error: {type: 'error' as const, text: '失败'}
      }

      return h(NSpace, {size: 4, vertical: true}, {
        default: () => endpointTypes.map((type: string) => {
          const status = testStatus.value[row.key]?.[type] || 'idle'
          const config = statusMap[status]
          const typeLabelMap: Record<string, string> = {
            'openai-chat': 'OpenAI Chat',
            'openai-response': 'OpenAI Responses',
            'openai-image': 'OpenAI Images',
            anthropic: 'Anthropic Messages',
            gemini: 'Google Gemini'
          }
          const typeLabel = typeLabelMap[type] || type
          return h(NTag, {type: config.type, size: 'small'}, {default: () => `${typeLabel}: ${config.text}`})
        })
      })
    }
  },
  {
    title: '操作',
    key: 'actions',
    width: 120,
    align: 'center',
    fixed: 'right',
    render(row) {
      const endpointTypes = endpointTypesEditMap.value[row.key] || row.endpoint_types
      const isAnyTesting = endpointTypes.some((type: string) => testStatus.value[row.key]?.[type] === 'testing')

      return h(NButton, {
        size: 'tiny',
        type: 'info',
        loading: isAnyTesting,
        onClick: () => handleTest(row)
      }, {
        icon: () => h(NIcon, null, {default: () => h(PlayCircleOutline)}),
        default: () => '测试'
      })
    }
  }
]

// 初始化编辑状态和选中状态
function initEditMap() {
  editMap.value = {}
  endpointTypesEditMap.value = {}
  keyGroupsEditMap.value = {}
  const defaultChecked: string[] = []

  if (hasSynced.value && !syncFailed.value && syncResult.value) {
    // 有同步结果
    syncResult.value.diff.diffs.forEach((d) => {
      if (d.type === 'existing' && d.existing_config) {
        editMap.value[d.upstream_model] = d.existing_config.unified_model
        endpointTypesEditMap.value[d.upstream_model] = d.existing_config.endpoint_types || ['openai']
        keyGroupsEditMap.value[d.upstream_model] = d.key_groups || d.existing_config.key_groups || ['Default']
        defaultChecked.push(d.upstream_model)
      } else if (d.type === 'added') {
        editMap.value[d.upstream_model] = d.upstream_model
        endpointTypesEditMap.value[d.upstream_model] = ['openai']
        keyGroupsEditMap.value[d.upstream_model] = d.key_groups || ['Default']
        // 新增的模型默认不选中
      } else if (d.type === 'removed') {
        editMap.value[d.upstream_model] = d.existing_config?.unified_model || d.upstream_model
        endpointTypesEditMap.value[d.upstream_model] = d.existing_config?.endpoint_types || ['openai']
        keyGroupsEditMap.value[d.upstream_model] = d.existing_config?.key_groups || ['Default']
        // 删除的模型默认选中
        defaultChecked.push(d.upstream_model)
      }
    })
  } else {
    // 仅本地配置
    localConfigs.value.forEach(config => {
      editMap.value[config.upstream_model] = config.unified_model
      endpointTypesEditMap.value[config.upstream_model] = config.endpoint_types || ['openai']
      keyGroupsEditMap.value[config.upstream_model] = config.key_groups || ['Default']
      defaultChecked.push(config.upstream_model)
    })
  }

  checkedKeys.value = defaultChecked
}

// 加载本地配置
async function loadLocalConfigs() {
  loading.value = true
  try {
    const channel = await channelApi.get(props.channelId)
    localConfigs.value = channel.model_configs || []
    const groups = new Set<string>()
    channel.keys?.forEach((key) => {
      if (key.key_group) {
        groups.add(key.key_group)
      }
    })
    if (groups.size === 0) {
      groups.add('Default')
    }
    keyGroupOptions.value = Array.from(groups).sort().map((group) => ({
      label: group,
      value: group
    }))
  } catch (error: any) {
    console.error('Failed to load local configs:', error)
    window.$message?.error('加载本地配置失败')
  } finally {
    loading.value = false
  }
}

// 加载统一模型列表
async function loadUnifiedModels() {
  loadingModels.value = true
  try {
    const result = await modelApi.list({page: 1, page_size: 1000})
    unifiedModels.value = result.items
  } catch (error: any) {
    console.error('Failed to load unified models:', error)
    window.$message?.error('加载统一模型列表失败')
  } finally {
    loadingModels.value = false
  }
}

// 同步上游模型
async function handleSyncModels() {
  const currentRequestId = syncRequestId.value + 1
  syncRequestId.value = currentRequestId
  const currentChannelId = props.channelId
  syncing.value = true
  syncFailed.value = false

  try {
    const result = await channelApi.syncModels(currentChannelId)

    if (
        currentRequestId !== syncRequestId.value ||
        !props.modelValue ||
        props.channelId !== currentChannelId
    ) {
      return
    }

    syncResult.value = result
    hasSynced.value = true

    // 清空之前的编辑状态，重新初始化
    editMap.value = {}

    // 用 try-catch 包裹初始化逻辑，防止错误导致 loading 无法重置
    try {
      initEditMap()
    } catch (initError) {
      console.error('[ModelManagementDialog] Init edit map failed:', initError)
      // 初始化失败不影响同步成功状态
    }

    window.$message?.success('同步成功')
  } catch (error: any) {
    console.error('[ModelManagementDialog] Sync failed:', error)
    if (
        currentRequestId !== syncRequestId.value ||
        !props.modelValue ||
        props.channelId !== currentChannelId
    ) {
      return
    }
    syncFailed.value = true
    hasSynced.value = true
    syncResult.value = null
    window.$message?.error(error.response?.data?.error || '同步失败，仅显示本地配置')
  } finally {
    // 确保无论发生什么都要重置 loading 状态
    if (currentRequestId === syncRequestId.value) {
      syncing.value = false
    }
  }
}

// 添加模型配置
async function handleAddModel() {
  try {
    await channelApi.createModelConfig(
        props.channelId,
        modelForm.unified_model,
        modelForm.upstream_model,
        modelForm.endpoint_types,
        modelForm.key_groups
    )
    window.$message?.success('添加成功')
    showAddModelDialog.value = false
    modelForm.upstream_model = ''
    modelForm.unified_model = ''
    modelForm.endpoint_types = ['openai']
    modelForm.key_groups = ['Default']
    await loadLocalConfigs()
    initEditMap()
    emit('refresh')
  } catch (error: any) {
    console.error('Failed to add model config:', error)
    window.$message?.error(error.response?.data?.error || '添加失败')
  }
}

// 保存更改
async function handleSave() {
  saving.value = true

  try {
    if (!hasSynced.value || syncFailed.value) {
      // 没有同步或同步失败，只处理本地配置的修改
      const updatePromises: Promise<any>[] = []
      const deletePromises: Promise<void>[] = []

      // 遍历本地配置
      for (const config of localConfigs.value) {
        const isChecked = checkedKeys.value.includes(config.upstream_model)
        const currentUnifiedModel = editMap.value[config.upstream_model]
        const currentEndpointTypes = endpointTypesEditMap.value[config.upstream_model]
        const currentKeyGroups = keyGroupsEditMap.value[config.upstream_model] || config.key_groups || ['Default']

        if (isChecked) {
          // 验证端点类型
          if (!currentEndpointTypes || currentEndpointTypes.length === 0) {
            window.$message?.error(`模型 "${config.upstream_model}" 必须选择至少一个端点类型`)
            saving.value = false
            return
          }
          if (!currentKeyGroups || currentKeyGroups.length === 0) {
            window.$message?.error(`模型 "${config.upstream_model}" 必须选择至少一个密钥分组`)
            saving.value = false
            return
          }
        }

        if (!isChecked) {
          // 取消选中，删除配置
          deletePromises.push(channelApi.deleteModelConfig(config.id))
        } else if (currentUnifiedModel && currentUnifiedModel !== config.unified_model) {
          // 选中且统一模型有修改，更新配置
          updatePromises.push(
              channelApi.updateModelConfig(config.id, {
                unified_model: currentUnifiedModel,
                endpoint_types: currentEndpointTypes,
                key_groups: currentKeyGroups
              })
          )
        } else if (
            currentEndpointTypes && JSON.stringify(currentEndpointTypes) !== JSON.stringify(config.endpoint_types)
        ) {
          // 选中且端点类型有修改，更新配置
          updatePromises.push(
              channelApi.updateModelConfig(config.id, {
                endpoint_types: currentEndpointTypes,
                key_groups: currentKeyGroups
              })
          )
        } else if (
            currentKeyGroups && JSON.stringify(currentKeyGroups) !== JSON.stringify(config.key_groups || ['Default'])
        ) {
          updatePromises.push(
              channelApi.updateModelConfig(config.id, {
                key_groups: currentKeyGroups
              })
          )
        }
      }

      // 执行所有删除和更新操作
      await Promise.all([...deletePromises, ...updatePromises])

      if (deletePromises.length > 0 || updatePromises.length > 0) {
        window.$message?.success('保存成功')
        emit('refresh')
      }
    } else {
      // 有同步结果，使用同步保存逻辑
      // 验证：检查所有选中的模型是否都选择了统一模型和端点类型
      for (const key of checkedKeys.value) {
        const unifiedModel = editMap.value[key]
        const endpointTypes = endpointTypesEditMap.value[key]
        const keyGroups = keyGroupsEditMap.value[key]

        if (!unifiedModel) {
          window.$message?.error(`请为上游模型 ${key} 选择统一模型`)
          saving.value = false
          return
        }

        if (!endpointTypes || endpointTypes.length === 0) {
          window.$message?.error(`请为上游模型 ${key} 选择至少一个端点类型`)
          saving.value = false
          return
        }

        if (!keyGroups || keyGroups.length === 0) {
          window.$message?.error(`请为上游模型 ${key} 选择至少一个密钥分组`)
          saving.value = false
          return
        }
      }

      // 准备数据
      const addModels: any[] = []
      const updateModels: any[] = []
      const deleteModelIDs: number[] = []

      syncResult.value!.diff.diffs.forEach((d) => {
        const isChecked = checkedKeys.value.includes(d.upstream_model)
        const currentUnifiedModel = editMap.value[d.upstream_model]
        const currentEndpointTypes = endpointTypesEditMap.value[d.upstream_model]
        const currentKeyGroups = keyGroupsEditMap.value[d.upstream_model] || d.key_groups || ['Default']
        const existingUnifiedModel = d.existing_config?.unified_model
        const existingEndpointTypes = d.existing_config?.endpoint_types
        const existingKeyGroups = d.existing_config?.key_groups || ['Default']

        if (d.type === 'existing') {
          // 已配置的模型
          if (isChecked) {
            // 选中了，检查是否需要更新
            if (
                currentUnifiedModel !== existingUnifiedModel
                || JSON.stringify(currentEndpointTypes) !== JSON.stringify(existingEndpointTypes)
                || JSON.stringify(currentKeyGroups) !== JSON.stringify(existingKeyGroups)
            ) {
              updateModels.push({
                id: d.existing_config!.id,
                unified_model: currentUnifiedModel,
                upstream_model: d.upstream_model,
                endpoint_types: currentEndpointTypes,
                key_groups: currentKeyGroups
              })
            }
          } else {
            // 没选中，删除
            deleteModelIDs.push(d.existing_config!.id)
          }
        } else if (d.type === 'added') {
          // 新增模型
          if (isChecked) {
            addModels.push({
              unified_model: currentUnifiedModel,
              upstream_model: d.upstream_model,
              endpoint_types: currentEndpointTypes,
              key_groups: currentKeyGroups
            })
          }
        } else if (d.type === 'removed') {
          // 删除模型（默认选中且禁用）
          if (isChecked && d.existing_config) {
            deleteModelIDs.push(d.existing_config.id)
          }
        }
      })

      // 调用 API
      await channelApi.applySync(
          props.channelId,
          {
            add_models: addModels,
            update_models: updateModels,
            delete_model_ids: deleteModelIDs
          }
      )

      window.$message?.success('保存成功')
      emit('refresh')
    }

    handleClose()
  } catch (error: any) {
    console.error('Failed to save:', error)
    window.$message?.error(error.response?.data?.error || '保存失败')
  } finally {
    saving.value = false
  }
}

// 测试模型
async function handleTest(row: ModelDisplayType) {
  // 获取当前选择的端点类型
  const endpointTypes = endpointTypesEditMap.value[row.key] || row.endpoint_types
  const keyGroups = keyGroupsEditMap.value[row.key] || row.key_groups

  // 验证是否选择了端点类型
  if (!endpointTypes || endpointTypes.length === 0) {
    window.$message?.error('请先选择至少一个端点类型')
    return
  }
  if (!keyGroups || keyGroups.length === 0) {
    window.$message?.error('请先选择至少一个密钥分组')
    return
  }

  // 获取当前选择的统一模型
  const unifiedModel = editMap.value[row.key] || row.unified_model

  // 初始化测试状态
  if (!testStatus.value[row.key]) {
    testStatus.value[row.key] = {}
  }

  // 为所有端点类型设置测试中状态
  endpointTypes.forEach((type: string) => {
    testStatus.value[row.key][type] = 'testing'
  })

  // 测试所有端点类型
  const testPromises = endpointTypes.map(async (endpointType: string) => {
    try {
      const result = await channelApi.testModel(
          props.channelId,
          row.upstream_model,
          unifiedModel,
          endpointType,
          keyGroups
      )

      if (result.success) {
        testStatus.value[row.key][endpointType] = 'success'
      } else {
        testStatus.value[row.key][endpointType] = 'error'
        window.$message?.error(`${endpointType}: ${result.message}`)
      }
    } catch (error: any) {
      console.error(`Failed to test model with ${endpointType}:`, error)
      testStatus.value[row.key][endpointType] = 'error'
      window.$message?.error(`${endpointType}: ${error.response?.data?.error || '测试失败'}`)
    }
  })

  await Promise.all(testPromises)

  // 检查是否所有测试都成功
  const allSuccess = endpointTypes.every((type: string) => testStatus.value[row.key][type] === 'success')
  if (allSuccess) {
    window.$message?.success(`模型 "${row.upstream_model}" 所有端点类型测试成功`)
  }
}

// 关闭对话框
function handleClose() {
  emit('update:modelValue', false)
}

// 监听对话框打开
watch(() => props.modelValue, async (newVal) => {
  if (!newVal) {
    syncRequestId.value += 1
    syncing.value = false
    return
  }
  if (props.channelId > 0) {
    // 重置状态
    syncRequestId.value += 1
    syncing.value = false
    syncResult.value = null
    syncFailed.value = false
    hasSynced.value = false
    editMap.value = {}
    endpointTypesEditMap.value = {}
    keyGroupsEditMap.value = {}
    checkedKeys.value = []
    localConfigs.value = []
    unifiedModels.value = []
    endpoints.value = []
    testStatus.value = {}

    // 重置分页为第一页
    pagination.page = 1

    try {
      // 先加载数据
      await Promise.all([
        loadLocalConfigs(),
        loadUnifiedModels(),
        loadEndpoints()
      ])
      // 数据加载完成后再初始化编辑状态和选中状态
      initEditMap()
      // 等待 Vue 更新视图
      await nextTick()

    } catch (error) {
      console.error('[ModelManagementDialog] Error loading data:', error)
    }
  }
})
</script>

<style scoped>
/* 对话框样式 */
.model-management-dialog {
  --primary-color: #18a058;
  --primary-color-hover: #36ad6a;
  --info-color: #2080f0;
  --warning-color: #f0a020;
  --success-color: #18a058;
  --error-color: #d03050;
}

/* 统计卡片样式 */
.stats-card {
  background: linear-gradient(135deg, #f5f7fa 0%, #ffffff 100%);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
}

.stat-item {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 8px 0;
}

.stat-icon {
  width: 56px;
  height: 56px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.stat-icon-success {
  background: linear-gradient(135deg, #e8f7ef 0%, #d4f1e4 100%);
  color: var(--success-color);
}

.stat-icon-info {
  background: linear-gradient(135deg, #e8f4ff 0%, #d4e9ff 100%);
  color: var(--info-color);
}

.stat-icon-primary {
  background: linear-gradient(135deg, #e8f0ff 0%, #d4e0ff 100%);
  color: var(--primary-color);
}

.stat-icon-warning {
  background: linear-gradient(135deg, #fff7e8 0%, #ffedd4 100%);
  color: var(--warning-color);
}

.stat-icon-default {
  background: linear-gradient(135deg, #f5f5f6 0%, #e8e8e9 100%);
  color: #666;
}

.stat-content {
  flex: 1;
  min-width: 0;
}

.stat-label {
  font-size: 13px;
  color: #666;
  margin-bottom: 4px;
  font-weight: 500;
}

.stat-value {
  font-size: 28px;
  font-weight: 700;
  line-height: 1;
  color: #333;
}

.stat-value-success {
  color: var(--success-color);
}

.stat-value-info {
  color: var(--info-color);
}

.stat-value-primary {
  color: var(--primary-color);
}

.stat-value-warning {
  color: var(--warning-color);
}

/* 提示信息样式 */
.info-alert {
  background: linear-gradient(135deg, #f0f9ff 0%, #e0f2fe 100%);
  border-left: 4px solid var(--info-color);
}

.warning-alert {
  background: linear-gradient(135deg, #fffbeb 0%, #fef3c7 100%);
  border-left: 4px solid var(--warning-color);
}

.success-alert {
  background: linear-gradient(135deg, #f0fdf4 0%, #dcfce7 100%);
  border-left: 4px solid var(--success-color);
}


/* 添加模型对话框样式 */
.add-model-dialog {
  --n-border-radius: 12px;
}

.add-model-dialog :deep(.n-card__content) {
  padding: 24px;
}

.add-model-dialog :deep(.n-input),
.add-model-dialog :deep(.n-base-selection) {
  border-radius: 8px;
  transition: all 0.3s;
}

.add-model-dialog :deep(.n-input:focus),
.add-model-dialog :deep(.n-base-selection:focus) {
  box-shadow: 0 0 0 2px rgba(24, 160, 88, 0.1);
}

.add-model-dialog :deep(.n-form-item-label) {
  font-weight: 600;
  color: #333;
  font-size: 14px;
  padding-bottom: 8px;
}

.add-model-dialog :deep(.n-form-item-feedback) {
  font-size: 12px;
  color: #999;
  margin-top: 4px;
}

/* 按钮样式优化 */
.model-management-dialog :deep(.n-button) {
  transition: all 0.3s;
}

.model-management-dialog :deep(.n-button:hover) {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.model-management-dialog :deep(.n-button--primary) {
  background: linear-gradient(135deg, var(--primary-color) 0%, var(--primary-color-hover) 100%);
  border: none;
}

.model-management-dialog :deep(.n-button--primary:hover) {
  background: linear-gradient(135deg, var(--primary-color-hover) 0%, #40c478 100%);
}

/* 加载动画 */
.model-management-dialog :deep(.n-spin) {
  color: var(--primary-color);
}

/* 响应式调整 */
@media (max-width: 1440px) {
  .model-management-dialog {
    width: 1200px !important;
  }
}

@media (max-width: 1200px) {
  .model-management-dialog {
    width: 95vw !important;
  }

  .stats-card :deep(.n-grid-item) {
    min-width: 50%;
  }
}

/* 标签样式优化 */
.model-management-dialog :deep(.n-tag) {
  border-radius: 6px;
  font-weight: 500;
  padding: 4px 12px;
}
</style>
