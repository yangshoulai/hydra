<template>
  <div class="space-y-4">
    <!-- 页面头部 -->
    <n-card :bordered="false" class="page-header-card">
      <n-space justify="space-between" align="center">
        <n-space vertical :size="4">
          <n-text class="page-title">统一模型管理</n-text>
          <n-text depth="3" class="page-subtitle">
            管理系统中的所有统一模型，用于将不同渠道的模型映射到统一名称
          </n-text>
        </n-space>
        <n-button type="primary" @click="showCreateDialog = true" size="large" strong>
          <template #icon>
            <n-icon>
              <AddOutline/>
            </n-icon>
          </template>
          添加模型
        </n-button>
      </n-space>
    </n-card>

    <!-- 模型列表 -->
    <n-data-table
        :columns="columns"
        :data="models"
        :pagination="false"
        :bordered="true"
        striped
        :single-line="false"
        :loading="loading"
        :scroll-x="1160"
    />

    <!-- 创建模型对话框 -->
    <n-modal
        v-model:show="showCreateDialog"
        preset="card"
        title="添加模型"
        :style="{ width: '600px' }"
        :mask-closable="false"
        :closable="true"
        @close="showCreateDialog = false"
        @keydown.esc="showCreateDialog = false"
        :bordered="false"
        class="model-dialog"
    >
      <n-space vertical :size="20">
        <n-alert type="info" :bordered="false">
          <template #icon>
            <n-icon>
              <InformationCircleOutline/>
            </n-icon>
          </template>
          创建新的统一模型。统一模型用于将不同渠道的相同模型映射到统一名称。
        </n-alert>

        <n-form
            ref="createFormRef"
            :model="createForm"
            :rules="createRules"
            label-placement="top"
            size="large"
        >
          <n-form-item label="模型名称" path="name">
            <n-input
                v-model:value="createForm.name"
                placeholder="请输入模型名称，如：gpt-4"
                @input="createForm.name = createForm.name.toLowerCase()"
            >
              <template #prefix>
                <n-icon>
                  <CodeOutline/>
                </n-icon>
              </template>
            </n-input>
            <template #feedback>
              统一的模型标识名称，自动转换为小写
            </template>
          </n-form-item>

          <n-form-item label="厂商" path="provider_id">
            <n-select
                v-model:value="createForm.provider_id"
                :options="providerOptions"
                placeholder="请选择厂商"
                :loading="loadingProviders"
                filterable
            >
              <template #prefix>
                <n-icon>
                  <BuildOutline/>
                </n-icon>
              </template>
            </n-select>
            <template #feedback>
              选择模型所属的厂商或提供商
            </template>
          </n-form-item>

          <n-form-item label="备注" path="remark">
            <n-input
                v-model:value="createForm.remark"
                type="textarea"
                placeholder="请输入备注（可选）"
                :autosize="{ minRows: 3, maxRows: 6 }"
            >
              <template #prefix>
                <n-icon>
                  <TextOutline/>
                </n-icon>
              </template>
            </n-input>
          </n-form-item>
        </n-form>
      </n-space>

      <template #footer>
        <n-space justify="end" :size="12">
          <n-button @click="showCreateDialog = false" size="large">
            取消
          </n-button>
          <n-button type="primary" @click="handleCreate" :loading="creating" size="large" strong>
            <template #icon>
              <n-icon>
                <AddOutline/>
              </n-icon>
            </template>
            确定
          </n-button>
        </n-space>
      </template>
    </n-modal>

    <!-- 编辑模型对话框 -->
    <n-modal
        v-model:show="showEditDialog"
        preset="card"
        title="编辑模型"
        :style="{ width: '600px' }"
        :mask-closable="false"
        :closable="true"
        @close="showEditDialog = false"
        @keydown.esc="showEditDialog = false"
        :bordered="false"
        class="model-dialog"
    >
      <n-space vertical :size="20">
        <n-alert type="info" :bordered="false" class="info-alert">
          <template #icon>
            <n-icon>
              <InformationCircleOutline/>
            </n-icon>
          </template>
          修改统一模型的配置信息。
        </n-alert>

        <n-form
            ref="editFormRef"
            :model="editForm"
            :rules="editRules"
            label-placement="top"
            size="large"
        >
          <n-form-item label="模型名称" path="name">
            <n-input
                v-model:value="editForm.name"
                placeholder="请输入模型名称"
                @input="editForm.name = editForm.name?.toLowerCase()"
            >
              <template #prefix>
                <n-icon>
                  <CodeOutline/>
                </n-icon>
              </template>
            </n-input>
            <template #feedback>
              统一的模型标识名称，自动转换为小写
            </template>
          </n-form-item>

          <n-form-item label="厂商" path="provider_id">
            <n-select
                v-model:value="editForm.provider_id"
                :options="providerOptions"
                placeholder="请选择厂商"
                :loading="loadingProviders"
                filterable
            >
              <template #prefix>
                <n-icon>
                  <BuildOutline/>
                </n-icon>
              </template>
            </n-select>
            <template #feedback>
              选择模型所属的厂商或提供商
            </template>
          </n-form-item>

          <n-form-item label="备注" path="remark">
            <n-input
                v-model:value="editForm.remark"
                type="textarea"
                placeholder="请输入备注（可选）"
                :autosize="{ minRows: 3, maxRows: 6 }"
            >
              <template #prefix>
                <n-icon>
                  <TextOutline/>
                </n-icon>
              </template>
            </n-input>
          </n-form-item>
        </n-form>
      </n-space>

      <template #footer>
        <n-space justify="end" :size="12">
          <n-button @click="showEditDialog = false" size="large">
            取消
          </n-button>
          <n-button type="primary" @click="handleUpdate" :loading="updating" size="large" strong>
            <template #icon>
              <n-icon>
                <SaveOutline/>
              </n-icon>
            </template>
            保存
          </n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
