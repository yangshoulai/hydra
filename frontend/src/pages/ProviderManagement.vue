<template>
  <div class="space-y-4 animate-fade-in provider-management-page">
    <!-- 页面头部 -->
    <n-card :bordered="false" class="page-header-card">
      <n-space justify="space-between" align="center">
        <n-space vertical :size="4">
          <n-text class="page-title">厂商管理</n-text>
          <n-text depth="3" class="page-subtitle">
            管理系统中的所有 AI 服务厂商，支持从远程服务器同步厂商列表
          </n-text>
        </n-space>
        <n-space>
          <n-button type="info" @click="handleSync" :loading="syncing" size="large" secondary strong>
            <template #icon>
              <n-icon>
                <SyncOutline/>
              </n-icon>
            </template>
            同步厂商
          </n-button>
          <n-button type="primary" @click="showCreateDialog = true" size="large" strong>
            <template #icon>
              <n-icon>
                <AddOutline/>
              </n-icon>
            </template>
            添加厂商
          </n-button>
        </n-space>
      </n-space>
    </n-card>

    <!-- 厂商列表 -->
    <n-data-table
        :columns="columns"
        :data="providers"
        :pagination="false"
        :bordered="true"
        striped
        :single-line="false"
        :scroll-x="960"
        :loading="loading"
    />

    <!-- 创建厂商对话框 -->
    <n-modal
        v-model:show="showCreateDialog"
        preset="card"
        title="添加厂商"
        :style="{ width: '600px' }"
        :mask-closable="false"
        :closable="true"
        @close="showCreateDialog = false"
        @keydown.esc="showCreateDialog = false"
        :bordered="false"
        class="provider-dialog"
    >
      <n-space vertical :size="20">
        <n-alert type="info" :bordered="false" class="info-alert">
          <template #icon>
            <n-icon>
              <InformationCircleOutline/>
            </n-icon>
          </template>
          创建新的 AI 服务厂商。厂商信息将用于组织和展示不同的模型提供商。
        </n-alert>

        <n-form
            ref="createFormRef"
            :model="createForm"
            :rules="createRules"
            label-placement="top"
            size="large"
        >
          <n-form-item label="厂商ID" path="id">
            <n-input
                v-model:value="createForm.id"
                placeholder="请输入厂商ID，如：openai"
                @input="createForm.id = createForm.id.toLowerCase()"
            >
              <template #prefix>
                <n-icon>
                  <KeyOutline/>
                </n-icon>
              </template>
            </n-input>
            <template #feedback>
              厂商的唯一标识符，自动转换为小写
            </template>
          </n-form-item>

          <n-form-item label="厂商名称" path="name">
            <n-input
                v-model:value="createForm.name"
                placeholder="请输入厂商名称，如：OpenAI"
            >
              <template #prefix>
                <n-icon>
                  <BuildOutline/>
                </n-icon>
              </template>
            </n-input>
            <template #feedback>
              厂商的显示名称
            </template>
          </n-form-item>

          <n-form-item label="图标 URL" path="icon">
            <n-input
                v-model:value="createForm.icon"
                placeholder="请输入图标 URL（可选）"
            >
              <template #prefix>
                <n-icon>
                  <ImageOutline/>
                </n-icon>
              </template>
            </n-input>
            <template #feedback>
              厂商的图标 URL 地址
            </template>
          </n-form-item>

          <n-form-item label="Lobe 图标" path="lobeIcon">
            <n-input
                v-model:value="createForm.lobeIcon"
                placeholder="请输入 Lobe 图标组件名（可选），如：Claude.Color"
            >
              <template #prefix>
                <n-icon>
                  <BuildOutline/>
                </n-icon>
              </template>
            </n-input>
            <template #feedback>
              使用 LobeChat 图标组件的名称
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

    <!-- 编辑厂商对话框 -->
    <n-modal
        v-model:show="showEditDialog"
        preset="card"
        title="编辑厂商"
        :style="{ width: '600px' }"
        :mask-closable="false"
        :closable="true"
        @close="showEditDialog = false"
        @keydown.esc="showEditDialog = false"
        :bordered="false"
        class="provider-dialog"
    >
      <n-space vertical :size="20">
        <n-alert type="info" :bordered="false" class="info-alert">
          <template #icon>
            <n-icon>
              <InformationCircleOutline/>
            </n-icon>
          </template>
          修改厂商的配置信息。
        </n-alert>

        <n-form
            ref="editFormRef"
            :model="editForm"
            :rules="editRules"
            label-placement="top"
            size="large"
        >
          <n-form-item label="厂商ID">
            <n-input
                :value="currentEditProvider?.id"
                disabled
            >
              <template #prefix>
                <n-icon>
                  <KeyOutline/>
                </n-icon>
              </template>
            </n-input>
            <template #feedback>
              厂商ID不可修改
            </template>
          </n-form-item>

          <n-form-item label="厂商名称" path="name">
            <n-input
                v-model:value="editForm.name"
                placeholder="请输入厂商名称"
            >
              <template #prefix>
                <n-icon>
                  <BuildOutline/>
                </n-icon>
              </template>
            </n-input>
            <template #feedback>
              厂商的显示名称
            </template>
          </n-form-item>

          <n-form-item label="图标 URL" path="icon">
            <n-input
                v-model:value="editForm.icon"
                placeholder="请输入图标 URL（可选）"
            >
              <template #prefix>
                <n-icon>
                  <ImageOutline/>
                </n-icon>
              </template>
            </n-input>
            <template #feedback>
              厂商的图标 URL 地址
            </template>
          </n-form-item>

          <n-form-item label="Lobe 图标" path="lobeIcon">
            <n-input
                v-model:value="editForm.lobeIcon"
                placeholder="请输入 Lobe 图标组件名（可选），如：Claude.Color"
            >
              <template #prefix>
                <n-icon>
                  <BuildOutline/>
                </n-icon>
              </template>
            </n-input>
            <template #feedback>
              使用 LobeChat 图标组件的名称
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

    <!-- 同步厂商对话框 -->
    <n-modal
        v-model:show="showSyncDialog"
        preset="card"
        title="同步远程厂商"
        :style="{ width: '900px' }"
        :mask-closable="false"
        :closable="true"
        @close="showSyncDialog = false"
        @keydown.esc="showSyncDialog = false"
        :bordered="false"
        class="sync-dialog"
    >
      <n-space vertical :size="20">
        <!-- 说明信息 -->
        <n-alert type="info" :bordered="false" class="info-alert">
          <template #icon>
            <n-icon>
              <InformationCircleOutline/>
            </n-icon>
          </template>
          从远程服务器获取厂商列表，选择需要添加的厂商后点击"添加选中的厂商"。
        </n-alert>

        <!-- 统计信息 -->
        <n-card size="small" :bordered="false" class="stats-card">
          <n-grid :cols="3" :x-gap="20" responsive="screen">
            <n-grid-item>
              <div class="stat-item">
                <div class="stat-icon stat-icon-info">
                  <n-icon size="20">
                    <CloudOutline/>
                  </n-icon>
                </div>
                <div class="stat-content">
                  <div class="stat-label">远程厂商总数</div>
                  <div class="stat-value">{{ remoteProviders.length }}</div>
                </div>
              </div>
            </n-grid-item>
            <n-grid-item>
              <div class="stat-item">
                <div class="stat-icon stat-icon-success">
                  <n-icon size="20">
                    <CheckmarkCircleOutline/>
                  </n-icon>
                </div>
                <div class="stat-content">
                  <div class="stat-label">可添加</div>
                  <div class="stat-value stat-value-success">{{ availableProviders.length }}</div>
                </div>
              </div>
            </n-grid-item>
            <n-grid-item>
              <div class="stat-item">
                <div class="stat-icon stat-icon-primary">
                  <n-icon size="20">
                    <CheckboxOutline/>
                  </n-icon>
                </div>
                <div class="stat-content">
                  <div class="stat-label">已选中</div>
                  <div class="stat-value stat-value-primary">{{ checkedProviderIds.length }}</div>
                </div>
              </div>
            </n-grid-item>
          </n-grid>
        </n-card>

        <!-- 厂商列表 -->
        <n-data-table
            :columns="syncColumns"
            :data="availableProviders"
            :pagination="syncPagination"
            :bordered="true"
            :loading="syncing"
            :row-key="(row: RemoteProvider) => row.id"
            v-model:checked-row-keys="checkedProviderIds"
            size="small"
        />

        <!-- 操作按钮 -->
        <n-space justify="end" :size="12">
          <n-button @click="showSyncDialog = false" size="large">
            取消
          </n-button>
          <n-button
              type="primary"
              @click="handleAddSyncProviders"
              :loading="adding"
              :disabled="checkedProviderIds.length === 0"
              size="large"
              strong
          >
            <template #icon>
              <n-icon>
                <AddOutline/>
              </n-icon>
            </template>
            添加选中的厂商 ({{ checkedProviderIds.length }})
          </n-button>
        </n-space>
      </n-space>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import {computed, h, nextTick, onMounted, reactive, ref, watch} from 'vue'
