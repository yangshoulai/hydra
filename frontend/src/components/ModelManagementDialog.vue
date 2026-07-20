<template>
  <n-modal
    v-model:show="show"
    preset="card"
    :title="`模型配置 · ${channelName}`"
    style="width: min(1320px, calc(100vw - 48px))"
    :mask-closable="false"
    :closable="true"
    @close="handleClose"
  >
    <template #header-extra>
      <n-space>
        <n-button size="small" type="info" secondary :loading="syncing" @click="handleSyncModels">同步上游模型</n-button>
        <n-button size="small" type="primary" @click="showAddModelDialog = true">手动添加</n-button>
      </n-space>
    </template>

    <n-space vertical :size="12">
      <section class="panel-card">
        <div class="panel-card__body model-test-profile-bar">
          <div>
            <div class="model-test-profile-bar__label">模型测试客户端请求头</div>
            <div class="muted">仅影响本弹窗内发起的模型测试；默认不附加额外客户端请求头。</div>
          </div>
          <n-select
            v-model:value="selectedClientHeaderProfileId"
            :options="clientHeaderProfileOptions"
            style="width: 280px"
            placeholder="请选择测试请求头"
          />
        </div>
      </section>

      <section class="metric-grid" style="grid-template-columns: repeat(4, minmax(140px, 1fr))">
        <div class="metric-tile">
          <div class="metric-tile__label">本地已配置</div>
          <div class="metric-tile__value">{{ stats.localConfigured }}</div>
        </div>
        <div class="metric-tile">
          <div class="metric-tile__label">上游模型总数</div>
          <div class="metric-tile__value">{{ stats.upstreamTotal }}</div>
        </div>
        <div class="metric-tile">
          <div class="metric-tile__label">待新增</div>
          <div class="metric-tile__value">{{ stats.toAdd }}</div>
        </div>
        <div class="metric-tile">
          <div class="metric-tile__label">待删除</div>
          <div class="metric-tile__value">{{ stats.toRemove }}</div>
        </div>
      </section>

      <section class="panel-card">
        <header class="panel-card__header">
          <h3 class="panel-card__title">模型配置列表</h3>
          <n-text depth="3" style="font-size: 12px">已选中 {{ checkedKeys.length }} 条</n-text>
        </header>
        <div class="panel-card__body">
          <n-data-table
            :columns="columns"
            :data="displayModels"
            :loading="loading"
            :row-key="(row: ModelDisplayType) => row.key"
            v-model:checked-row-keys="checkedKeys"
            :pagination="pagination"
            :single-line="false"
            :scroll-x="1240"
          />
        </div>
      </section>

      <n-space justify="end">
        <n-button @click="handleClose">取消</n-button>
        <n-button type="primary" :loading="saving" @click="handleSave">保存变更</n-button>
      </n-space>
    </n-space>

    <n-modal
      v-model:show="showConfigDialog"
      preset="card"
      title="端点与测试配置"
      style="width: 580px"
      :closable="true"
    >
      <n-form label-placement="top" size="medium">
        <n-form-item label="当前模型">
          <n-text code>{{ activeConfigRow?.channel_model }}</n-text>
        </n-form-item>
        <n-form-item label="端点类型">
          <n-select
            v-model:value="configEditor.endpoint_types"
            class="compact-multi-select"
            :options="endpointTypeOptions"
            multiple
            size="small"
            :max-tag-count="2"
            placeholder="请选择端点类型"
          />
        </n-form-item>
        <n-form-item label="密钥分组">
          <n-select
            v-model:value="configEditor.key_groups"
            class="compact-multi-select"
            :options="keyGroupOptions"
            multiple
            size="small"
            :max-tag-count="2"
            placeholder="请选择密钥分组"
          />
        </n-form-item>
        <n-form-item label="模型测试提示词（可选）">
          <n-input
            v-model:value="configEditor.test_prompt"
            type="textarea"
            :autosize="{ minRows: 2, maxRows: 5 }"
            placeholder="为空时使用系统设置中的默认测试提示词"
          />
        </n-form-item>
      </n-form>

      <template #footer>
        <n-space justify="space-between" align="center" style="width: 100%">
          <n-text depth="3" style="font-size: 12px">{{ activeTestDetail || '尚未进行测试' }}</n-text>
          <n-space>
            <n-button @click="showConfigDialog = false">关闭</n-button>
            <n-button type="info" :loading="activeTesting" @click="handleTestActiveConfig">立即测试</n-button>
            <n-button type="primary" @click="applyConfigEditor">应用</n-button>
          </n-space>
        </n-space>
      </template>
    </n-modal>

    <n-modal
      v-model:show="showAddModelDialog"
      preset="card"
      title="手动添加模型"
      style="width: 520px"
    >
      <n-form ref="modelFormRef" :model="modelForm" :rules="modelRules" label-placement="top" size="medium">
        <n-form-item label="渠道模型名称" path="channel_model">
          <n-input v-model:value="modelForm.channel_model" placeholder="例如：gpt-4o" />
        </n-form-item>
        <n-form-item label="统一模型" path="model">
          <n-select
            v-model:value="modelForm.model"
            :options="modelOptions"
            :loading="loadingModels"
            filterable
            placeholder="请选择统一模型"
          />
        </n-form-item>
        <n-form-item label="模型权重" path="weight">
          <n-input-number v-model:value="modelForm.weight" :min="1" :max="1000" style="width: 100%" placeholder="1-1000" />
        </n-form-item>
        <n-form-item label="端点类型" path="endpoint_types">
          <n-select
            v-model:value="modelForm.endpoint_types"
            class="compact-multi-select"
            :options="endpointTypeOptions"
            multiple
            size="small"
            :max-tag-count="2"
            placeholder="请选择端点类型"
          />
        </n-form-item>
        <n-form-item label="密钥分组" path="key_groups">
          <n-select
            v-model:value="modelForm.key_groups"
            class="compact-multi-select"
            :options="keyGroupOptions"
            multiple
            size="small"
            :max-tag-count="2"
            placeholder="请选择密钥分组"
          />
        </n-form-item>
        <n-form-item label="模型测试提示词（可选）">
          <n-input
            v-model:value="modelForm.test_prompt"
            type="textarea"
            :autosize="{ minRows: 2, maxRows: 4 }"
            placeholder="为空时使用系统默认提示词"
          />
        </n-form-item>
      </n-form>

      <template #footer>
        <n-space justify="end">
          <n-button @click="showAddModelDialog = false">取消</n-button>
          <n-button type="primary" @click="handleAddModel">确认添加</n-button>
        </n-space>
      </template>
    </n-modal>

    <n-modal
      v-model:show="showQuickCreateModelDialog"
      preset="card"
      title="新增统一模型"
      style="width: 520px"
      :mask-closable="false"
    >
      <n-form
        ref="quickCreateModelFormRef"
        :model="quickCreateModelForm"
        :rules="quickCreateModelRules"
        label-placement="top"
        size="medium"
      >
        <n-form-item label="模型名称" path="name">
          <n-input v-model:value="quickCreateModelForm.name" placeholder="例如：gpt-5.4" />
        </n-form-item>
        <n-form-item label="所属厂商" path="provider_id">
          <n-select
            v-model:value="quickCreateModelForm.provider_id"
            :options="providerOptions"
            :loading="loadingProviders"
            filterable
            placeholder="请选择厂商"
          />
        </n-form-item>
      </n-form>

      <template #footer>
        <n-space justify="end">
          <n-button @click="showQuickCreateModelDialog = false">取消</n-button>
          <n-button type="primary" :loading="creatingUnifiedModel" @click="handleQuickCreateModel">
            保存模型
          </n-button>
        </n-space>
      </template>
    </n-modal>

    <ImageTestDialog
      v-model:show="showImageTestDialog"
      :initial-prompt="imageTestInitialPrompt"
      :initial-text-prompt="imageTestInitialTextPrompt"
      :initial-generation-prompt="imageTestInitialGenerationPrompt"
      :initial-edit-prompt="imageTestInitialEditPrompt"
      :initial-size="imageTestInitialSize"
      :initial-quality="imageTestInitialQuality"
      :mode="imageTestMode"
      :endpoint-types="imageTestEndpointTypes"
      :loading="activeTesting"
      @submit="handleImageTestSubmit"
    />

    <ModelTestResultDialog
      v-model:show="showTestResultDialog"
      :title="testResultTitle"
      :items="testResultItems"
    />
  </n-modal>
