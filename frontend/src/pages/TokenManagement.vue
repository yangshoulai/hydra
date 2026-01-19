<template>
  <div class="space-y-4">
    <!-- 页面头部 -->
    <n-card :bordered="false" class="page-header-card">
      <n-space justify="space-between" align="center">
        <n-space vertical :size="4">
          <n-text class="page-title">访问令牌管理</n-text>
          <n-text depth="3" class="page-subtitle">
            管理系统的访问令牌，用于 API 调用和身份验证
          </n-text>
        </n-space>
        <n-button type="primary" @click="showCreateDialog = true" size="large" strong>
          <template #icon>
            <n-icon>
              <AddOutline/>
            </n-icon>
          </template>
          新建令牌
        </n-button>
      </n-space>
    </n-card>

    <!-- 过滤表单 -->
    <n-form inline :label-width="50" :model="filters" :label-placement="'left'" :label-align="'left'" :show-feedback="false">
      <n-grid :cols="24" :x-gap="24" responsive="screen">
        <n-form-item-gi :span="6" label="名称">
          <n-input
              v-model:value="filters.name"
              placeholder="输入令牌名称"
              clearable
              @update:value="handleFilterChange"
          />
        </n-form-item-gi>
        <n-form-item-gi :span="6" label="状态">
          <n-select
              v-model:value="filters.status"
              placeholder="选择状态"
              clearable
              :options="statusOptions"
              @update:value="handleFilterChange"
          />
        </n-form-item-gi>
        <n-form-item-gi :span="6" label="令牌">
          <n-input
              v-model:value="filters.token"
              placeholder="输入令牌"
              clearable
              @update:value="handleFilterChange"
          />
        </n-form-item-gi>
        <n-form-item-gi :span="6">
          <n-space>
            <n-button type="primary" @click="handleSearch">
              <template #icon>
                <n-icon>
                  <SearchOutline/>
                </n-icon>
              </template>
              查询
            </n-button>
            <n-button @click="handleReset">
              <template #icon>
                <n-icon>
                  <RefreshOutline/>
                </n-icon>
              </template>
              重置
            </n-button>
          </n-space>
        </n-form-item-gi>
      </n-grid>
    </n-form>

    <n-data-table
        :columns="columns"
        :data="tokens"
        :pagination="false"
        :scroll-x="1480"
        :single-line="false"
        striped
        :loading="isLoading"
        :row-key="(row: Token) => row.id"
        @update:sorter="handleSorterChange"
    />

    <div class="flex justify-end">
      <n-pagination
          :page="pagination.page"
          :on-update-page="pagination.onChange"
          @update:page-size="pagination.onUpdatePageSize"
          :page-size="pagination.pageSize"
          :item-count="pagination.total"
          :page-sizes="pagination.pageSizes"
          :show-size-picker="pagination.showSizePicker"
      />
    </div>


    <!-- 创建令牌对话框 -->
    <n-modal
        v-model:show="showCreateDialog"
        preset="card"
        title="新建访问令牌"
        :style="{ width: '720px' }"
        :mask-closable="false"
        :closable="true"
        @close="showCreateDialog = false"
        @keydown.esc="showCreateDialog = false"
        :bordered="false"
        class="token-dialog"
    >
      <n-space vertical :size="20">
        <n-alert type="info" :bordered="false">
          <template #icon>
            <n-icon>
              <InformationCircleOutline/>
            </n-icon>
          </template>
          创建访问令牌用于 API 调用。令牌创建成功后将显示完整的访问令牌，请妥善保管。
        </n-alert>

        <n-form
            ref="formRef"
            :model="formData"
            :rules="rules"
            label-placement="top"
            size="large"
        >
          <n-space vertical :size="12">
            <n-form-item label="令牌名称" path="name">
              <n-input
                  v-model:value="formData.name"
                  placeholder="请输入令牌名称（用于识别此令牌的用途）"
                  maxlength="20"
                  show-count
              >
                <template #prefix>
                  <n-icon>
                    <TextOutline/>
                  </n-icon>
                </template>
              </n-input>
              <template #feedback>
                为令牌取一个易于识别的名称，便于管理
              </template>
            </n-form-item>

            <n-form-item label="过期时间" path="expires_at">
              <n-radio-group v-model:value="expireType" @update:value="handleExpireTypeChange">
                <n-space vertical :size="12">
                  <n-radio value="never" class="radio-item">
                    <n-space align="center">
                      <n-icon size="18">
                        <InfiniteOutline/>
                      </n-icon>
                      永不过期
                    </n-space>
                  </n-radio>
                  <n-radio value="custom" class="radio-item">
                    <n-space align="center">
                      <n-icon size="18">
                        <CalendarOutline/>
                      </n-icon>
                      自定义过期时间
                    </n-space>
                  </n-radio>
                </n-space>
              </n-radio-group>
            </n-form-item>

            <n-form-item v-if="expireType === 'custom'" label="选择过期时间" path="expires_at">
              <n-date-picker
                  v-model:value="expiresAtValue"
                  type="datetime"
                  placeholder="请选择过期时间"
                  :is-date-disabled="isDateDisabled"
                  :time-picker-props="{ format: 'HH:mm:ss' }"
                  style="width: 100%"
              >
                <template #date-icon>
                  <n-icon>
                    <CalendarOutline/>
                  </n-icon>
                </template>
              </n-date-picker>
              <template #feedback>
                选择令牌的过期时间，过期后令牌将无法使用
              </template>
            </n-form-item>
          </n-space>
        </n-form>
      </n-space>

      <template #footer>
        <n-space justify="end" :size="12">
          <n-button @click="showCreateDialog = false" size="large">
            取消
          </n-button>
          <n-button type="primary" :loading="isCreating" @click="handleCreate" size="large" strong>
            <template #icon>
              <n-icon>
                <AddOutline/>
              </n-icon>
            </template>
            创建令牌
          </n-button>
        </n-space>
      </template>
    </n-modal>

    <!-- 令牌创建成功对话框 -->
    <n-modal
        v-model:show="showSuccessDialog"
        preset="card"
        title="令牌创建成功"
        :style="{ width: '720px' }"
        :bordered="false"
        :mask-closable="false"
        class="success-dialog"
    >
      <n-space vertical :size="24">
        <!-- 成功提示 -->
        <n-alert type="success" :bordered="false">
          <template #icon>
            <n-icon>
              <CheckmarkCircleOutline/>
            </n-icon>
          </template>
          访问令牌已成功创建。请立即复制并妥善保管，此令牌只会显示一次！
        </n-alert>

        <!-- 令牌信息卡片 -->
        <n-card :bordered="true" class="token-info-card">
          <n-space vertical :size="16">
            <div class="token-info-item">
              <div class="token-info-label">
                <n-icon size="16">
                  <TextOutline/>
                </n-icon>
                令牌名称
              </div>
              <div class="token-info-value">{{ createdToken?.name }}</div>
            </div>

            <n-divider style="margin: 0"/>

            <div class="token-info-item">
              <div class="token-info-label">
                <n-icon size="16">
                  <KeyOutline/>
                </n-icon>
                访问令牌
              </div>
              <div class="token-display-container">
                <n-text code class="token-display">{{ createdToken?.access_token }}</n-text>
                <n-button
                    text
                    size="small"
                    @click="handleCopyToken"
                >
                  <template #icon>
                    <n-icon>
                      <CopyOutline/>
                    </n-icon>
                  </template>
                </n-button>
              </div>
            </div>
          </n-space>
        </n-card>

        <!-- 警告提示 -->
        <n-alert type="warning" :bordered="false">
          <template #icon>
            <n-icon>
              <WarningOutline/>
            </n-icon>
          </template>
          请立即复制并保存此令牌。关闭此对话框后，您将无法再次查看完整的令牌内容。
        </n-alert>
      </n-space>

      <template #footer>
        <n-space justify="end" :size="12">
          <n-button type="primary" @click="handleCopyAndClose" size="large" strong>
            <template #icon>
              <n-icon>
                <CopyOutline/>
              </n-icon>
            </template>
            复制并关闭
          </n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import {computed, h, onMounted, reactive, ref} from 'vue'