import type {DataTableColumns, FormInst, FormRules} from 'naive-ui'
import {
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
  NModal,
  NSpace,
  NText
} from 'naive-ui'
import {
  AddOutline,
  BuildOutline,
  CheckboxOutline,
  CheckmarkCircleOutline,
  CloudOutline,
  CreateOutline,
  ImageOutline,
  InformationCircleOutline,
  KeyOutline,
  SaveOutline,
  SyncOutline,
  TextOutline,
  TrashOutline
} from '@vicons/ionicons5'
import providerApi from '@/services/providerService'
import type {CreateProviderRequest, Provider, RemoteProvider, UpdateProviderRequest} from '@/types/model'
import ProviderIcon from '@/components/ProviderIcon.vue'

// 状态
const loading = ref(false)
const creating = ref(false)
const updating = ref(false)
const syncing = ref(false)
const adding = ref(false)
const providers = ref<Provider[]>([])
const showCreateDialog = ref(false)
const showEditDialog = ref(false)
const showSyncDialog = ref(false)
const currentEditProvider = ref<Provider | null>(null)
const remoteProviders = ref<RemoteProvider[]>([])
const checkedProviderIds = ref<string[]>([])

// 表单引用
const createFormRef = ref<FormInst | null>(null)
const editFormRef = ref<FormInst | null>(null)

