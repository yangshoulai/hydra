<template>
  <div class="app-page">
    <n-alert v-if="listError" type="error" :bordered="false">
      <n-space align="center" justify="space-between" style="width: 100%">
        <span>{{ listError }}</span>
        <n-button text type="error" @click="loadTokens">重试</n-button>
      </n-space>
    </n-alert>

    <section class="panel-card">
      <header class="panel-card__header table-toolbar">
        <div class="table-toolbar__title">
          <h3 class="panel-card__title">访问令牌</h3>
        </div>
        <div class="table-toolbar__actions">
          <n-button size="small" type="primary" @click="showCreateDialog = true">
            <template #icon>
              <n-icon>
                <AddOutline />
              </n-icon>
            </template>
            新建令牌
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
            placeholder="令牌名称"
            @keyup.enter="handleSearch"
          >
            <template #prefix>
              <n-icon>
                <SearchOutline />
              </n-icon>
            </template>
          </n-input>
          <n-select
            v-model:value="filters.status"
            class="table-filter-select"
            size="small"
            clearable
            placeholder="状态"
            :options="statusOptions"
          />
          <n-button size="small" type="primary" @click="handleSearch">查询</n-button>
          <n-button size="small" quaternary @click="handleReset">重置</n-button>
        </div>

        <n-data-table
          :columns="columns"
          :data="tokens"
          :loading="isLoading"
          :locale="tableLocale"
          :pagination="false"
          :single-line="false"
          :scroll-x="1720"
          :row-key="(row: Token) => row.id"
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

    <n-modal
      v-model:show="showCreateDialog"
      preset="card"
      title="新建访问令牌"
      :style="{ width: '560px' }"
    >
      <n-form :model="formData" :rules="rules" label-placement="top" size="medium">
        <n-form-item label="令牌名称" path="name">
          <n-input v-model:value="formData.name" maxlength="20" placeholder="用于区分用途，例如：生产客户端-A" />
        </n-form-item>

        <n-form-item label="可访问模型" path="allowed_models">
          <div class="form-stack">
            <n-select
              v-model:value="formData.allowed_models"
              :options="modelOptions"
              :loading="loadingModels"
              filterable
              multiple
              clearable
              placeholder="留空则不限制模型"
            />
            <p class="form-hint">不配置时默认不限制模型。</p>
          </div>
        </n-form-item>

        <n-form-item label="过期策略" path="expireType">
          <n-radio-group v-model:value="expireType" @update:value="handleExpireTypeChange">
            <n-radio value="never">永不过期</n-radio>
            <n-radio value="custom">自定义过期时间</n-radio>
          </n-radio-group>
        </n-form-item>

        <n-form-item v-if="expireType === 'custom'" label="过期时间" path="expiresAt">
          <n-date-picker
            v-model:value="expiresAtValue"
            type="datetime"
            style="width: 100%"
            :is-date-disabled="isDateDisabled"
            :time-picker-props="{ format: 'HH:mm:ss' }"
          />
        </n-form-item>
      </n-form>

      <template #footer>
        <n-space justify="end">
          <n-button @click="showCreateDialog = false">取消</n-button>
          <n-button type="primary" :loading="isCreating" @click="handleCreate">创建令牌</n-button>
        </n-space>
      </template>
    </n-modal>

    <n-modal
      v-model:show="showSuccessDialog"
      preset="card"
      title="令牌创建成功"
      :style="{ width: '640px' }"
      :mask-closable="false"
    >
      <n-space vertical :size="14">
        <n-alert type="warning" :bordered="false">
          完整令牌仅展示这一次，请立即复制并妥善保管。
        </n-alert>

        <div class="token-info-row">
          <span class="muted">令牌名称</span>
          <strong>{{ createdToken?.name }}</strong>
        </div>
        <div class="token-info-row token-info-row--token">
          <span class="muted">完整令牌</span>
          <div class="token-string-wrap">
            <n-text code class="inline-code" style="word-break: break-all">{{ createdToken?.access_token }}</n-text>
            <n-button text type="primary" aria-label="复制完整令牌" @click="handleCopyToken">
              <template #icon>
                <n-icon>
                  <CopyOutline />
                </n-icon>
              </template>
            </n-button>
          </div>
        </div>
      </n-space>

      <template #footer>
        <n-space justify="end">
          <n-button type="primary" @click="handleCopyAndClose">复制并关闭</n-button>
        </n-space>
      </template>
    </n-modal>

    <n-modal
      v-model:show="showModelsDialog"
      preset="card"
      title="配置令牌可访问模型"
      :style="{ width: '560px' }"
    >
      <n-form-item label="令牌" label-placement="top">
        <n-text code>{{ modelEditingToken?.name }}</n-text>
      </n-form-item>
      <n-form-item label="可访问模型" label-placement="top">
        <div class="form-stack">
          <n-select
            v-model:value="editingAllowedModels"
            :options="modelOptions"
            :loading="loadingModels"
            multiple
            filterable
            clearable
            placeholder="留空则不限制模型"
          />
          <p class="form-hint">不配置时默认不限制模型。</p>
        </div>
      </n-form-item>

      <template #footer>
        <n-space justify="end">
          <n-button @click="showModelsDialog = false">取消</n-button>
          <n-button type="primary" :loading="updatingModels" @click="handleUpdateTokenModels">保存</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import {
  type DataTableColumns,
  NAlert,
  NButton,
  NDatePicker,
  NForm,
  NFormItem,
  NIcon,
  NInput,
  NModal,
  NPagination,
  NRadio,
  NRadioGroup,
  NSelect,
  NSpace,
  NTag,
  NText,
  NTooltip,
  useDialog,
  useMessage,
} from 'naive-ui'
import {
  AddOutline,
  CopyOutline,
  PencilOutline,
  PowerOutline,
  SearchOutline,
  TrashOutline,
} from '@vicons/ionicons5'
import tokensService, { type CreateTokenResponse, type Token, type TokenListParams } from '@/services/tokensService'
import { modelApi } from '@/services/modelService'
import { toastApiError } from '@/utils/error'
import { formatCompactNumber } from '@/utils/number'