</template>

<script setup lang="ts">
import { computed, h, nextTick, reactive, ref, watch } from 'vue'
import {
  type DataTableColumns,
  type FormInst,
  type FormRules,
  NButton,
  NDataTable,
  NForm,
  NFormItem,
  NIcon,
  NInput,
  NInputNumber,
  NModal,
  NPopconfirm,
  NSelect,
  NSpace,
  NTag,
  NText,
  NTooltip,
} from 'naive-ui'
import {
  AddOutline,
  CheckmarkCircleOutline,
  CloseCircleOutline,
  PlayOutline,
  SettingsOutline,
} from '@vicons/ionicons5'
import { channelApi } from '@/services/channelService'
import { modelApi } from '@/services/modelService'
import { endpointApi } from '@/services/endpointService'
import { providerApi } from '@/services/providerService'
import { settingsService } from '@/services/settingsService'
import ModelTestResultDialog from '@/components/ModelTestResultDialog.vue'
import ImageTestDialog from '@/components/ImageTestDialog.vue'
import type {
  Channel,
  ChannelModelConfig,
  ModelConfigItem,
  ModelConfigUpdateItem,
  SyncResult,
} from '@/types/channel'
import type { ModelTestResultItem } from '@/types/modelTest'
import type { Model, Provider } from '@/types/model'
import type { EndpointInfo } from '@/types/endpoint'
import { createModelTestResultItem, formatTestResultSummary } from '@/utils/modelTest'
import { getErrorMessage, toastApiError } from '@/utils/error'
import { feedback } from '@/services/feedback'
import { buildModelTestClientHeaderProfileOptions, parseModelTestClientHeaderProfiles } from '@/utils/modelTestClientHeaders'

interface Props {
  channelId: number
  channelName: string
  modelValue: boolean
}

interface ModelDisplayType {
  key: string
  channel_model: string
  model: string
  weight: number
  endpoint_types: string[]
  key_groups: string[]
  test_prompt: string
  status: 'configured' | 'to_add' | 'to_remove'
  disabled: boolean
  channel_status: 'active' | 'inactive' | 'unconfigured'
  config_id?: number
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  refresh: []
}>()

const show = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit('update:modelValue', value),
})

const loading = ref(false)
const syncing = ref(false)
const saving = ref(false)
const hasSynced = ref(false)
const syncFailed = ref(false)
const syncResult = ref<SyncResult | null>(null)

const showAddModelDialog = ref(false)
const showQuickCreateModelDialog = ref(false)
const showConfigDialog = ref(false)
const activeConfigKey = ref<string>('')
const quickCreateTargetKey = ref('')

const localConfigs = ref<ChannelModelConfig[]>([])
const unifiedModels = ref<Model[]>([])
const providers = ref<Provider[]>([])
const endpoints = ref<EndpointInfo[]>([])
const loadingModels = ref(false)
const loadingProviders = ref(false)
const creatingUnifiedModel = ref(false)

const checkedKeys = ref<string[]>([])

const modelEditMap = ref<Record<string, string>>({})
const weightEditMap = ref<Record<string, number>>({})
const endpointTypesEditMap = ref<Record<string, string[]>>({})
const keyGroupsEditMap = ref<Record<string, string[]>>({})
const testPromptEditMap = ref<Record<string, string>>({})
const imageTestSizeEditMap = ref<Record<string, string>>({})
const imageTestQualityEditMap = ref<Record<string, string>>({})

const testStatus = ref<Record<string, Record<string, 'idle' | 'testing' | 'success' | 'error'>>>({})
const testDetailMap = ref<Record<string, string>>({})