// 创建表单
const createForm = reactive<CreateProviderRequest>({
  id: '',
  name: '',
  icon: '',
  lobeIcon: '',
  remark: ''
})

// 编辑表单
const editForm = reactive<UpdateProviderRequest>({
  name: '',
  icon: '',
  lobeIcon: '',
  remark: ''
})

// 创建表单验证规则
const createRules: FormRules = {
  id: {
    required: true,
    message: '请输入厂商ID',
    trigger: ['blur', 'input']
  },
  name: {
    required: true,
    message: '请输入厂商名称',
    trigger: ['blur', 'input']
  }
}

// 编辑表单验证规则
const editRules: FormRules = {
  name: {
    required: true,
    message: '请输入厂商名称',
    trigger: ['blur', 'input']
  }
}


// 同步对话框分页配置
const syncPagination = reactive({
  page: 1,
  pageSize: 10,
  showSizePicker: true,
  pageSizes: [10, 20, 50],
  onChange: (page: number) => {
    syncPagination.page = page
  },
  onUpdatePageSize: (pageSize: number) => {
    syncPagination.pageSize = pageSize
    syncPagination.page = 1
  }
})

// 可添加的厂商（远程存在但本地不存在的）
const availableProviders = computed(() => {
  const localIds = new Set(providers.value.map(p => p.id))
  return remoteProviders.value.filter(rp => !localIds.has(rp.id))
})

// 表格列
const columns: DataTableColumns<Provider> = [
  {
    title: '图标',
    key: 'icon',
    width: 80,
    align: 'center',
    render: (row) => {
      return h(ProviderIcon, {
        lobeIcon: row.lobeIcon,
        iconURL: row.icon,
        alt: row.name,
        size: 24
      })
    }
  },
  {
    title: 'ID',
    key: 'id',
    width: 160,
    align: 'left',
    sorter: (a, b) => a.id.localeCompare(b.id)
  },
  {
    title: '厂商名称',
    key: 'name',
    width: 160,
    render: (row) => {
      return h(NText, {tag: 'strong'}, {default: () => row.name})
    },
    sorter: (a, b) => a.name.toLowerCase().localeCompare(b.name.toLowerCase())
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
    align: 'center',
    width: 200,
    render: (row) => {
      return new Date(row.created_at).toLocaleString('zh-CN')
    }
  },
  {
    title: '操作',
    key: 'actions',
    width: 160,
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

// 同步对话框表格列
const syncColumns: DataTableColumns<RemoteProvider> = [
  {
    type: 'selection'
  },
  {
    title: '图标',
    key: 'iconURL',
    width: 80,
    align: 'center',
    render: (row) => {
      return h(ProviderIcon, {
        lobeIcon: row.lobeIcon,
        iconURL: row.iconURL,
        alt: row.name,
        size: 20
      })
    }
  },
  {
    title: 'ID',
    key: 'id',
    width: 200
  },
  {
    title: '厂商名称',
    key: 'name',
    width: 280,
    render: (row) => {
      return h(NText, {tag: 'strong'}, {default: () => row.name})
    }
  }
]

// 加载厂商列表
async function loadProviders() {
  loading.value = true
  try {
    providers.value = await providerApi.list()
  } catch (error: any) {
    console.error('Failed to load providers:', error)
    window.$message?.error(error.response?.data?.error || '加载厂商列表失败')
  } finally {
    loading.value = false
  }
}

// 创建厂商
async function handleCreate() {
  if (!createFormRef.value) return

  try {
    await createFormRef.value.validate()
    creating.value = true

    await providerApi.create(createForm)

    window.$message?.success('创建成功')
    showCreateDialog.value = false

    // 重置表单
    createForm.id = ''
    createForm.name = ''
    createForm.icon = ''
    createForm.lobeIcon = ''
    createForm.remark = ''

    await loadProviders()
  } catch (error: any) {
    console.error('Failed to create provider:', error)
    if (error.errors) {
      // 表单验证错误
      return
    }
    window.$message?.error(error.response?.data?.error || '创建失败')
  } finally {
    creating.value = false
  }
}

// 编辑厂商
function handleEdit(provider: Provider) {
  currentEditProvider.value = provider
  editForm.name = provider.name
  editForm.icon = provider.icon
  editForm.lobeIcon = provider.lobeIcon || ''
  editForm.remark = provider.remark
  showEditDialog.value = true
}

// 更新厂商
async function handleUpdate() {
  if (!editFormRef.value || !currentEditProvider.value) return

  try {
    await editFormRef.value.validate()
    updating.value = true

    await providerApi.update(currentEditProvider.value.id, editForm)

    window.$message?.success('更新成功')
    showEditDialog.value = false
    currentEditProvider.value = null

    await loadProviders()
  } catch (error: any) {
    console.error('Failed to update provider:', error)
    if (error.errors) {
      // 表单验证错误
      return
    }
    window.$message?.error(error.response?.data?.error || '更新失败')
  } finally {
    updating.value = false
  }
}

// 删除厂商
async function handleDelete(provider: Provider) {
  await window.$dialog?.warning({
    title: '确认删除',
    content: `确定要删除厂商 "${provider.name}" 吗？`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await providerApi.delete(provider.id)
        window.$message?.success('删除成功')
        await loadProviders()
      } catch (error: any) {
        console.error('Failed to delete provider:', error)
        window.$message?.error(error.response?.data?.error || '删除失败')
      }
    }
  })
}