// @ts-nocheck
import {h, onMounted, reactive, ref} from 'vue'
import {
  type DataTableColumns,
  type FormInst,
  type FormRules,
  NAlert,
  NButton,
  NCard,
  NDataTable,
  NForm,
  NFormItem,
  NIcon,
  NInput,
  NModal,
  NSelect,
  NSpace,
  NText
} from 'naive-ui'
import {
  AddOutline,
  BuildOutline,
  CodeOutline,
  CreateOutline,
  InformationCircleOutline,
  SaveOutline,
  TextOutline,
  TrashOutline
} from '@vicons/ionicons5'
import {type CreateModelRequest, modelApi, type UpdateModelRequest} from '../services/modelService'
import type {Model} from '../types/model'
import providerApi from '@/services/providerService'
import type {Provider} from '@/types/model'

// 状态
const loading = ref(false)
const creating = ref(false)
const updating = ref(false)
const loadingProviders = ref(false)
const models = ref<Model[]>([])
const providers = ref<Provider[]>([])
const showCreateDialog = ref(false)
const showEditDialog = ref(false)
const currentEditModel = ref<Model | null>(null)

// 厂商选项
const providerOptions = ref<Array<{ label: string, value: string }>>([])

// 表单引用
const createFormRef = ref<FormInst | null>(null)
const editFormRef = ref<FormInst | null>(null)

// 创建表单
const createForm = reactive<CreateModelRequest>({
  name: '',
  provider_id: null,
  remark: ''
})

// 编辑表单
const editForm = reactive<UpdateModelRequest>({
  name: '',
  provider_id: undefined,
  remark: ''
})

// 创建表单验证规则
const createRules: FormRules = {
  name: {
    required: true,
    message: '请输入模型名称',
    trigger: ['blur', 'input']
  },
  provider_id: {
    required: true,
    type: 'string',
    message: '请选择厂商',
    trigger: ['blur', 'change']
  }
}

// 编辑表单验证规则
const editRules: FormRules = {
  name: {
    required: true,
    message: '请输入模型名称',
    trigger: ['blur', 'input']
  },
  provider_id: {
    type: 'string',
    message: '请选择厂商',
    trigger: ['blur', 'change']
  }
}

// 分页配置

// 表格列
const columns: DataTableColumns<Model> = [
  {
    title: 'ID',
    key: 'id',
    width: 80,
    align: 'left',
    sorter: (a, b) => a.id - b.id
  },
  {
    title: '模型名称',
    key: 'name',
    width: 240,
    render: (row) => {
      return h(NText, {code: true}, {default: () => row.name})
    },
    sorter: (a, b) => a.name.toLowerCase().localeCompare(b.name.toLowerCase())
  },
  {
    title: '厂商',
    key: 'provider',
    width: 240,
    sorter: (a, b) => {
      const aName = a.provider?.name || ''
      const bName = b.provider?.name || ''
      return aName.localeCompare(bName)
    },
    render: (row) => {
      if (row.provider) {
        return h(NText, {tag: 'strong'}, {default: () => row.provider!.name})
      }
      return h(NText, {depth: 3}, {default: () => '未设置'})
    }
  },
  {
    title: '备注',
    key: 'remark',
    width: 200,
    ellipsis: {
      tooltip: true
    }
  },
  {
    title: '创建时间',
    key: 'created_at',
    width: 200,
    align: 'center',
    render: (row) => {
      return new Date(row.created_at).toLocaleString('zh-CN')
    }
  },
  {
    title: '操作',
    key: 'actions',
    width: 200,
    fixed: 'right',
    align: 'center',
    render: (row) => {
      return h(
          NSpace,
          {size: 'small', justify: 'center'},
          {
            default: () => [
              h(
                  NButton,
                  {
                    size: 'small',
                    onClick: () => handleEdit(row)
                  },
                  {
                    default: () => '编辑',
                    icon: () => h(NIcon, null, {default: () => h(CreateOutline)})
                  }
              ),
              h(
                  NButton,
                  {
                    size: 'small',
                    type: 'error',
                    onClick: () => handleDelete(row)
                  },
                  {
                    default: () => '删除',
                    icon: () => h(NIcon, null, {default: () => h(TrashOutline)})
                  }
              )
            ]
          }
      )
    }
  }
]

