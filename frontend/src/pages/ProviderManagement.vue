<template>
  <div class="app-page">
    <n-alert v-if="listError" type="error" :bordered="false">
      <n-space align="center" justify="space-between" style="width: 100%">
        <span>{{ listError }}</span>
        <n-button text type="error" @click="loadProviders">重试</n-button>
      </n-space>
    </n-alert>

    <section class="panel-card">
      <header class="panel-card__header table-toolbar">
        <div class="table-toolbar__title">
          <h3 class="panel-card__title">厂商列表</h3>
        </div>
        <div class="table-toolbar__actions">
          <n-button size="small" secondary :loading="syncing" @click="handleSync">同步远程厂商</n-button>
          <n-button size="small" type="primary" @click="showCreateDialog = true">
            <template #icon>
              <n-icon>
                <AddOutline />
              </n-icon>
            </template>
            添加厂商
          </n-button>
        </div>
      </header>
      <div class="panel-card__body">
        <n-data-table
          :columns="columns"
          :data="providers"
          :loading="loading"
          :locale="tableLocale"
          :pagination="pagination"
          :single-line="false"
          :scroll-x="960"
        />
      </div>
    </section>

    <n-modal v-model:show="showCreateDialog" preset="card" title="添加厂商" style="width: 520px">
      <n-form ref="createFormRef" :model="createForm" :rules="createRules" label-placement="top" size="medium">
        <n-form-item label="厂商 ID" path="id">
          <n-input v-model:value="createForm.id" placeholder="例如：openai" @input="createForm.id = createForm.id.toLowerCase()" />
        </n-form-item>
        <n-form-item label="厂商名称" path="name">
          <n-input v-model:value="createForm.name" placeholder="例如：OpenAI" />
        </n-form-item>
        <n-form-item label="图标 URL" path="icon">
          <n-input v-model:value="createForm.icon" placeholder="可选" />
        </n-form-item>
        <n-form-item label="备注" path="remark">
          <n-input v-model:value="createForm.remark" type="textarea" :autosize="{ minRows: 2, maxRows: 5 }" placeholder="可选备注" />
        </n-form-item>
      </n-form>

      <template #footer>
        <n-space justify="end">
          <n-button @click="showCreateDialog = false">取消</n-button>
          <n-button type="primary" :loading="creating" @click="handleCreate">确认创建</n-button>
        </n-space>
      </template>
    </n-modal>

    <n-modal v-model:show="showEditDialog" preset="card" title="编辑厂商" style="width: 520px">
      <n-form ref="editFormRef" :model="editForm" :rules="editRules" label-placement="top" size="medium">
        <n-form-item label="厂商 ID">
          <n-input :value="currentEditProvider?.id" disabled />
        </n-form-item>
        <n-form-item label="厂商名称" path="name">
          <n-input v-model:value="editForm.name" placeholder="例如：OpenAI" />
        </n-form-item>
        <n-form-item label="图标 URL" path="icon">
          <n-input v-model:value="editForm.icon" placeholder="可选" />
        </n-form-item>
        <n-form-item label="备注" path="remark">
          <n-input v-model:value="editForm.remark" type="textarea" :autosize="{ minRows: 2, maxRows: 5 }" placeholder="可选备注" />
        </n-form-item>
      </n-form>

      <template #footer>
        <n-space justify="end">
          <n-button @click="showEditDialog = false">取消</n-button>
          <n-button type="primary" :loading="updating" @click="handleUpdate">保存</n-button>
        </n-space>
      </template>
    </n-modal>

    <n-modal
      v-model:show="showSyncDialog"
      preset="card"
      title="同步远程厂商"
      style="width: 860px"
    >
      <n-space vertical :size="10">
        <n-input
          v-model:value="syncKeyword"
          clearable
          placeholder="搜索厂商 ID / 名称"
          style="max-width: 320px"
        />

        <n-data-table
          :columns="syncColumns"
          :data="filteredRemoteProviders"
          :row-key="(row: RemoteProvider) => row.id"
          v-model:checked-row-keys="checkedProviderIds"
          :pagination="syncPagination"
          :locale="{ emptyText: '无匹配厂商' }"
          :single-line="false"
        />

        <n-space justify="end">
          <n-button @click="showSyncDialog = false">取消</n-button>
          <n-button type="primary" :loading="adding" :disabled="selectedImportableCount === 0" @click="handleAddSyncProviders">
            导入新增厂商 ({{ selectedImportableCount }})
          </n-button>
        </n-space>
      </n-space>
    </n-modal>
    <n-modal
      v-model:show="showModelsDialog"
      preset="card"
      :title="modelsDialogTitle"
      style="width: 720px"
    >
      <n-data-table
        :columns="modelsColumns"
        :data="providerModels"
        :loading="loadingModels"
        :pagination="false"
        :single-line="false"
        :locale="{ emptyText: loadingModels ? '加载中...' : '该厂商暂无模型' }"
        :row-key="(row: Model) => row.id"
        :max-height="520"
      />
      <template #footer>
        <n-space justify="end">
          <n-button @click="showModelsDialog = false">关闭</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, h, nextTick, onMounted, reactive, ref, watch } from 'vue'