const channelWeight = ref(100)
const keyGroupOptions = ref<Array<{ label: string; value: string }>>([{ label: 'Default', value: 'Default' }])

const modelFormRef = ref<FormInst | null>(null)
const quickCreateModelFormRef = ref<FormInst | null>(null)
const modelForm = reactive({
  channel_model: '',
  model: '',
  weight: 100,
  endpoint_types: ['OpenAIChatCompletions'],
  key_groups: ['Default'],
  test_prompt: '',
})

const quickCreateModelForm = reactive({
  name: '',
  provider_id: null as string | null,
})

const modelRules: FormRules = {
  channel_model: { required: true, message: '请输入渠道模型名称', trigger: ['blur', 'input'] },
  model: { required: true, message: '请选择统一模型', trigger: ['blur', 'change'] },
  weight: { type: 'number', required: true, message: '请输入权重', trigger: ['blur', 'change'] },
  endpoint_types: { type: 'array', required: true, message: '请选择至少一个端点类型', trigger: ['change'] },
  key_groups: { type: 'array', required: true, message: '请选择至少一个密钥分组', trigger: ['change'] },
}

const quickCreateModelRules: FormRules = {
  name: { required: true, message: '请输入模型名称', trigger: ['blur', 'input'] },
  provider_id: { required: true, type: 'string', message: '请选择厂商', trigger: ['blur', 'change'] },
}

const configEditor = reactive({
  endpoint_types: [] as string[],
  key_groups: [] as string[],
  test_prompt: '',
})

const showImageTestDialog = ref(false)
const showTestResultDialog = ref(false)
const pendingImageTestRow = ref<ModelDisplayType | null>(null)
const imageTestMode = ref<'generation' | 'edit' | 'mixed'>('edit')
const imageTestEndpointTypes = ref<string[]>([])
const imageTestInitialPrompt = ref('')
const imageTestInitialTextPrompt = ref('')
const imageTestInitialGenerationPrompt = ref('')
const imageTestInitialEditPrompt = ref('')
const imageTestInitialSize = ref('1024x1024')
const imageTestInitialQuality = ref('low')
const testResultTitle = ref('模型测试结果')
const testResultItems = ref<ModelTestResultItem[]>([])
const clientHeaderProfileOptions = ref(buildModelTestClientHeaderProfileOptions([]))
const selectedClientHeaderProfileId = ref('')

const DEFAULT_IMAGE_TEST_SIZE = '1024x1024'
const DEFAULT_IMAGE_TEST_QUALITY = 'low'
const IMAGE_GENERATION_DEFAULT_PROMPT = '请生成一只戴着耳机的柯基'
const IMAGE_EDIT_DEFAULT_PROMPT = '请将图片中的背景替换为星空，并保持主体不变'

const activeConfigRow = computed(() => displayModels.value.find((item) => item.key === activeConfigKey.value))
const activeTestDetail = computed(() => testDetailMap.value[activeConfigKey.value] || '')
const activeTesting = computed(() => {
  const row = activeConfigRow.value
  if (!row) return false
  const statusMap = testStatus.value[activeConfigKey.value]
  if (!statusMap) return false
  const endpointTypes = endpointTypesEditMap.value[row.key] || row.endpoint_types || []
  return endpointTypes.some((type) => statusMap[type] === 'testing')
})

const endpointTypeOptions = computed(() => endpoints.value.map((item) => ({ label: item.name, value: item.type })))
const providerOptions = computed(() =>
  providers.value.map((provider) => ({
    label: provider.name,
    value: provider.id,
  })),
)
const modelOptions = computed(() =>
  unifiedModels.value.map((model) => ({
    label: model.provider ? `${model.name} (${model.provider.name})` : model.name,
    value: model.name,
  })),
)
const localModelNameSet = computed(() => new Set(unifiedModels.value.map((item) => normalizeModelName(item.name))))
const stats = computed(() => {
  if (!hasSynced.value || syncFailed.value || !syncResult.value?.diff) {
    return {
      localConfigured: localConfigs.value.length,
      upstreamTotal: 0,
      toAdd: 0,
      toRemove: 0,
    }
  }

  return {
    localConfigured: syncResult.value.diff.existing_count,
    upstreamTotal: syncResult.value.diff.total_upstream_models,
    toAdd: syncResult.value.diff.added_count,
    toRemove: syncResult.value.diff.removed_count,
  }
})

const displayModels = computed<ModelDisplayType[]>(() => {
  if (!hasSynced.value || syncFailed.value || !syncResult.value) {
    return localConfigs.value.map((config) => {
      const endpointTypes = endpointTypesEditMap.value[config.channel_model] || config.endpoint_types || ['OpenAIChatCompletions']

      return {
        key: config.channel_model,
        channel_model: config.channel_model,
        model: modelEditMap.value[config.channel_model] || config.model,
        weight: weightEditMap.value[config.channel_model] || config.weight || channelWeight.value,
        endpoint_types: endpointTypes,
        key_groups: keyGroupsEditMap.value[config.channel_model] || config.key_groups || ['Default'],
        test_prompt: testPromptEditMap.value[config.channel_model] ?? config.test_prompt ?? '',
        status: 'configured',
        disabled: false,
        channel_status: config.status || 'unconfigured',
        config_id: config.id,
      }
    })
  }

  const rows: ModelDisplayType[] = []

  syncResult.value.diff.diffs.forEach((item) => {
    const model = modelEditMap.value[item.channel_model] || item.existing_config?.model || item.channel_model
    const weight =
      weightEditMap.value[item.channel_model] || item.existing_config?.weight || channelWeight.value
    const endpointTypes =
      endpointTypesEditMap.value[item.channel_model] ||
      item.existing_config?.endpoint_types ||
      ['OpenAIChatCompletions']
    const keyGroups =
      keyGroupsEditMap.value[item.channel_model] ||
      item.key_groups ||
      item.existing_config?.key_groups ||
      ['Default']
    const testPrompt = testPromptEditMap.value[item.channel_model] ?? item.existing_config?.test_prompt ?? ''

    const status: ModelDisplayType['status'] =
      item.type === 'existing' ? 'configured' : item.type === 'added' ? 'to_add' : 'to_remove'

    rows.push({
      key: item.channel_model,
      channel_model: item.channel_model,
      model,
      weight,
      endpoint_types: endpointTypes,
      key_groups: keyGroups,
      test_prompt: testPrompt,
      status,
      disabled: status === 'to_remove',
      channel_status: item.existing_config?.status || 'unconfigured',
      config_id: item.existing_config?.id,
    })
  })

  const statusOrder = { configured: 1, to_remove: 2, to_add: 3 }
  rows.sort((a, b) => {
    const diff = statusOrder[a.status] - statusOrder[b.status]
    if (diff !== 0) return diff
    return a.channel_model.localeCompare(b.channel_model)
  })

  return rows
})

