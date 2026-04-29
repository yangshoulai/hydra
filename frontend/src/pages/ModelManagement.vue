<template>
  <div class="app-page">
    <n-alert v-if="listError" type="error" :bordered="false">
      <n-space align="center" justify="space-between" style="width: 100%">
        <span>{{ listError }}</span>
        <n-button text type="error" @click="loadModels">重试</n-button>
      </n-space>
    </n-alert>

    <section class="panel-card">
      <header class="panel-card__header table-toolbar">
        <div class="table-toolbar__title">
          <h3 class="panel-card__title">模型列表</h3>
        </div>
        <div class="table-toolbar__actions">
          <n-button size="small" type="primary" @click="showCreateDialog = true">
            <template #icon>
              <n-icon>
                <AddOutline/>
              </n-icon>
            </template>
            添加模型
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
              placeholder="模型名称"
              @keyup.enter="handleSearch"
          >
            <template #prefix>
              <n-icon>
                <SearchOutline/>
              </n-icon>
            </template>
          </n-input>
          <n-select
              v-model:value="filters.provider_id"
              class="table-filter-select"
              size="small"
              clearable
              filterable
              placeholder="厂商"
              :options="providerOptions"
          />
          <n-button size="small" type="primary" @click="handleSearch">查询</n-button>
          <n-button size="small" quaternary @click="handleReset">重置</n-button>
        </div>

        <n-data-table
            :columns="columns"
            :data="models"
            :pagination="false"
            :loading="loading"
            :locale="tableLocale"
            :single-line="false"
            :scroll-x="1040"
            :row-key="(row: Model) => row.id"
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

    <n-modal v-model:show="showCreateDialog" preset="card" title="添加模型" style="width: 520px">
      <n-form ref="createFormRef" :model="createForm" :rules="createRules" label-placement="top" size="medium">
        <n-form-item label="模型名称" path="name">
          <n-input v-model:value="createForm.name" placeholder="例如：gpt-4o" @input="createForm.name = createForm.name.toLowerCase()"/>
        </n-form-item>
        <n-form-item label="所属厂商" path="provider_id">
          <n-select v-model:value="createForm.provider_id" :options="providerOptions" filterable :loading="loadingProviders" placeholder="请选择厂商"/>
        </n-form-item>
        <n-form-item label="备注" path="remark">
          <n-input v-model:value="createForm.remark" type="textarea" :autosize="{ minRows: 2, maxRows: 5 }" placeholder="可选备注"/>
        </n-form-item>
      </n-form>

      <template #footer>
        <n-space justify="end">
          <n-button @click="showCreateDialog = false">取消</n-button>
          <n-button type="primary" :loading="creating" @click="handleCreate">确认创建</n-button>
        </n-space>
      </template>
    </n-modal>

    <n-modal v-model:show="showEditDialog" preset="card" title="编辑模型" style="width: 520px">
      <n-form ref="editFormRef" :model="editForm" :rules="editRules" label-placement="top" size="medium">
        <n-form-item label="模型名称" path="name">
          <n-input v-model:value="editForm.name" placeholder="请输入模型名称" @input="editForm.name = editForm.name?.toLowerCase()"/>
        </n-form-item>
        <n-form-item label="所属厂商" path="provider_id">
          <n-select v-model:value="editForm.provider_id" :options="providerOptions" filterable :loading="loadingProviders" placeholder="请选择厂商"/>
        </n-form-item>
        <n-form-item label="备注" path="remark">
          <n-input v-model:value="editForm.remark" type="textarea" :autosize="{ minRows: 2, maxRows: 5 }" placeholder="可选备注"/>
        </n-form-item>
      </n-form>

      <template #footer>
        <n-space justify="end">
          <n-button @click="showEditDialog = false">取消</n-button>
          <n-button type="primary" :loading="updating" @click="handleUpdate">保存</n-button>
        </n-space>
      </template>
    </n-modal>

    <ModelChannelsDrawer :model-id="selectedModelId" :model-name="selectedModelName" v-model:show="showChannelsDrawer"/>
  </div>
</template>

<script setup lang="ts">
import {computed, h, onMounted, reactive, ref} from 'vue'
import {
  type DataTableColumns,
  type FormInst,
  type FormRules,
  NAlert,
  NButton,
  NDataTable,
  NForm,
  NFormItem,
  NIcon,
  NInput,
  NModal,
  NPagination,
  NSelect,
  NSpace,
  NText,
  NTooltip,
} from 'naive-ui'
import {AddOutline, CopyOutline, LayersOutline, PencilOutline, SearchOutline, TrashOutline} from '@vicons/ionicons5'
import {type CreateModelRequest, modelApi, type UpdateModelRequest} from '@/services/modelService'
import type {Model, ModelListParams, Provider} from '@/types/model'
import { providerApi } from '@/services/providerService'
import ProviderIcon from '@/components/ProviderIcon.vue'
import ModelChannelsDrawer from '@/components/ModelChannelsDrawer.vue'
import {getErrorMessage, toastApiError} from '@/utils/error'
import { feedback } from '@/services/feedback'