import type { DataTableColumns, FormInst, FormRules } from 'naive-ui'
import {
  NAlert,
  NButton,
  NDataTable,
  NForm,
  NFormItem,
  NIcon,
  NInput,
  NModal,
  NSpace,
  NTag,
  NText,
  NTooltip,
} from 'naive-ui'
import { AddOutline, CubeOutline, PencilOutline, TrashOutline } from '@vicons/ionicons5'
import { providerApi } from '@/services/providerService'
import { modelApi } from '@/services/modelService'
import type { CreateProviderRequest, Model, Provider, RemoteProvider, UpdateProviderRequest } from '@/types/model'
import ProviderIcon from '@/components/ProviderIcon.vue'
import { getErrorMessage, toastApiError } from '@/utils/error'
import { feedback } from '@/services/feedback'

const loading = ref(false)
const listError = ref('')
const creating = ref(false)
const updating = ref(false)
const syncing = ref(false)
const adding = ref(false)

const providers = ref<Provider[]>([])
const remoteProviders = ref<RemoteProvider[]>([])
const checkedProviderIds = ref<string[]>([])
const syncKeyword = ref('')

const showCreateDialog = ref(false)
const showEditDialog = ref(false)
const showSyncDialog = ref(false)
const showModelsDialog = ref(false)
const currentEditProvider = ref<Provider | null>(null)
const currentModelsProvider = ref<Provider | null>(null)
const providerModels = ref<Model[]>([])
const loadingModels = ref(false)

const createFormRef = ref<FormInst | null>(null)
const editFormRef = ref<FormInst | null>(null)

const createForm = reactive<CreateProviderRequest>({
  id: '',
  name: '',
  icon: '',
  remark: '',
})

const editForm = reactive<UpdateProviderRequest>({
  name: '',
  icon: '',
  remark: '',
})

const createRules: FormRules = {
  id: { required: true, message: '请输入厂商 ID', trigger: ['blur', 'input'] },
  name: { required: true, message: '请输入厂商名称', trigger: ['blur', 'input'] },
}

const editRules: FormRules = {
  name: { required: true, message: '请输入厂商名称', trigger: ['blur', 'input'] },
}

const localProviderIds = computed(() => new Set(providers.value.map((item) => item.id)))

const filteredRemoteProviders = computed(() => {
  const keyword = syncKeyword.value.trim().toLowerCase()
  const rows = remoteProviders.value.filter((item) => {
    if (!keyword) return true
    return item.id.toLowerCase().includes(keyword) || item.name.toLowerCase().includes(keyword)
  })
  return rows.sort((a, b) => {
    const aLocal = localProviderIds.value.has(a.id)
    const bLocal = localProviderIds.value.has(b.id)
    if (aLocal !== bLocal) return aLocal ? -1 : 1
    return a.name.localeCompare(b.name)
  })
})

const selectedImportableCount = computed(() =>
  checkedProviderIds.value.filter((id) => !localProviderIds.value.has(id)).length,
)

const tableLocale = computed(() => ({
  emptyText: loading.value ? '加载中...' : '暂无厂商数据',
}))