const dialog = useDialog()
const message = useMessage()

const isLoading = ref(false)
const listError = ref('')
const isCreating = ref(false)
const loadingModels = ref(false)
const updatingModels = ref(false)

const showCreateDialog = ref(false)
const showSuccessDialog = ref(false)
const showModelsDialog = ref(false)

const tokens = ref<Token[]>([])
const createdToken = ref<CreateTokenResponse | null>(null)
const modelOptions = ref<Array<{ label: string; value: string }>>([])

const modelEditingToken = ref<Token | null>(null)
const editingAllowedModels = ref<string[]>([])
const allModelNames = computed(() => modelOptions.value.map((item) => item.value))

const sortState = reactive({
  columnKey: 'created_at' as 'id' | 'status' | 'created_at' | 'last_used_at',
  order: 'desc' as 'asc' | 'desc',
})

const filters = reactive<TokenListParams>({
  name: '',
  status: null,
})

const statusOptions = [
  { label: '启用', value: 'active' },
  { label: '禁用', value: 'disabled' },
]

const formData = ref({
  name: '',
  allowed_models: [] as string[],
})

const expireType = ref<'never' | 'custom'>('never')
const expiresAtValue = ref<number | null>(null)

const rules = {
  name: [
    { required: true, message: '请输入令牌名称', trigger: 'blur' },
    { min: 2, max: 20, message: '名称长度应在 2-20 个字符', trigger: 'blur' },
  ],
}

const tableLocale = computed(() => ({
  emptyText: isLoading.value ? '加载中...' : '暂无令牌数据',
}))

const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0,
  showSizePicker: true,
  pageSizes: [10, 20, 50],
  onChange: (page: number) => {
    pagination.page = page
    loadTokens()
  },
  onUpdatePageSize: (size: number) => {
    pagination.pageSize = size
    pagination.page = 1
    loadTokens()
  },
})