const loading = ref(false)
const listError = ref('')
const creating = ref(false)
const updating = ref(false)
const loadingProviders = ref(false)

const models = ref<Model[]>([])
const providers = ref<Provider[]>([])

const showCreateDialog = ref(false)
const showEditDialog = ref(false)
const showChannelsDrawer = ref(false)

const currentEditModel = ref<Model | null>(null)
const selectedModelId = ref(0)
const selectedModelName = ref('')

const filters = reactive<ModelListParams>({
  name: '',
  provider_id: null,
})

const sortState = reactive({
  columnKey: '' as 'id' | 'name' | '',
  order: false as boolean | 'asc' | 'desc',
})

const providerOptions = ref<Array<{ label: string; value: string }>>([])

const createFormRef = ref<FormInst | null>(null)
const editFormRef = ref<FormInst | null>(null)

const createForm = reactive<CreateModelRequest>({
  name: '',
  provider_id: null,
  remark: '',
})

const editForm = reactive<UpdateModelRequest>({
  name: '',
  provider_id: undefined,
  remark: '',
})

const createRules: FormRules = {
  name: {required: true, message: '请输入模型名称', trigger: ['blur', 'input']},
  provider_id: {required: true, type: 'string', message: '请选择厂商', trigger: ['blur', 'change']},
}

const editRules: FormRules = {
  name: {required: true, message: '请输入模型名称', trigger: ['blur', 'input']},
}

const tableLocale = computed(() => ({
  emptyText: loading.value ? '加载中...' : '暂无模型数据',
}))

const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0,
  showSizePicker: true,
  pageSizes: [10, 20, 50],
  onChange: (page: number) => {
    pagination.page = page
    loadModels()
  },
  onUpdatePageSize: (size: number) => {
    pagination.pageSize = size
    pagination.page = 1
    loadModels()
  },
})

function getSortOrder(key: string) {
  if (sortState.columnKey === key) {
    return sortState.order === 'asc' ? 'ascend' : sortState.order === 'desc' ? 'descend' : false
  }
  return false
}