const columns: DataTableColumns<Provider> = [
  {
    title: '图标',
    key: 'icon',
    width: 80,
    align: 'center',
    render: (row) => h(ProviderIcon, { iconURL: row.icon, alt: row.name, size: 24 }),
  },
  {
    title: '厂商 ID',
    key: 'id',
    width: 180,
    sorter: (a, b) => a.id.localeCompare(b.id),
  },
  {
    title: '厂商名称',
    key: 'name',
    width: 200,
    render: (row) => h(NText, { strong: true }, { default: () => row.name }),
    sorter: (a, b) => a.name.localeCompare(b.name),
  },
  {
    title: '模型数',
    key: 'model_count',
    width: 100,
    align: 'center',
    render: (row) => row.model_count ?? 0,
  },
  {
    title: '创建时间',
    key: 'created_at',
    width: 180,
    align: 'center',
    render: (row) => new Date(row.created_at).toLocaleString('zh-CN'),
  },
  {
    title: '备注',
    key: 'remark',
    minWidth: 160,
    ellipsis: { tooltip: true },
  },
  {
    title: '操作',
    key: 'actions',
    width: 130,
    fixed: 'right',
    align: 'center',
    render: (row) =>
      h(NSpace, { size: 4, justify: 'center', class: 'table-action-group' }, {
        default: () => [
          renderActionIcon({
            tooltip: '模型列表',
            ariaLabel: `查看厂商 ${row.name} 的模型列表`,
            icon: CubeOutline,
            type: 'info',
            onClick: () => openModelsDialog(row),
          }),
          renderActionIcon({
            tooltip: '编辑',
            ariaLabel: `编辑厂商 ${row.name}`,
            icon: PencilOutline,
            type: 'warning',
            onClick: () => handleEdit(row),
          }),
          renderActionIcon({
            tooltip: '删除',
            ariaLabel: `删除厂商 ${row.name}`,
            icon: TrashOutline,
            type: 'error',
            onClick: () => handleDelete(row),
          }),
        ],
      }),
  },
]

const modelsDialogTitle = computed(() => {
  if (!currentModelsProvider.value) return '模型列表'
  return `模型列表 · ${currentModelsProvider.value.name}（${providerModels.value.length}）`
})

const modelsColumns: DataTableColumns<Model> = [
  {
    title: 'ID',
    key: 'id',
    width: 70,
    align: 'right',
  },
  {
    title: '模型名称',
    key: 'name',
    minWidth: 220,
    render: (row) => h(NText, { code: true, style: 'font-size: 12px' }, { default: () => row.name }),
  },
  {
    title: '备注',
    key: 'remark',
    minWidth: 160,
    ellipsis: { tooltip: true },
  },
  {
    title: '创建时间',
    key: 'created_at',
    width: 170,
    align: 'center',
    render: (row) => new Date(row.created_at).toLocaleString('zh-CN'),
  },
]

async function openModelsDialog(provider: Provider) {
  currentModelsProvider.value = provider
  providerModels.value = []
  showModelsDialog.value = true
  loadingModels.value = true
  try {
    const result = await modelApi.list({ provider_id: provider.id, page: 1, page_size: 1000 })
    providerModels.value = result.items
  } catch (err) {
    toastApiError(err, '加载模型列表失败')
  } finally {
    loadingModels.value = false
  }
}

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

const pagination = reactive({
  page: 1,
  pageSize: 20,
  showSizePicker: true,
  pageSizes: [10, 20, 50],
  onChange: (page: number) => {
    pagination.page = page
  },
  onUpdatePageSize: (size: number) => {
    pagination.pageSize = size
    pagination.page = 1
  },
})

const syncColumns: DataTableColumns<RemoteProvider> = [
  { type: 'selection' },
  {
    title: '图标',
    key: 'iconURL',
    width: 80,
    align: 'center',
    render: (row) => h(ProviderIcon, { iconURL: row.iconURL, alt: row.name, size: 20 }),
  },
  { title: '厂商 ID', key: 'id', width: 180 },
  { title: '厂商名称', key: 'name', minWidth: 260 },
  {
    title: '本地状态',
    key: 'status',
    width: 120,
    render: (row) =>
      h(
        NTag,
        {
          bordered: false,
          type: localProviderIds.value.has(row.id) ? 'info' : 'success',
          size: 'small',
        },
        { default: () => (localProviderIds.value.has(row.id) ? '已存在' : '可导入') },
      ),
  },
]