const pagination = reactive({
  page: 1,
  pageSize: 12,
  showSizePicker: true,
  pageSizes: [12, 24, 50],
  onChange: (page: number) => {
    pagination.page = page
  },
  onUpdatePageSize: (size: number) => {
    pagination.pageSize = size
    pagination.page = 1
  },
})

const columns: DataTableColumns<ModelDisplayType> = [
  {
    type: 'selection',
    width: 46,
    disabled: (row) => row.disabled,
  },
  {
    title: '渠道模型',
    key: 'channel_model',
    width: 240,
    ellipsis: { tooltip: true },
    render: (row) =>
      h(
        NSpace,
        { size: 4, align: 'center', wrap: false },
        {
          default: () => [
            h(
              NText,
              { code: true, style: 'max-width: 198px; display: inline-block;' },
              { default: () => row.channel_model },
            ),
            hasSynced.value && !hasLocalModel(row.channel_model)
              ? h(
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
                        circle: true,
                        type: 'info',
                        'aria-label': `新增统一模型 ${row.channel_model}`,
                        onClick: () => openQuickCreateModelDialog(row),
                      },
                      { icon: () => h(NIcon, null, { default: () => h(AddOutline) }) },
                    ),
                  default: () => '新增统一模型',
                },
              )
              : null,
          ],
        },
      ),
  },
  {
    title: '统一模型',
    key: 'model',
    minWidth: 240,
    render: (row) =>
      h(NSelect, {
        value: modelEditMap.value[row.key] || row.model,
        options: modelOptions.value,
        size: 'small',
        filterable: true,
        placeholder: '选择统一模型',
        style: { width: '100%' },
        onUpdateValue: (value: string) => {
          modelEditMap.value[row.key] = value
        },
      }),
  },
  {
    title: '权重',
    key: 'weight',
    width: 150,
    align: 'center',
    render: (row) =>
      h(NInputNumber, {
        value: weightEditMap.value[row.key] || row.weight,
        size: 'small',
        min: 1,
        max: 1000,
        style: { width: '100%' },
        onUpdateValue: (value: number | null) => {
          weightEditMap.value[row.key] = value && value > 0 ? value : row.weight
        },
      }),
  },
  {
    title: '配置状态',
    key: 'status',
    width: 110,
    align: 'center',
    render: (row) => {
      const map = {
        configured: { type: 'success' as const, text: '已配置' },
        to_add: { type: 'info' as const, text: '待新增' },
        to_remove: { type: 'warning' as const, text: '待删除' },
      }
      return h(NTag, { size: 'small', type: map[row.status].type, bordered: false }, { default: () => map[row.status].text })
    },
  },
  {
    title: '状态',
    key: 'channel_status',
    width: 110,
    align: 'center',
    render: (row) => {
      if (!row.config_id) {
        return h(NText, { depth: 3, style: 'font-size: 12px' }, { default: () => '—' })
      }
      const isActive = row.channel_status === 'active'
      return h(
        NTag,
        { size: 'small', type: isActive ? 'success' : 'default', bordered: false },
        { default: () => (isActive ? '启用' : '停用') },
      )
    },
  },
  {
    title: '端点 / 分组',
    key: 'config',
    width: 120,
    align: 'center',
    render: (row) => {
      const endpointCount = (endpointTypesEditMap.value[row.key] || row.endpoint_types).length
      const keyGroupCount = (keyGroupsEditMap.value[row.key] || row.key_groups).length
      return h(
        NSpace,
        { size: 4, justify: 'center', align: 'center', class: 'table-action-group', wrap: false },
        {
          default: () => [
            h(
              NText,
              { depth: 3, style: 'font-size: 11px; white-space: nowrap' },
              { default: () => `${endpointCount}/${keyGroupCount}` },
            ),
            renderActionIcon({
              tooltip: '编辑端点/分组/测试提示词',
              ariaLabel: `编辑模型 ${row.channel_model} 的端点、分组和测试提示词`,
              icon: SettingsOutline,
              type: 'info',
              onClick: () => openConfigDialog(row),
            }),
          ],
        },
      )
    },
  },
  {
    title: '操作',
    key: 'actions',
    width: 100,
    align: 'center',
    render: (row) => {
      const children: any[] = [
        renderActionIcon({
          tooltip: '测试当前配置',
          ariaLabel: `测试模型 ${row.channel_model} 的当前配置`,
          icon: PlayOutline,
          type: 'info',
          loading: isRowTesting(row.key),
          onClick: () => handleTest(row),
        }),
      ]

      if (row.config_id) {
        const isActive = row.channel_status === 'active'
        children.push(
          h(
            NPopconfirm,
            { onPositiveClick: () => handleToggleModelStatus(row) },
            {
              default: () => `确定要${isActive ? '停用' : '启用'}该模型配置吗？`,
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
                          quaternary: true,
                          circle: true,
                          type: isActive ? 'warning' : 'success',
                          'aria-label': `${isActive ? '停用' : '启用'}配置 ${row.config_id}`,
                        },
                        {
                          icon: () =>
                            h(NIcon, null, {
                              default: () => h(isActive ? CloseCircleOutline : CheckmarkCircleOutline),
                            }),
                        },
                      ),
                    default: () => (isActive ? '停用' : '启用'),
                  },
                ),
            },
          ),
        )
      }

      return h(
        NSpace,
        { size: 4, justify: 'center', align: 'center', class: 'table-action-group', wrap: false },
        { default: () => children },
      )
    },
  },
]

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