import {
  type DataTableColumns,
  NAlert,
  NButton,
  NCard,
  NDataTable,
  NDatePicker,
  NDivider,
  NForm,
  NFormItem,
  NFormItemGi,
  NGrid,
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
  useDialog,
  useMessage,
} from 'naive-ui'
import {
  AddOutline,
  CalendarOutline,
  CheckmarkCircleOutline,
  CopyOutline,
  InfiniteOutline,
  InformationCircleOutline,
  KeyOutline,
  RefreshOutline,
  SearchOutline,
  TextOutline,
  ToggleOutline,
  TrashOutline,
  WarningOutline
} from '@vicons/ionicons5'
import type {CreateTokenResponse, Token, TokenListParams} from '@/services/tokensService'
import tokensService from '@/services/tokensService'

const dialog = useDialog()
const message = useMessage()

const isLoading = ref(false)
const isCreating = ref(false)
const showCreateDialog = ref(false)
const showSuccessDialog = ref(false)

const tokens = ref<Token[]>([])
const createdToken = ref<CreateTokenResponse | null>(null)

// 过滤条件
const filters = reactive<TokenListParams>({
  name: '',
  status: null,
  token: ''
})

// 排序状态
const sortState = reactive({
  columnKey: 'created_at' as 'id' | 'status' | 'created_at' | 'last_used_at',
  order: 'desc' as 'asc' | 'desc'
})