const syncPagination = reactive({
  page: 1,
  pageSize: 10,
  showSizePicker: true,
  pageSizes: [10, 20, 50],
  onChange: (page: number) => {
    syncPagination.page = page
  },
  onUpdatePageSize: (size: number) => {
    syncPagination.pageSize = size
    syncPagination.page = 1
  },
})

async function loadProviders() {
  loading.value = true
  listError.value = ''
  try {
    providers.value = await providerApi.list()
  } catch (err) {
    listError.value = getErrorMessage(err, '加载厂商列表失败')
    feedback.message?.error(listError.value)
  } finally {
    loading.value = false
  }
}

async function handleCreate() {
  if (!createFormRef.value) return

  try {
    await createFormRef.value.validate()
    creating.value = true

    await providerApi.create(createForm)

    feedback.message?.success('厂商创建成功')
    showCreateDialog.value = false

    createForm.id = ''
    createForm.name = ''
    createForm.icon = ''
    createForm.remark = ''

    await loadProviders()
  } catch (err) {
    if (!(err as { errors?: unknown })?.errors) {
      toastApiError(err, '创建失败')
    }
  } finally {
    creating.value = false
  }
}

function handleEdit(provider: Provider) {
  currentEditProvider.value = provider
  editForm.name = provider.name
  editForm.icon = provider.icon
  editForm.remark = provider.remark
  showEditDialog.value = true
}

async function handleUpdate() {
  if (!editFormRef.value || !currentEditProvider.value) return

  try {
    await editFormRef.value.validate()
    updating.value = true

    await providerApi.update(currentEditProvider.value.id, editForm)

    feedback.message?.success('更新成功')
    showEditDialog.value = false
    currentEditProvider.value = null

    await loadProviders()
  } catch (err) {
    if (!(err as { errors?: unknown })?.errors) {
      toastApiError(err, '更新失败')
    }
  } finally {
    updating.value = false
  }
}

async function handleDelete(provider: Provider) {
  await feedback.dialog?.warning({
    title: '确认删除',
    content: `确定删除厂商“${provider.name}”吗？`,
    positiveText: '确认删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await providerApi.delete(provider.id)
        feedback.message?.success('删除成功')
        await loadProviders()
      } catch (err) {
        toastApiError(err, '删除失败')
      }
    },
  })
}

async function handleSync() {
  syncing.value = true
  try {
    const remoteData = await providerApi.syncRemoteProviders()
    remoteProviders.value = remoteData
    checkedProviderIds.value = remoteData.filter((item) => localProviderIds.value.has(item.id)).map((item) => item.id)
    syncKeyword.value = ''
    syncPagination.page = 1

    await nextTick()
    showSyncDialog.value = true
    feedback.message?.success(`已拉取远程厂商 ${remoteData.length} 条`)
  } catch (err) {
    toastApiError(err, '同步失败')
  } finally {
    syncing.value = false
  }
}

async function handleAddSyncProviders() {
  if (!checkedProviderIds.value.length) {
    feedback.message?.warning('请先选择要导入的厂商')
    return
  }

  adding.value = true
  try {
    const selectedProviders = remoteProviders.value.filter(
      (item) => checkedProviderIds.value.includes(item.id) && !localProviderIds.value.has(item.id),
    )

    if (selectedProviders.length === 0) {
      feedback.message?.info('当前选中项均已存在，本次无需导入')
      return
    }

    const payload: CreateProviderRequest[] = selectedProviders.map((item) => ({
      id: item.id,
      name: item.name,
      icon: item.iconURL,
      remark: item.name,
    }))

    const result = await providerApi.batchCreate(payload)
    if (result.created > 0) {
      feedback.message?.success(`导入成功 ${result.created} 条${result.failed > 0 ? `，失败 ${result.failed} 条` : ''}`)
      showSyncDialog.value = false
      await loadProviders()
    } else {
      feedback.message?.error('没有可导入的厂商')
    }
  } catch (err) {
    toastApiError(err, '导入失败')
  } finally {
    adding.value = false
  }
}

watch(showSyncDialog, (open) => {
  if (!open) {
    checkedProviderIds.value = []
    remoteProviders.value = []
    syncKeyword.value = ''
    syncPagination.page = 1
  }
})

onMounted(() => {
  loadProviders()
})
</script>

<style scoped>
.table-toolbar__actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: auto;
}
</style>