function isRowTesting(key: string): boolean {
  const rowMap = testStatus.value[key]
  if (!rowMap) return false
  const row = displayModels.value.find((item) => item.key === key)
  const endpointTypes = row
    ? (endpointTypesEditMap.value[key] || row.endpoint_types || [])
    : Object.keys(rowMap)
  return endpointTypes.some((type) => rowMap[type] === 'testing')
}

function getImageTestSize(key: string): string {
  return imageTestSizeEditMap.value[key] || DEFAULT_IMAGE_TEST_SIZE
}

function getImageTestQuality(key: string): string {
  return imageTestQualityEditMap.value[key] || DEFAULT_IMAGE_TEST_QUALITY
}

function setImageTestOptions(key: string, size?: string, quality?: string) {
  imageTestSizeEditMap.value[key] = size || DEFAULT_IMAGE_TEST_SIZE
  imageTestQualityEditMap.value[key] = quality || DEFAULT_IMAGE_TEST_QUALITY
}

function initEditState() {
  modelEditMap.value = {}
  weightEditMap.value = {}
  endpointTypesEditMap.value = {}
  keyGroupsEditMap.value = {}
  testPromptEditMap.value = {}
  imageTestSizeEditMap.value = {}
  imageTestQualityEditMap.value = {}
  testStatus.value = {}
  testDetailMap.value = {}

  const defaultChecked: string[] = []

  if (hasSynced.value && !syncFailed.value && syncResult.value) {
    syncResult.value.diff.diffs.forEach((item) => {
      if (item.type === 'existing' && item.existing_config) {
        modelEditMap.value[item.channel_model] = item.existing_config.model
        weightEditMap.value[item.channel_model] = item.existing_config.weight || channelWeight.value
        const endpointTypes = item.existing_config.endpoint_types || ['OpenAIChatCompletions']
        endpointTypesEditMap.value[item.channel_model] = endpointTypes
        keyGroupsEditMap.value[item.channel_model] =
          item.key_groups || item.existing_config.key_groups || ['Default']
        testPromptEditMap.value[item.channel_model] = item.existing_config.test_prompt || ''
        defaultChecked.push(item.channel_model)
      } else if (item.type === 'added') {
        modelEditMap.value[item.channel_model] = item.channel_model
        weightEditMap.value[item.channel_model] = channelWeight.value
        const endpointTypes = ['OpenAIChatCompletions']
        endpointTypesEditMap.value[item.channel_model] = endpointTypes
        keyGroupsEditMap.value[item.channel_model] = item.key_groups || ['Default']
        testPromptEditMap.value[item.channel_model] = ''
      } else if (item.type === 'removed') {
        modelEditMap.value[item.channel_model] = item.existing_config?.model || item.channel_model
        weightEditMap.value[item.channel_model] = item.existing_config?.weight || channelWeight.value
        const endpointTypes = item.existing_config?.endpoint_types || ['OpenAIChatCompletions']
        endpointTypesEditMap.value[item.channel_model] = endpointTypes
        keyGroupsEditMap.value[item.channel_model] = item.existing_config?.key_groups || ['Default']
        testPromptEditMap.value[item.channel_model] = item.existing_config?.test_prompt || ''
        defaultChecked.push(item.channel_model)
      }
    })
  } else {
    localConfigs.value.forEach((config) => {
      modelEditMap.value[config.channel_model] = config.model
      weightEditMap.value[config.channel_model] = config.weight || channelWeight.value
      const endpointTypes = config.endpoint_types || ['OpenAIChatCompletions']
      endpointTypesEditMap.value[config.channel_model] = endpointTypes
      keyGroupsEditMap.value[config.channel_model] = config.key_groups || ['Default']
      testPromptEditMap.value[config.channel_model] = config.test_prompt || ''
      defaultChecked.push(config.channel_model)
    })
  }

  checkedKeys.value = defaultChecked
}

async function loadLocalConfigs() {
  if (!props.channelId) return

  loading.value = true
  try {
    const channel: Channel = await channelApi.get(props.channelId)
    channelWeight.value = channel.weight || 100
    modelForm.weight = channelWeight.value
    localConfigs.value = channel.model_configs || []

    const groups = new Set<string>()
    channel.channel_keys?.forEach((item) => {
      if (item.channel_key_group) groups.add(item.channel_key_group)
    })
    if (!groups.size) groups.add('Default')

    keyGroupOptions.value = Array.from(groups)
      .sort((a, b) => a.localeCompare(b))
      .map((group) => ({ label: group, value: group }))
  } finally {
    loading.value = false
  }
}

async function loadUnifiedModels() {
  loadingModels.value = true
  try {
    const result = await modelApi.list({ page: 1, page_size: 1000 })
    unifiedModels.value = result.items
  } catch {
    feedback.message?.error('加载统一模型失败')
  } finally {
    loadingModels.value = false
  }
}

async function loadProviders() {
  loadingProviders.value = true
  try {
    providers.value = await providerApi.list()
  } catch {
    feedback.message?.error('加载厂商列表失败')
  } finally {
    loadingProviders.value = false
  }
}

async function loadClientHeaderProfiles() {
  try {
    const settings = await settingsService.getAllSettings()
    const profiles = parseModelTestClientHeaderProfiles(settings.model_test_client_header_profiles)
    clientHeaderProfileOptions.value = buildModelTestClientHeaderProfileOptions(profiles)
    if (!clientHeaderProfileOptions.value.some((item) => item.value === selectedClientHeaderProfileId.value)) {
      selectedClientHeaderProfileId.value = ''
    }
  } catch {
    clientHeaderProfileOptions.value = buildModelTestClientHeaderProfileOptions([])
    selectedClientHeaderProfileId.value = ''
  }
}