// 状态选项
const statusOptions = [
  {label: '启用', value: 'active'},
  {label: '禁用', value: 'disabled'}
]

const formData = ref({
  name: '',
})

const expireType = ref<'never' | 'custom'>('never')
const expiresAtValue = ref<number | null>(null)

const rules = {
  name: [
    {required: true, message: '请输入令牌名称', trigger: 'blur'},
    {min: 2, max: 20, message: '名称长度应在 2-20 字符之间', trigger: 'blur'},
  ],
}

// 禁用过去的日期
const isDateDisabled = (timestamp: number) => {
  return timestamp < Date.now()
}

// 过期类型改变
const handleExpireTypeChange = (value: 'never' | 'custom') => {
  if (value === 'never') {
    expiresAtValue.value = null
  }
}

// 格式化日期时间为 YYYY-MM-DD HH:mm:ss
const formatDateTime = (timestamp: number) => {
  const date = new Date(timestamp)
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hours = String(date.getHours()).padStart(2, '0')
  const minutes = String(date.getMinutes()).padStart(2, '0')
  const seconds = String(date.getSeconds()).padStart(2, '0')
  return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`
}

// 表格列定义（使用 computed 响应式更新排序状态）
const columns = computed<DataTableColumns<Token>>(() => {
  const getSortOrder = (key: string) => {
    if (sortState.columnKey === key) {
      return sortState.order === 'asc' ? 'ascend' : 'descend'
    }
    return false
  }

  return [
    {
      title: 'ID',
      key: 'id',
      width: 80,
      align: 'left',
      sortable: true,
      sorter: 'default',
      sortOrder: getSortOrder('id')
    },
    {
      title: '名称',
      key: 'name',
      width: 200
    },
    {
      title: '令牌',
      key: 'token_preview',
      width: 240,
      render: (row) => {
        return h(NText, {code: true}, {default: () => row.token_preview})
      },
    },
    {
      title: '状态',
      key: 'status',
      align: 'center',
      width: 120,
      sortable: true,
      sorter: 'default',
      sortOrder: getSortOrder('status'),
      render: (row) => {
        // 检查是否过期
        const isExpired = row.expires_at ? new Date(row.expires_at) < new Date() : false
        const isDisabled = row.status !== 'active'

        let type: 'success' | 'warning' | 'default' = 'success'
        let text = '启用'

        if (isExpired) {
          type = 'warning'
          text = '已过期'
        } else if (isDisabled) {
          type = 'default'
          text = '禁用'
        }

        return h(
            NTag,
            {type, size: 'small'},
            {default: () => text}
        )
      },
    },
    {
      title: '过期时间',
      key: 'expires_at',
      align: 'center',
      width: 200,
      render: (row) => {
        if (!row.expires_at) {
          return h('span', {style: 'color: #9ca3af;'}, '永不过期')
        }
        const isExpired = new Date(row.expires_at) < new Date()
        const style = isExpired ? 'color: #f59e0b; font-weight: 500;' : 'color: #4b5563;'
        return h('span', {style}, row.expires_at)
      },
    },
    {
      title: '创建时间',
      key: 'created_at',
      width: 200,
      align: 'center',
      sortable: true,
      sorter: 'default',
      sortOrder: getSortOrder('created_at')
    },
    {
      title: '最后使用',
      key: 'last_used_at',
      width: 200,
      align: 'center',
      sortable: true,
      sorter: 'default',
      sortOrder: getSortOrder('last_used_at'),
      render: (row) => {
        return row.last_used_at || '-'
      },
    },
    {
      title: '操作',
      key: 'actions',
      width: 240,
      align: 'center',
      fixed: 'right',
      render: (row) => {
        return h('div', {style: 'display: flex; gap: 8px; justify-content: center;'}, [
          h(
              NButton,
              {
                size: 'tiny',
                type: 'primary',
                onClick: () => handleCopyTokenFromList(row),
              },
              {
                default: () => '复制',
                icon: () => h(NIcon, null, {default: () => h(CopyOutline)})
              }
          ),
          h(
              NButton,
              {
                size: 'tiny',
                type: 'warning',
                onClick: () => handleToggleStatus(row),
                style: 'margin-left: 8px',
              },
              {
                default: () => (row.status === 'active' ? '禁用' : '启用'),
                icon: () => h(NIcon, null, {default: () => h(ToggleOutline)})
              }
          ),
          h(
              NButton,
              {
                size: 'tiny',
                type: 'error',
                style: 'margin-left: 8px',
                onClick: () => handleDelete(row),
              },
              {
                default: () => '删除',
                icon: () => h(NIcon, null, {default: () => h(TrashOutline)})
              }
          ),
        ])
      },
    },
  ]
})

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
  onUpdatePageSize: (pageSize: number) => {
    pagination.pageSize = pageSize
    pagination.page = 1
    loadTokens()
  }
})

// 处理排序变化
function handleSorterChange(sorter: { columnKey: string; order: 'ascend' | 'descend' | false }) {
  if (sorter.columnKey) {
    sortState.columnKey = sorter.columnKey as 'id' | 'status' | 'created_at' | 'last_used_at'
    sortState.order = sorter.order === 'ascend' ? 'asc' : sorter.order === 'descend' ? 'desc' : 'asc'
  } else {
    sortState.columnKey = 'created_at'
    sortState.order = 'desc'
  }

  pagination.page = 1
  loadTokens()
}

// 过滤条件变化时重置到第一页
function handleFilterChange() {
  pagination.page = 1
}

// 搜索
function handleSearch() {
  pagination.page = 1
  loadTokens()
}

// 重置
function handleReset() {
  filters.name = ''
  filters.status = null
  filters.token = ''
  pagination.page = 1
  loadTokens()
}

// 加载令牌列表
const loadTokens = async () => {
  isLoading.value = true
  try {
    const params: TokenListParams = {
      page: pagination.page,
      page_size: pagination.pageSize,
      name: filters.name || undefined,
      status: filters.status || undefined,
      token: filters.token || undefined,
      sort_by: sortState.columnKey,
      sort_order: sortState.order
    }

    const result = await tokensService.list(params)
    tokens.value = result.items
    pagination.total = result.total
  } catch (error) {
    console.error('Failed to load tokens:', error)
    message.error('无法加载令牌列表')
  } finally {
    isLoading.value = false
  }
}

// 创建令牌
const handleCreate = async () => {
  // 验证
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
    // 准备过期时间
    let expiresAt = ''
    if (expireType.value === 'custom' && expiresAtValue.value) {
      expiresAt = formatDateTime(expiresAtValue.value)
    }

    createdToken.value = await tokensService.createToken({
      name: formData.value.name,
      expires_at: expiresAt,
    })

    showCreateDialog.value = false
    showSuccessDialog.value = true

    // 重置表单
    formData.value.name = ''
    expireType.value = 'never'
    expiresAtValue.value = null

    // 刷新列表
    await loadTokens()
  } catch (error: any) {
    console.error('Failed to create token:', error)
    // 提取后端返回的具体错误信息
    const errorMsg = error.response?.data?.message || error.response?.data?.error || '创建令牌失败'
    message.error(errorMsg)
  } finally {
    isCreating.value = false
  }
}

// 复制令牌
const handleCopyToken = async () => {
  if (!createdToken.value?.access_token) return

  try {
    await navigator.clipboard.writeText(createdToken.value.access_token)
    message.success('令牌已复制到剪贴板')
  } catch (error) {
    console.error('Failed to copy token:', error)
    message.error('复制失败，请手动复制')
  }
}

// 复制并关闭
const handleCopyAndClose = async () => {
  if (!createdToken.value?.access_token) return

  try {
    await navigator.clipboard.writeText(createdToken.value.access_token)
    message.success('令牌已复制到剪贴板')
    showSuccessDialog.value = false
    createdToken.value = null
  } catch (error) {
    console.error('Failed to copy token:', error)
    message.error('复制失败，请手动复制')
  }
}

// 从列表复制脱敏令牌
const handleCopyTokenFromList = async (token: Token) => {
  try {
    await navigator.clipboard.writeText(token.token)
    message.success('令牌已复制')
  } catch (error) {
    console.error('Failed to copy token:', error)
    message.error('复制失败，请手动复制')
  }
}

// 切换令牌状态
const handleToggleStatus = (token: Token) => {
  dialog.warning({
    title: '确认操作',
    content: `确定要${token.status === 'active' ? '禁用' : '启用'}此令牌吗？`,
    positiveText: '确定',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await tokensService.toggleTokenStatus(token.id)
        message.success('令牌状态已更新')
        await loadTokens()
      } catch (error) {
        console.error('Failed to toggle token status:', error)
        message.error('更新令牌状态失败')
      }
    },
  })
}

// 删除令牌
const handleDelete = (token: Token) => {
  dialog.warning({
    title: '删除确认',
    content: `确定要删除令牌"${token.name}"吗？此操作不可恢复！`,
    positiveText: '确定删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await tokensService.deleteToken(token.id)
        message.success('令牌已删除')
        await loadTokens()
      } catch (error) {
        console.error('Failed to delete token:', error)
        message.error('删除令牌失败')
      }
    },
  })
}

onMounted(() => {
  loadTokens()
})
</script>

<style scoped>
/* 页面样式 */

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


/* 表单项样式优化 */
.token-dialog :deep(.n-form-item-label) {
  font-weight: 600;
  color: #333;
  font-size: 14px;
  padding-bottom: 8px;
}

.token-dialog :deep(.n-form-item-feedback) {
  font-size: 12px;
  color: #999;
  margin-top: 4px;
}

/* Radio 样式优化 */
.radio-item {
  padding: 8px 12px;
  border-radius: 8px;
  transition: all 0.3s;
}

.radio-item:hover {
  background: #f5f5f6;
}

/* 输入框样式优化 */
.token-dialog :deep(.n-input) {
  border-radius: 8px;
  transition: all 0.3s;
}

.token-dialog :deep(.n-input:focus) {
  box-shadow: 0 0 0 2px rgba(24, 160, 88, 0.1);
}

/* 图标样式 */
.token-dialog :deep(.n-input__prefix) {
  color: #999;
  margin-right: 8px;
}

.token-dialog :deep(.n-input:focus .n-input__prefix) {
  color: var(--primary-color);
}

/* 令牌信息卡片 */
.token-info-card {
  background: linear-gradient(135deg, #fafafa 0%, #ffffff 100%);
  border-radius: 8px;
}

.token-info-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.token-info-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: #666;
  font-weight: 500;
}

.token-info-value {
  font-size: 16px;
  color: #333;
  font-weight: 600;
  word-break: break-all;
}

.token-display-container {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px;
  background: #f5f5f6;
  border-radius: 6px;
  border: 1px solid #e8e8e9;
}

.token-display {
  font-family: 'Courier New', monospace;
  font-size: 13px;
  word-break: break-all;
  flex: 1;
}


</style>