function isDateDisabled(timestamp: number) {
  return timestamp < Date.now()
}

function handleExpireTypeChange(value: 'never' | 'custom') {
  if (value === 'never') {
    expiresAtValue.value = null
  }
}

function formatDateTime(timestamp: number) {
  const date = new Date(timestamp)
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hours = String(date.getHours()).padStart(2, '0')
  const minutes = String(date.getMinutes()).padStart(2, '0')
  const seconds = String(date.getSeconds()).padStart(2, '0')
  return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`
}

function getSortOrder(key: string) {
  if (sortState.columnKey === key) {
    return sortState.order === 'asc' ? 'ascend' : 'descend'
  }
  return false
}

function getTokenAvailableModels(row: Token) {
  if (row.allowed_models?.length) {
    return row.allowed_models
  }
  return allModelNames.value
}

function renderTokenModelsTooltip(row: Token) {
  const isUnrestricted = !row.allowed_models || row.allowed_models.length === 0
  const models = [...getTokenAvailableModels(row)].sort((a, b) => a.localeCompare(b))

  if (models.length === 0) {
    return isUnrestricted ? '不限制模型，当前暂无可展示的模型' : '当前未配置可用模型'
  }

  return h('div', { class: 'token-model-tooltip' }, [
    h('div', {
      style: {
        width: '560px',
        maxWidth: 'min(560px, calc(100vw - 48px))',
      },
    }, [
      h('div', {
        style: {
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          marginBottom: '8px',
          gap: '12px',
        },
      }, [
        h('span', {
          style: {
            fontSize: '12px',
            color: 'rgba(255, 255, 255, 0.82)',
          },
        }, isUnrestricted ? '全部模型' : '可用模型'),
        h('span', {
          style: {
            flexShrink: '0',
            padding: '1px 8px',
            borderRadius: '999px',
            fontSize: '11px',
            lineHeight: '18px',
            color: 'rgba(255, 255, 255, 0.72)',
            background: 'rgba(255, 255, 255, 0.08)',
          },
        }, `${models.length} 个`),
      ]),
      h(
        'div',
        {
          style: {
            overflow: 'visible',
          },
        },
        h(
          'div',
          {
            style: {
              fontSize: '12px',
              lineHeight: '1.7',
              color: 'rgba(255, 255, 255, 0.92)',
              wordBreak: 'break-word',
              whiteSpace: 'normal',
            },
          },
          models.join(', '),
        ),
      ),
    ]),
  ])
}

const columns = computed<DataTableColumns<Token>>(() => [
  {
    title: 'ID',
    key: 'id',
    width: 70,
    fixed: 'left',
    sortable: true,
    sorter: 'default',
    sortOrder: getSortOrder('id'),
  },
  {
    title: '名称',
    key: 'name',
    width: 180,
    fixed: 'left',
    ellipsis: { tooltip: true },
  },
  {
    title: '令牌',
    key: 'token_preview',
    width: 240,
    render: (row) => h(NText, { code: true, style: 'font-size: 12px' }, { default: () => row.token_preview }),
  },
  {
    title: '可用模型',
    key: 'allowed_models',
    width: 120,
    align: 'center',
    render: (row) => {
      const isUnrestricted = !row.allowed_models || row.allowed_models.length === 0
      const summary = isUnrestricted ? '全部' : `${row.allowed_models.length} 个`

      return h(
        NTooltip,
        { placement: 'top' },
        {
          trigger: () =>
            h(
              NTag,
              {
                size: 'small',
                type: isUnrestricted ? 'success' : 'info',
                bordered: false,
                round: true,
                style: 'cursor: help;',
              },
              { default: () => summary },
            ),
          default: () => renderTokenModelsTooltip(row),
        },
      )
    },
  },
  {
    title: 'Token 用量',
    key: 'token_usage',
    width: 180,
    align: 'right',
    render: (row) => `${formatCompactNumber(row.prompt_tokens)} / ${formatCompactNumber(row.completion_tokens)}`,
  },
  {
    title: '状态',
    key: 'status',
    width: 120,
    align: 'center',
    sortable: true,
    sorter: 'default',
    sortOrder: getSortOrder('status'),
    render: (row) => {
      const isExpired = row.expires_at ? new Date(row.expires_at).getTime() < Date.now() : false
      let type: 'success' | 'warning' | 'default' = 'success'
      let text = '启用'
      if (isExpired) {
        type = 'warning'
        text = '已过期'
      } else if (row.status !== 'active') {
        type = 'default'
        text = '禁用'
      }
      return h(NTag, { size: 'small', type, bordered: false }, { default: () => text })
    },
  },
  {
    title: '过期时间',
    key: 'expires_at',
    width: 180,
    align: 'center',
    render: (row) => row.expires_at || '永不过期',
  },
  {
    title: '创建时间',
    key: 'created_at',
    width: 180,
    align: 'center',
    sortable: true,
    sorter: 'default',
    sortOrder: getSortOrder('created_at'),
  },
  {
    title: '最后使用',
    key: 'last_used_at',
    width: 180,
    align: 'center',
    sortable: true,
    sorter: 'default',
    sortOrder: getSortOrder('last_used_at'),
    render: (row) => row.last_used_at || '-',
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
            tooltip: '复制令牌',
            ariaLabel: `复制令牌 ${row.name}`,
            icon: CopyOutline,
            onClick: () => handleCopyTokenFromList(row),
          }),
          renderActionIcon({
            tooltip: '模型权限',
            ariaLabel: `配置令牌 ${row.name} 的模型权限`,
            icon: PencilOutline,
            type: 'info',
            onClick: () => openTokenModelsDialog(row),
          }),
          renderActionIcon({
            tooltip: row.status === 'active' ? '禁用' : '启用',
            ariaLabel: `${row.status === 'active' ? '禁用' : '启用'}令牌 ${row.name}`,
            icon: PowerOutline,
            type: 'warning',
            onClick: () => handleToggleStatus(row),
          }),
          renderActionIcon({
            tooltip: '删除',
            ariaLabel: `删除令牌 ${row.name}`,
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

function handleSorterChange(sorter: { columnKey: string; order: 'ascend' | 'descend' | false }) {
  if (sorter.columnKey) {
    sortState.columnKey = sorter.columnKey as 'id' | 'status' | 'created_at' | 'last_used_at'
    sortState.order = sorter.order === 'ascend' ? 'asc' : sorter.order === 'descend' ? 'desc' : 'desc'
  } else {
    sortState.columnKey = 'created_at'
    sortState.order = 'desc'
  }

  pagination.page = 1
  loadTokens()
}

function handleSearch() {
  pagination.page = 1
  loadTokens()
}

function handleReset() {
  filters.name = ''
  filters.status = null
  pagination.page = 1
  loadTokens()
}

async function loadModelOptions() {
  loadingModels.value = true
  try {
    const result = await modelApi.list({ page: 1, page_size: 1000 })
    modelOptions.value = result.items.map((item) => ({
      label: item.name,
      value: item.name,
    }))
  } catch {
    message.error('加载模型选项失败')
  } finally {
    loadingModels.value = false
  }
}

async function loadTokens() {
  isLoading.value = true
  listError.value = ''
  try {
    const params: TokenListParams = {
      page: pagination.page,
      page_size: pagination.pageSize,
      name: filters.name || undefined,
      status: filters.status || undefined,
      sort_by: sortState.columnKey,
      sort_order: sortState.order,
    }

    const result = await tokensService.list(params)
    tokens.value = result.items
    pagination.total = result.total
  } catch {
    listError.value = '无法加载令牌列表'
    message.error(listError.value)
  } finally {
    isLoading.value = false
  }
}

async function handleCreate() {
  if (!formData.value.name) {
    message.error('请输入令牌名称')
    return
  }

  if (expireType.value === 'custom' && !expiresAtValue.value) {
    message.error('请选择过期时间')
    return
  }

  isCreating.value = true
  try {
    let expiresAt = ''
    if (expireType.value === 'custom' && expiresAtValue.value) {
      expiresAt = formatDateTime(expiresAtValue.value)
    }

    createdToken.value = await tokensService.createToken({
      name: formData.value.name,
      expires_at: expiresAt,
      allowed_models: formData.value.allowed_models,
    })

    showCreateDialog.value = false
    showSuccessDialog.value = true

    formData.value.name = ''
    formData.value.allowed_models = []
    expireType.value = 'never'
    expiresAtValue.value = null

    await loadTokens()
  } catch (err) {
    toastApiError(err, '创建令牌失败')
  } finally {
    isCreating.value = false
  }
}

async function handleCopyToken() {
  if (!createdToken.value?.access_token) return

  try {
    await navigator.clipboard.writeText(createdToken.value.access_token)
    message.success('已复制到剪贴板')
  } catch {
    message.error('复制失败，请手动复制')
  }
}

async function handleCopyAndClose() {
  await handleCopyToken()
  showSuccessDialog.value = false
  createdToken.value = null
}

async function handleCopyTokenFromList(token: Token) {
  try {
    await navigator.clipboard.writeText(token.token)
    message.success('令牌已复制')
  } catch {
    message.error('复制失败，请手动复制')
  }
}

function openTokenModelsDialog(token: Token) {
  modelEditingToken.value = token
  editingAllowedModels.value = [...(token.allowed_models || [])]
  showModelsDialog.value = true
}

async function handleUpdateTokenModels() {
  if (!modelEditingToken.value) return

  updatingModels.value = true
  try {
    await tokensService.updateTokenModels(modelEditingToken.value.id, editingAllowedModels.value)
    message.success('模型权限已更新')
    showModelsDialog.value = false
    modelEditingToken.value = null
    await loadTokens()
  } catch (err) {
    toastApiError(err, '更新失败')
  } finally {
    updatingModels.value = false
  }
}

function handleToggleStatus(token: Token) {
  dialog.warning({
    title: '确认操作',
    content: `确定要${token.status === 'active' ? '禁用' : '启用'}该令牌吗？`,
    positiveText: '确认',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await tokensService.toggleTokenStatus(token.id)
        message.success('令牌状态已更新')
        await loadTokens()
      } catch {
        message.error('更新失败')
      }
    },
  })
}

function handleDelete(token: Token) {
  dialog.warning({
    title: '删除确认',
    content: `确定删除令牌“${token.name}”吗？该操作不可恢复。`,
    positiveText: '确认删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await tokensService.deleteToken(token.id)
        message.success('令牌已删除')
        await loadTokens()
      } catch {
        message.error('删除失败')
      }
    },
  })
}

onMounted(() => {
  loadModelOptions()
  loadTokens()
})
</script>

<style scoped>
.table-toolbar__actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: auto;
}

.form-stack {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 0;
}

.token-info-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 8px 0;
}

.token-info-row--token {
  align-items: flex-start;
}

.token-string-wrap {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  max-width: 420px;
}

.token-model-tooltip {
  width: 560px;
  max-width: min(560px, calc(100vw - 48px));
}

.token-model-tooltip__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
  gap: 12px;
}

.token-model-tooltip__title {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.82);
}

.token-model-tooltip__count {
  flex-shrink: 0;
  padding: 1px 8px;
  border-radius: 999px;
  font-size: 11px;
  line-height: 18px;
  color: rgba(255, 255, 255, 0.72);
  background: rgba(255, 255, 255, 0.08);
}

.token-model-tooltip__list {
  overflow: visible;
}

.token-model-tooltip__content {
  font-size: 12px;
  line-height: 1.7;
  color: rgba(255, 255, 255, 0.92);
  word-break: break-word;
  white-space: normal;
}
</style>