function normalizeModelName(name: string): string {
  return (name || '').trim().toLowerCase()
}

function hasLocalModel(channelModel: string): boolean {
  return localModelNameSet.value.has(normalizeModelName(channelModel))
}

function openQuickCreateModelDialog(row: ModelDisplayType) {
  quickCreateTargetKey.value = row.key
  quickCreateModelForm.name = row.channel_model
  quickCreateModelForm.provider_id = null
  showQuickCreateModelDialog.value = true
}

async function handleQuickCreateModel() {
  if (!quickCreateModelFormRef.value) return

  try {
    await quickCreateModelFormRef.value.validate()
  } catch {
    return
  }

  const name = quickCreateModelForm.name.trim()
  if (!name) {
    feedback.message?.warning('请输入模型名称')
    return
  }

  if (hasLocalModel(name)) {
    feedback.message?.warning(`模型 ${name} 已存在`)
    showQuickCreateModelDialog.value = false
    return
  }

  creatingUnifiedModel.value = true
  try {
    const created = await modelApi.create({
      name,
      provider_id: quickCreateModelForm.provider_id,
    })
    await loadUnifiedModels()
    if (quickCreateTargetKey.value) {
      modelEditMap.value[quickCreateTargetKey.value] = created.name
    }
    showQuickCreateModelDialog.value = false
    feedback.message?.success(`统一模型 ${created.name} 创建成功`)
  } catch (err) {
    toastApiError(err, '创建统一模型失败')
  } finally {
    creatingUnifiedModel.value = false
  }
}

async function loadEndpoints() {
  try {
    endpoints.value = await endpointApi.list()
  } catch {
    feedback.message?.error('加载端点类型失败')
  }
}

async function handleSyncModels() {
  if (!props.channelId) return

  syncing.value = true
  syncFailed.value = false
  try {
    const result = await channelApi.syncModels(props.channelId)
    syncResult.value = result
    hasSynced.value = true
    initEditState()
    feedback.message?.success('同步完成')
  } catch (err) {
    syncFailed.value = true
    hasSynced.value = true
    syncResult.value = null
    toastApiError(err, '同步失败，已回退本地模式')
  } finally {
    syncing.value = false
  }
}

async function handleAddModel() {
  if (!modelFormRef.value) return

  try {
    await modelFormRef.value.validate()

    await channelApi.createModelConfig(
      props.channelId,
      modelForm.model,
      modelForm.channel_model,
      modelForm.weight,
      modelForm.endpoint_types,
      modelForm.key_groups,
      modelForm.test_prompt,
    )

    feedback.message?.success('模型配置已添加')
    showAddModelDialog.value = false

    modelForm.channel_model = ''
    modelForm.model = ''
    modelForm.weight = channelWeight.value
    modelForm.endpoint_types = ['OpenAIChatCompletions']
    modelForm.key_groups = ['Default']
    modelForm.test_prompt = ''

    hasSynced.value = false
    syncFailed.value = false
    syncResult.value = null

    await loadLocalConfigs()
    initEditState()
    emit('refresh')
  } catch (err) {
    if (!(err as { errors?: unknown })?.errors) {
      toastApiError(err, '添加模型失败')
    }
  }
}

function openConfigDialog(row: ModelDisplayType) {
  activeConfigKey.value = row.key
  configEditor.endpoint_types = [...(endpointTypesEditMap.value[row.key] || row.endpoint_types)]
  configEditor.key_groups = [...(keyGroupsEditMap.value[row.key] || row.key_groups)]
  configEditor.test_prompt = testPromptEditMap.value[row.key] ?? row.test_prompt ?? ''
  showConfigDialog.value = true
}

function applyConfigEditor() {
  if (!activeConfigKey.value) return

  if (!configEditor.endpoint_types.length) {
    feedback.message?.warning('请至少选择一个端点类型')
    return
  }
  if (!configEditor.key_groups.length) {
    feedback.message?.warning('请至少选择一个密钥分组')
    return
  }

  const endpointTypes = [...configEditor.endpoint_types]
  endpointTypesEditMap.value[activeConfigKey.value] = endpointTypes
  keyGroupsEditMap.value[activeConfigKey.value] = [...configEditor.key_groups]
  testPromptEditMap.value[activeConfigKey.value] = configEditor.test_prompt?.trim() || ''
  showConfigDialog.value = false
}

async function handleTestActiveConfig() {
  const row = activeConfigRow.value
  if (!row) return

  if (!configEditor.endpoint_types.length) {
    feedback.message?.warning('请至少选择一个端点类型')
    return
  }
  if (!configEditor.key_groups.length) {
    feedback.message?.warning('请至少选择一个密钥分组')
    return
  }

  endpointTypesEditMap.value[row.key] = [...configEditor.endpoint_types]
  keyGroupsEditMap.value[row.key] = [...configEditor.key_groups]
  testPromptEditMap.value[row.key] = configEditor.test_prompt?.trim() || ''

  await handleTest(row)
}

async function handleTest(row: ModelDisplayType) {
  const endpointTypes = endpointTypesEditMap.value[row.key] || row.endpoint_types
  const includesImageGeneration = endpointTypes.includes('OpenAIImagesGenerations')
  const includesImageEdit = endpointTypes.includes('OpenAIImagesEdits')
  if (includesImageGeneration || includesImageEdit) {
    pendingImageTestRow.value = row
    const configuredPrompt = testPromptEditMap.value[row.key] || row.test_prompt || ''
    imageTestMode.value = includesImageGeneration && includesImageEdit
      ? 'mixed'
      : includesImageGeneration
        ? 'generation'
        : 'edit'
    imageTestEndpointTypes.value = [...endpointTypes]
    imageTestInitialPrompt.value = configuredPrompt
    imageTestInitialTextPrompt.value = configuredPrompt
    imageTestInitialGenerationPrompt.value = configuredPrompt || IMAGE_GENERATION_DEFAULT_PROMPT
    imageTestInitialEditPrompt.value = configuredPrompt || IMAGE_EDIT_DEFAULT_PROMPT
    imageTestInitialSize.value = getImageTestSize(row.key)
    imageTestInitialQuality.value = getImageTestQuality(row.key)
    showImageTestDialog.value = true
    return
  }
  await executeModelTest(row)
}