const columns = computed<DataTableColumns<Model>>(() => [
  {
    title: 'ID',
    key: 'id',
    width: 70,
    sortable: true,
    sorter: 'default',
    sortOrder: getSortOrder('id'),
  },
  {
    title: '模型名称',
    key: 'name',
    width: 280,
    sortable: true,
    sorter: 'default',
    sortOrder: getSortOrder('name'),
    render: (row) =>
        h(NSpace, {size: 8, align: 'center'}, {
          default: () => [
            h(NText, {code: true, style: 'max-width: 280px'}, {default: () => row.name}),
            h(
                NTooltip,
                null,
                {
                  trigger: () =>
                      h(
                          NButton,
                          {
                            class: 'table-action-btn',
                            quaternary: true,
                            circle: true,
                            size: 'tiny',
                            'aria-label': `复制模型名 ${row.name}`,
                            onClick: (event: MouseEvent) => handleCopyModelName(row, event),
                          },
                          {icon: () => h(NIcon, null, {default: () => h(CopyOutline)})},
                      ),
                  default: () => '复制模型名',
                },
            ),
          ],
        }),
  },
  {
    title: '厂商',
    key: 'provider',
    width: 90,
    align: 'center',
    render: (row) => {
      const label = row.provider?.name || '未设置'
      return h(
          NTooltip,
          null,
          {
            trigger: () =>
                h(
                    'span',
                    {class: 'provider-cell'},
                    [h(ProviderIcon, {iconURL: row.provider?.icon, alt: label, size: 22})],
                ),
            default: () => label,
          },
      )
    },
  },
  {
    title: '渠道数',
    key: 'channel_count',
    width: 120,
    align: 'center',
    render: (row) => row.channel_count ?? 0,
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
    ellipsis: {tooltip: true},
  },
  {
    title: '操作',
    key: 'actions',
    width: 140,
    fixed: 'right',
    align: 'center',
    render: (row) =>
        h(NSpace, {size: 4, justify: 'center', class: 'table-action-group'}, {
          default: () => [
            renderActionIcon({
              tooltip: '关联渠道',
              ariaLabel: `查看模型 ${row.name} 的关联渠道`,
              icon: LayersOutline,
              type: 'info',
              onClick: () => handleViewChannels(row),
            }),
            renderActionIcon({
              tooltip: '编辑',
              ariaLabel: `编辑模型 ${row.name}`,
              icon: PencilOutline,
              type: 'warning',
              onClick: () => handleEdit(row),
            }),
            renderActionIcon({
              tooltip: '删除',
              ariaLabel: `删除模型 ${row.name}`,
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
                  icon: () => h(NIcon, null, {default: () => h(options.icon)}),
                },
            ),
        default: () => options.tooltip,
      },
  )
}

function handleSorterChange(sorter: { columnKey: string; order: 'ascend' | 'descend' | false }) {
  if (sorter.columnKey) {
    sortState.columnKey = sorter.columnKey as 'id' | 'name'
    sortState.order = sorter.order === 'ascend' ? 'asc' : sorter.order === 'descend' ? 'desc' : false
  } else {
    sortState.columnKey = ''
    sortState.order = false
  }

  pagination.page = 1
  loadModels()
}

function handleSearch() {
  pagination.page = 1
  loadModels()
}

function handleReset() {
  filters.name = ''
  filters.provider_id = null
  pagination.page = 1
  loadModels()
}

async function loadModels() {
  loading.value = true
  listError.value = ''
  try {
    const params: ModelListParams = {
      page: pagination.page,
      page_size: pagination.pageSize,
      name: filters.name || undefined,
      provider_id: filters.provider_id || undefined,
    }

    if (sortState.columnKey && sortState.order) {
      params.sort_by = sortState.columnKey
      params.sort_order = sortState.order as 'asc' | 'desc'
    }

    const result = await modelApi.list(params)
    models.value = result.items
    pagination.total = result.total
  } catch (err) {
    listError.value = getErrorMessage(err, '加载模型列表失败')
    feedback.message?.error(listError.value)
  } finally {
    loading.value = false
  }
}

async function loadProviders() {
  loadingProviders.value = true
  try {
    const providerList = await providerApi.list()
    providers.value = providerList
    providerOptions.value = providerList.map((provider) => ({
      label: provider.name,
      value: provider.id,
    }))
  } catch (err) {
    toastApiError(err, '加载厂商列表失败')
  } finally {
    loadingProviders.value = false
  }
}

async function handleCreate() {
  if (!createFormRef.value) return

  try {
    await createFormRef.value.validate()
    creating.value = true

    await modelApi.create(createForm)

    feedback.message?.success('模型创建成功')
    showCreateDialog.value = false

    createForm.name = ''
    createForm.provider_id = null
    createForm.remark = ''

    await loadModels()
  } catch (err) {
    if (!(err as { errors?: unknown })?.errors) {
      toastApiError(err, '创建失败')
    }
  } finally {
    creating.value = false
  }
}

function handleEdit(model: Model) {
  currentEditModel.value = model
  editForm.name = model.name
  editForm.provider_id = model.provider_id ?? undefined
  editForm.remark = model.remark
  showEditDialog.value = true
}

async function handleUpdate() {
  if (!editFormRef.value || !currentEditModel.value) return

  try {
    await editFormRef.value.validate()
    updating.value = true

    await modelApi.update(currentEditModel.value.id, editForm)

    feedback.message?.success('更新成功')
    showEditDialog.value = false
    currentEditModel.value = null

    await loadModels()
  } catch (err) {
    if (!(err as { errors?: unknown })?.errors) {
      toastApiError(err, '更新失败')
    }
  } finally {
    updating.value = false
  }
}

async function handleDelete(model: Model) {
  await feedback.dialog?.warning({
    title: '确认删除',
    content: `确定删除模型“${model.name}”吗？`,
    positiveText: '确认删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await modelApi.delete(model.id)
        feedback.message?.success('删除成功')
        await loadModels()
      } catch (err) {
        toastApiError(err, '删除失败')
      }
    },
  })
}

function handleViewChannels(model: Model) {
  selectedModelId.value = model.id
  selectedModelName.value = model.name
  showChannelsDrawer.value = true
}

function handleCopyModelName(model: Model, event: MouseEvent) {
  event.stopPropagation()
  navigator.clipboard
      .writeText(model.name)
      .then(() => feedback.message?.success(`已复制模型名：${model.name}`))
      .catch(() => feedback.message?.error('复制失败'))
}

onMounted(() => {
  loadProviders()
  loadModels()
})
</script>

<style scoped>
.table-toolbar__actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: auto;
}

.provider-cell {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: default;
}
</style>