// 加载模型列表
async function loadModels() {
  loading.value = true
  try {
    models.value = await modelApi.list()
  } catch (error: any) {
    console.error('Failed to load models:', error)
    window.$message?.error(error.response?.data?.error || '加载模型列表失败')
  } finally {
    loading.value = false
  }
}

// 创建模型
async function handleCreate() {
  if (!createFormRef.value) return

  try {
    await createFormRef.value.validate()
    creating.value = true

    await modelApi.create(createForm)

    window.$message?.success('创建成功')
    showCreateDialog.value = false

    // 重置表单
    createForm.name = ''
    createForm.provider_id = null
    createForm.remark = ''

    await loadModels()
  } catch (error: any) {
    console.error('Failed to create model:', error)
    if (error.errors) {
      // 表单验证错误
      return
    }
    window.$message?.error(error.response?.data?.error || '创建失败')
  } finally {
    creating.value = false
  }
}

// 加载厂商列表
async function loadProviders() {
  loadingProviders.value = true
  try {
    const providerList = await providerApi.list()
    providers.value = providerList
    providerOptions.value = providerList.map(p => ({
      label: p.name,
      value: p.id
    }))
  } catch (error: any) {
    console.error('Failed to load providers:', error)
    window.$message?.error(error.response?.data?.error || '加载厂商列表失败')
  } finally {
    loadingProviders.value = false
  }
}

// 编辑模型
function handleEdit(model: Model) {
  currentEditModel.value = model
  editForm.name = model.name
  editForm.provider_id = model.provider_id ?? undefined
  editForm.remark = model.remark
  showEditDialog.value = true
}

// 更新模型
async function handleUpdate() {
  if (!editFormRef.value || !currentEditModel.value) return

  try {
    await editFormRef.value.validate()
    updating.value = true

    await modelApi.update(currentEditModel.value.id, editForm)

    window.$message?.success('更新成功')
    showEditDialog.value = false
    currentEditModel.value = null

    await loadModels()
  } catch (error: any) {
    console.error('Failed to update model:', error)
    if (error.errors) {
      // 表单验证错误
      return
    }
    window.$message?.error(error.response?.data?.error || '更新失败')
  } finally {
    updating.value = false
  }
}

// 删除模型
async function handleDelete(model: Model) {
  await window.$dialog?.warning({
    title: '确认删除',
    content: `确定要删除模型 "${model.name}" 吗？`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await modelApi.delete(model.id)
        window.$message?.success('删除成功')
        await loadModels()
      } catch (error: any) {
        console.error('Failed to delete model:', error)
        window.$message?.error(error.response?.data?.error || '删除失败')
      }
    }
  })
}

// 初始化
onMounted(() => {
  loadProviders()
  loadModels()
})
</script>

<style scoped>
/* 页面头部卡片 */
.page-header-card {
  background: linear-gradient(135deg, #f5f7fa 0%, #ffffff 100%);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
  border-radius: 12px;
  padding: 24px;
}

.page-title {
  font-size: 24px;
  font-weight: 700;
  color: #333;
  line-height: 1.4;
}

.page-subtitle {
  font-size: 14px;
  line-height: 1.6;
}

/* 对话框样式 */
.model-dialog {
  --n-border-radius: 12px;
}

.model-dialog :deep(.n-card__content) {
  padding: 24px;
}

/* 提示信息样式 */
.info-alert {
  background: linear-gradient(135deg, #f0f9ff 0%, #e0f2fe 100%);
  border-left: 4px solid var(--info-color);
}


/* Alert 样式优化 */
.model-dialog :deep(.n-alert) {
  border-radius: 8px;
  padding: 16px 20px;
}

.model-dialog :deep(.n-alert .n-alert-body) {
  font-size: 13px;
  line-height: 1.6;
}
</style>