// 同步远程厂商
async function handleSync() {
  syncing.value = true
  try {
    // 获取远程厂商数据
    const remoteData = await providerApi.syncRemoteProviders()

    // 重置分页为第一页
    syncPagination.page = 1

    // 重置选中状态
    checkedProviderIds.value = []

    // 设置远程厂商数据
    remoteProviders.value = remoteData

    // 等待 Vue 更新
    await nextTick()

    // 显示同步对话框
    showSyncDialog.value = true

    window.$message?.success(`成功获取 ${remoteData.length} 个远程厂商`)
  } catch (error: any) {
    console.error('Failed to sync providers:', error)
    window.$message?.error(error.response?.data?.error || '同步远程厂商失败')
  } finally {
    syncing.value = false
  }
}

// 添加选中的同步厂商
async function handleAddSyncProviders() {
  if (checkedProviderIds.value.length === 0) {
    window.$message?.warning('请至少选择一个厂商')
    return
  }

  adding.value = true
  try {
    // 根据选中的 ID 找到对应的远程厂商数据
    const selectedProviders = remoteProviders.value.filter((rp: RemoteProvider) =>
        checkedProviderIds.value.includes(rp.id)
    )

    // 转换为创建请求格式
    const createRequests: CreateProviderRequest[] = selectedProviders.map((sp: RemoteProvider) => ({
      id: sp.id,
      name: sp.name,
      icon: sp.iconURL,
      lobeIcon: sp.lobeIcon,
      remark: sp.name
    }))

    // 批量创建厂商
    const result = await providerApi.batchCreate(createRequests)

    if (result.created > 0) {
      window.$message?.success(`成功添加 ${result.created} 个厂商${result.failed > 0 ? `，${result.failed} 个失败` : ''}`)
      // 关闭对话框并刷新列表
      showSyncDialog.value = false
      checkedProviderIds.value = []
      remoteProviders.value = []

      await loadProviders()
    } else {
      window.$message?.error('批量创建失败')
    }
  } catch (error: any) {
    console.error('Failed to add sync providers:', error)
    window.$message?.error(error.response?.data?.error || '添加厂商失败')
  } finally {
    adding.value = false
  }
}

// 初始化
onMounted(() => {
  loadProviders()
})

// 监听同步对话框关闭，重置状态
watch(showSyncDialog, (newVal) => {
  if (!newVal) {
    // 对话框关闭时重置状态
    checkedProviderIds.value = []
    remoteProviders.value = []
    syncPagination.page = 1
  }
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

/* 响应式调整 */
@media (max-width: 768px) {
  .page-header-card {
    padding: 16px;
  }

  .page-title {
    font-size: 20px;
  }

  .provider-dialog,
  .sync-dialog {
    width: 95vw !important;
  }

  .stats-card :deep(.n-grid-item) {
    min-width: 100%;
  }

  .provider-management-page :deep(.n-button) {
    font-size: 14px;
  }
}
</style>
