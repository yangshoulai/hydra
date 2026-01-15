<template>
  <div class="space-y-4 animate-fade-in">
    <!-- 操作栏 -->
    <div class="flex justify-between items-center">
      <n-space>
        <n-button type="primary" @click="showCreateDialog = true">
          <template #icon>
            <n-icon>
              <AddOutline />
            </n-icon>
          </template>
          添加厂商
        </n-button>
        <n-button type="info" @click="handleSync" :loading="syncing">
          <template #icon>
            <n-icon>
              <SyncOutline />
            </n-icon>
          </template>
          同步厂商
        </n-button>
      </n-space>
    </div>

    <!-- 厂商列表 -->
    <n-data-table
      :columns="columns"
      :data="providers"
      :pagination="pagination"
      :bordered="true"
      :loading="loading"
    />

    <!-- 创建厂商对话框 -->
    <n-modal v-model:show="showCreateDialog" preset="dialog" title="添加厂商">
      <n-form ref="createFormRef" :model="createForm" :rules="createRules" label-placement="left" label-width="80px">
        <n-form-item label="厂商ID" path="id">
          <n-input
            v-model:value="createForm.id"
            placeholder="请输入厂商ID，如：openai"
            @input="createForm.id = createForm.id.toLowerCase()"
          />
        </n-form-item>
        <n-form-item label="厂商名称" path="name">
          <n-input
            v-model:value="createForm.name"
            placeholder="请输入厂商名称，如：OpenAI"
          />
        </n-form-item>
        <n-form-item label="图标 URL" path="icon">
          <n-input v-model:value="createForm.icon" placeholder="请输入图标 URL（可选）" />
        </n-form-item>
        <n-form-item label="备注" path="remark">
          <n-input v-model:value="createForm.remark" type="textarea" placeholder="请输入备注" />
        </n-form-item>
      </n-form>
      <template #action>
        <n-space>
          <n-button @click="showCreateDialog = false">取消</n-button>
          <n-button type="primary" @click="handleCreate" :loading="creating"> 确定 </n-button>
        </n-space>
      </template>
    </n-modal>

    <!-- 编辑厂商对话框 -->
    <n-modal v-model:show="showEditDialog" preset="dialog" title="编辑厂商">
      <n-form ref="editFormRef" :model="editForm" :rules="editRules" label-placement="left" label-width="80px">
        <n-form-item label="厂商ID">
          <n-input :value="currentEditProvider?.id" disabled />
        </n-form-item>
        <n-form-item label="厂商名称" path="name">
          <n-input
            v-model:value="editForm.name"
            placeholder="请输入厂商名称"
          />
        </n-form-item>
        <n-form-item label="图标 URL" path="icon">
          <n-input v-model:value="editForm.icon" placeholder="请输入图标 URL（可选）" />
        </n-form-item>
        <n-form-item label="备注" path="remark">
          <n-input v-model:value="editForm.remark" type="textarea" placeholder="请输入备注" />
        </n-form-item>
      </n-form>
      <template #action>
        <n-space>
          <n-button @click="showEditDialog = false">取消</n-button>
          <n-button type="primary" @click="handleUpdate" :loading="updating"> 确定 </n-button>
        </n-space>
      </template>
    </n-modal>

    <!-- 同步厂商对话框 -->
    <n-modal v-model:show="showSyncDialog" preset="card" title="同步远程厂商" style="width: 800px">
      <n-alert type="info" style="margin-bottom: 16px">
        从远程服务器获取厂商列表，选择需要添加的厂商后点击"添加选中的厂商"。
      </n-alert>

      <n-space vertical :size="16">
        <!-- 统计信息 -->
        <n-statistic label="可添加厂商数">
          <template #default>
            <n-text type="info" strong style="font-size: 20px">
              {{ availableProviders.length }}
            </n-text>
          </template>
        </n-statistic>

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
        <n-space justify="end">
          <n-button @click="showSyncDialog = false">取消</n-button>
          <n-button
            type="primary"
            @click="handleAddSyncProviders"
            :loading="adding"
            :disabled="checkedProviderIds.length === 0"
          >
            添加选中的厂商 ({{ checkedProviderIds.length }})
          </n-button>
        </n-space>
      </n-space>
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
  NStatistic,
  NTag,
  NText,
} from 'naive-ui'
import { AddOutline, CreateOutline, SyncOutline, TrashOutline } from '@vicons/ionicons5'
import providerApi from '@/services/providerService'
import type { Provider, CreateProviderRequest, UpdateProviderRequest, RemoteProvider } from '@/types/model'

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
  remark: ''
})

// 编辑表单
const editForm = reactive<UpdateProviderRequest>({
  name: '',
  icon: '',
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

// 分页配置
const pagination = reactive({
  page: 1,
  pageSize: 20,
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
    title: 'ID',
    key: 'id',
    width: 120,
    sorter: (a, b) => a.id.localeCompare(b.id)
  },
  {
    title: '厂商名称',
    key: 'name',
    width: 280,
    render: (row) => {
      return h(NText, { tag: 'strong' }, { default: () => row.name })
    },
    sorter: (a, b) => a.name.toLowerCase().localeCompare(b.name.toLowerCase())
  },
  {
    title: '图标',
    key: 'icon',
    width: 60,
    render: (row) => {
      if (!row.icon) {
        return h(NText, { depth: 3 }, { default: () => '无' })
      }
      return h('img', {
        src: row.icon,
        alt: row.name,
        style: {
          width: '24px',
          height: '24px',
          objectFit: 'contain',
          verticalAlign: 'middle'
        }
      })
    }
  },
  {
    title: '备注',
    key: 'remark',
    ellipsis: {
      tooltip: true
    }
  },
  {
    title: '创建时间',
    key: 'created_at',
    width: 240,
    render: (row) => {
      return new Date(row.created_at).toLocaleString('zh-CN')
    }
  },
  {
    title: '操作',
    key: 'actions',
    width: 320,
    fixed: 'right',
    render: (row) => {
      return h(
        NSpace,
        { size: 'small' },
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
                icon: () => h(NIcon, null, { default: () => h(CreateOutline) })
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
                icon: () => h(NIcon, null, { default: () => h(TrashOutline) })
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
  },
  {
    title: '图标',
    key: 'iconURL',
    width: 80,
    render: (row) => {
      if (!row.iconURL) {
        return h(NText, {depth: 3}, {default: () => '无'})
      }
      return h('img', {
        src: row.iconURL,
        alt: row.name,
        style: {
          width: '32px',
          height: '32px',
          objectFit: 'contain',
          verticalAlign: 'middle'
        }
      })
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

    // 等待 Vue 更新后，设置默认选中的厂商
    await nextTick()

    // 计算本地已存在的厂商 ID
    const localIds = new Set(providers.value.map(p => p.id))

    // 只选中本地不存在的厂商
    const toAdd = remoteData.filter((rp: RemoteProvider) => !localIds.has(rp.id))
    checkedProviderIds.value = toAdd.map(rp => rp.id)

    console.log('本地厂商:', providers.value.map(p => p.id))
    console.log('远程厂商:', remoteData.map(rp => rp.id))
    console.log('可添加厂商:', toAdd.map(rp => rp.id))
    console.log('选中厂商:', checkedProviderIds.value)

    // 显示同步对话框
    showSyncDialog.value = true

    window.$message?.success(`成功获取 ${remoteData.length} 个远程厂商，其中 ${toAdd.length} 个可添加`)
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
:deep(.n-data-table__n-pagination) {
  margin-top: 16px;
}
</style>