async function handleToggleModelStatus(row: ModelDisplayType) {
  if (!row.config_id) {
    feedback.message?.warning('尚未保存的模型配置，请先保存')
    return
  }
  try {
    const updated = await channelApi.toggleChannelModelStatus(row.config_id)
    const targetStatus = updated.status

    const localIdx = localConfigs.value.findIndex((c) => c.id === row.config_id)
    const localConfig = localIdx !== -1 ? localConfigs.value[localIdx] : undefined
    if (localConfig) {
      localConfig.status = targetStatus
    }

    if (syncResult.value) {
      syncResult.value.diff.diffs.forEach((item) => {
        const existing = item.existing_config
        if (existing && existing.id === row.config_id) {
          existing.status = targetStatus
        }
      })
    }

    feedback.message?.success(`已${targetStatus === 'active' ? '启用' : '停用'}该模型配置`)
    emit('refresh')
  } catch (err) {
    toastApiError(err, '切换状态失败')
  }
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
  const row = pendingImageTestRow.value
  if (!row) return
  showImageTestDialog.value = false
  setImageTestOptions(row.key, payload.size, payload.quality)
  await executeModelTest(row, {
    prompt: payload.prompt,
    textPrompt: payload.textPrompt,
    imageGenerationPrompt: payload.generationPrompt,
    imageEditPrompt: payload.editPrompt,
    imageData: payload.imageData,
    imageSize: payload.size,
    imageQuality: payload.quality,
  })
  pendingImageTestRow.value = null
}

async function executeModelTest(
  row: ModelDisplayType,
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
  const key = row.key
  const endpointTypes = endpointTypesEditMap.value[key] || row.endpoint_types
  const keyGroups = keyGroupsEditMap.value[key] || row.key_groups
  const model = modelEditMap.value[key] || row.model
  const modelTestPrompt = (testPromptEditMap.value[key] ?? row.test_prompt ?? '').trim()

  if (!endpointTypes.length) {
    feedback.message?.warning('请先配置端点类型')
    return
  }
  if (!keyGroups.length) {
    feedback.message?.warning('请先配置密钥分组')
    return
  }

  const initialStatus: Record<string, 'idle' | 'testing' | 'success' | 'error'> = {}
  endpointTypes.forEach((type) => {
    initialStatus[type] = 'testing'
  })
  testStatus.value[key] = initialStatus
  const rowStatus = testStatus.value[key]!

  const summary: string[] = []
  let resultItems: ModelTestResultItem[] = []

  try {
    resultItems = await Promise.all(
      endpointTypes.map(async (endpointType, index) => {
        try {
          const isImageEndpoint = endpointType === 'OpenAIImagesGenerations' || endpointType === 'OpenAIImagesEdits'
          const testPrompt = endpointType === 'OpenAIImagesGenerations'
            ? (options?.imageGenerationPrompt || options?.prompt || modelTestPrompt)
            : endpointType === 'OpenAIImagesEdits'
              ? (options?.imageEditPrompt || options?.prompt || modelTestPrompt)
              : (options?.textPrompt ?? modelTestPrompt)
          const imageData = endpointType === 'OpenAIImagesEdits' ? (options?.imageData || '') : ''

          const result = await channelApi.testModel(
            props.channelId,
            row.channel_model,
            model,
            endpointType,
            keyGroups,
            {
              testPrompt,
              imageData,
              imageSize: isImageEndpoint ? (options?.imageSize || getImageTestSize(key)) : undefined,
              imageQuality: isImageEndpoint ? (options?.imageQuality || getImageTestQuality(key)) : undefined,
              clientHeaderProfileId: selectedClientHeaderProfileId.value,
            },
          )
          rowStatus[endpointType] = result.success ? 'success' : 'error'
          summary.push(`${endpointType}: ${formatTestResultSummary(result)}`)
          return createModelTestResultItem({
            id: `${key}:${endpointType}:${index}`,
            channelName: props.channelName,
            modelName: model,
            channelModel: row.channel_model,
            endpointType,
            result,
          })
        } catch (err) {
          rowStatus[endpointType] = 'error'
          const errorMessage = getErrorMessage(err, '测试失败')
          summary.push(`${endpointType}: ${errorMessage}`)
          return createModelTestResultItem({
            id: `${key}:${endpointType}:${index}`,
            channelName: props.channelName,
            modelName: model,
            channelModel: row.channel_model,
            endpointType,
            errorMessage,
          })
        }
      }),
    )
  } finally {
    endpointTypes.forEach((type) => {
      if (rowStatus[type] === 'testing') {
        rowStatus[type] = 'error'
      }
    })
  }

  const allPass = endpointTypes.every((type) => rowStatus[type] === 'success')
  testDetailMap.value[key] = summary.join('；')
  testResultTitle.value = `模型测试结果 · ${props.channelName} / ${row.channel_model}`
  testResultItems.value = resultItems
  showTestResultDialog.value = true

  if (!allPass && !resultItems.length) {
    feedback.message?.error('测试失败')
  }
}

async function handleSave() {
  saving.value = true
  try {
    if (!hasSynced.value || syncFailed.value || !syncResult.value) {
      const updatePromises: Promise<unknown>[] = []
      const deletePromises: Promise<void>[] = []

      for (const config of localConfigs.value) {
        const key = config.channel_model
        const checked = checkedKeys.value.includes(key)
        const model = modelEditMap.value[key] || config.model
        const weight = weightEditMap.value[key] || config.weight || channelWeight.value
        const endpointTypes = endpointTypesEditMap.value[key] || config.endpoint_types || ['OpenAIChatCompletions']
        const keyGroups = keyGroupsEditMap.value[key] || config.key_groups || ['Default']
        const testPrompt = (testPromptEditMap.value[key] ?? config.test_prompt ?? '').trim()

        if (!checked) {
          deletePromises.push(channelApi.deleteModelConfig(config.id))
          continue
        }

        if (!endpointTypes.length || !keyGroups.length) {
          feedback.message?.error(`模型 ${key} 必须配置端点与密钥分组`)
          saving.value = false
          return
        }

        const updatePayload: Partial<ChannelModelConfig> = {}
        if (model !== config.model) updatePayload.model = model
        if (weight !== (config.weight || channelWeight.value)) updatePayload.weight = weight
        if (JSON.stringify(endpointTypes) !== JSON.stringify(config.endpoint_types || ['OpenAIChatCompletions'])) {
          updatePayload.endpoint_types = endpointTypes
        }
        if (JSON.stringify(keyGroups) !== JSON.stringify(config.key_groups || ['Default'])) {
          updatePayload.key_groups = keyGroups
        }
        if (testPrompt !== (config.test_prompt || '')) {
          updatePayload.test_prompt = testPrompt
        }

        if (Object.keys(updatePayload).length > 0) {
          updatePromises.push(channelApi.updateModelConfig(config.id, updatePayload))
        }
      }

      await Promise.all([...deletePromises, ...updatePromises])
    } else {
      const addModels: ModelConfigItem[] = []
      const updateModels: ModelConfigUpdateItem[] = []
      const deleteModelIDs: number[] = []

      syncResult.value.diff.diffs.forEach((item) => {
        const key = item.channel_model
        const checked = checkedKeys.value.includes(key)

        const model = modelEditMap.value[key] || item.existing_config?.model || item.channel_model
        const weight = weightEditMap.value[key] || item.existing_config?.weight || channelWeight.value
        const endpointTypes = endpointTypesEditMap.value[key] || item.existing_config?.endpoint_types || ['OpenAIChatCompletions']
        const keyGroups = keyGroupsEditMap.value[key] || item.key_groups || item.existing_config?.key_groups || ['Default']
        const testPrompt = (testPromptEditMap.value[key] ?? item.existing_config?.test_prompt ?? '').trim()

        if (item.type === 'existing') {
          if (!item.existing_config) return

          if (!checked) {
            deleteModelIDs.push(item.existing_config.id)
            return
          }

          const changed =
            model !== item.existing_config.model ||
            weight !== (item.existing_config.weight || channelWeight.value) ||
            JSON.stringify(endpointTypes) !== JSON.stringify(item.existing_config.endpoint_types || ['OpenAIChatCompletions']) ||
            JSON.stringify(keyGroups) !== JSON.stringify(item.existing_config.key_groups || ['Default']) ||
            testPrompt !== (item.existing_config.test_prompt || '')

          if (changed) {
            updateModels.push({
              id: item.existing_config.id,
              model,
              channel_model: item.channel_model,
              weight,
              endpoint_types: endpointTypes,
              key_groups: keyGroups,
              test_prompt: testPrompt,
            })
          }
          return
        }

        if (item.type === 'added') {
          if (!checked) return

          addModels.push({
            model,
            channel_model: item.channel_model,
            weight,
            endpoint_types: endpointTypes,
            key_groups: keyGroups,
            test_prompt: testPrompt,
          })
          return
        }

        if (item.type === 'removed' && checked && item.existing_config) {
          deleteModelIDs.push(item.existing_config.id)
        }
      })

      await channelApi.applySync(props.channelId, {
        add_models: addModels,
        update_models: updateModels,
        delete_model_ids: deleteModelIDs,
      })
    }

    feedback.message?.success('模型配置已保存')
    emit('refresh')
    handleClose()
  } catch (err) {
    toastApiError(err, '保存失败')
  } finally {
    saving.value = false
  }
}

function handleClose() {
  showTestResultDialog.value = false
  testResultItems.value = []
  showQuickCreateModelDialog.value = false
  quickCreateTargetKey.value = ''
  show.value = false
}

watch(
  () => props.modelValue,
  async (open) => {
    if (!open || props.channelId <= 0) return

    hasSynced.value = false
    syncFailed.value = false
    syncResult.value = null

    await Promise.all([loadLocalConfigs(), loadUnifiedModels(), loadProviders(), loadEndpoints(), loadClientHeaderProfiles()])
    initEditState()
    modelForm.test_prompt = ''
    await nextTick()
  },
)
</script>

<style scoped>
:deep(.n-data-table td) {
  vertical-align: middle;
}

:deep(.n-data-table .n-data-table-th),
:deep(.n-data-table .n-data-table-td) {
  padding-left: 8px !important;
  padding-right: 8px !important;
}

:deep(.compact-multi-select .n-base-selection) {
  min-height: 34px;
}

:deep(.compact-multi-select .n-base-selection-label) {
  display: flex;
  align-items: center;
  padding-top: 4px !important;
  padding-bottom: 4px !important;
}

:deep(.compact-multi-select .n-base-selection-tags) {
  display: flex;
  align-items: center;
  align-content: center;
  gap: 6px 4px;
}

:deep(.compact-multi-select .n-base-selection-tag-wrapper) {
  display: inline-flex;
  align-items: center;
  padding: 0 6px 0 0 !important;
}

:deep(.compact-multi-select .n-base-selection-input-tag) {
  margin-bottom: 0 !important;
  height: 22px !important;
  line-height: 22px !important;
}

:deep(.compact-multi-select .n-base-selection-tag-wrapper .n-tag.n-tag--small) {
  font-size: 12px !important;
  height: 22px !important;
  line-height: 22px !important;
  padding: 0 8px !important;
  display: inline-flex;
  align-items: center;
}

:deep(.compact-multi-select .n-base-selection-tag-wrapper .n-tag .n-tag__content) {
  display: inline-flex;
  align-items: center;
}

.muted {
  font-size: 12px;
  color: var(--hydra-text-tertiary);
}

.model-test-profile-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.model-test-profile-bar__label {
  font-size: 13px;
  font-weight: 650;
  color: var(--hydra-text);
}

.image-test-upload {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.image-test-upload__input {
  width: 100%;
  font-size: 12px;
  color: #262626;
}
</style>
