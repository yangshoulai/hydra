<template>

  <div class="space-y-4 animate-fade-in">
    <!-- 操作栏 -->
    <div class="flex">
      <n-button type="primary" @click="showCreateDialog = true">
        <template #icon>
          <n-icon>
            <AddOutline/>
          </n-icon>
        </template>
        新建令牌
      </n-button>
    </div>

    <n-data-table
        :columns="columns"
        :data="tokens"
        bordered
        size="small"
        :loading="isLoading"
        :row-key="(row: Channel) => row.id"
    />


    <!-- 创建令牌对话框 -->
    <n-modal
        v-model:show="showCreateDialog"
        preset="card"
        title="新建访问令牌"
        :style="{ width: '800px' }"
        :mask-closable="false"
        :closable="true"
        @close="showCreateDialog = false"
        @keydown.esc="showCreateDialog = false"
    >
      <n-form ref="formRef" :model="formData" :rules="rules" label-placement="left" label-width="120">
        <n-form-item label="令牌名称" path="name">
          <n-input
              v-model:value="formData.name"
              placeholder="请输入令牌名称（用于识别此令牌的用途）"
              maxlength="20"
              show-count
          />
        </n-form-item>
        <n-text depth="3" style="font-size: 12px; margin-top: -16px; margin-bottom: 16px; display: block; padding-left: 120px;">
          为令牌取一个易于识别的名称，便于管理
        </n-text>

        <n-form-item label="过期时间" path="expires_at">
          <n-radio-group v-model:value="expireType" @update:value="handleExpireTypeChange">
            <n-space vertical :size="12">
              <n-radio value="never">
                永不过期
              </n-radio>
              <n-radio value="custom">
                自定义过期时间
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
          />
        </n-form-item>
        <n-text v-if="expireType === 'custom'" depth="3" style="font-size: 12px; margin-top: -16px; margin-bottom: 16px; display: block; padding-left: 120px;">
          选择令牌的过期时间，过期后令牌将无法使用
        </n-text>
      </n-form>

      <template #footer>
        <n-space justify="end">
          <n-button @click="showCreateDialog = false">取消</n-button>
          <n-button type="primary" :loading="isCreating" @click="handleCreate">
            创建令牌
          </n-button>
        </n-space>
      </template>
    </n-modal>

    <!-- 令牌创建成功对话框 -->
    <n-modal
        v-model:show="showSuccessDialog"
        preset="dialog"
        title="令牌创建成功"
    >
      <n-space vertical :size="20" style="margin-top: 16px">
        <div>
          <div class="token-label">名称</div>
          <div class="token-value">{{ createdToken?.name }}</div>
        </div>

        <div>
          <div class="token-label">访问令牌</div>
          <div class="token-display-container">
            <n-text code class="token-display">{{ createdToken?.access_token }}</n-text>
            <n-button
                text
                size="small"
                @click="handleCopyToken"
                class="copy-icon-button"
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

      <template #action>
        <n-button type="primary" @click="handleCopyAndClose">
          <template #icon>
            <n-icon>
              <CopyOutline/>
            </n-icon>
          </template>
          复制并关闭
        </n-button>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import {h, onMounted, ref} from 'vue'
import {
  type DataTableColumns,
  NButton,
  NDataTable,
  NDatePicker,
  NForm,
  NFormItem,
  NIcon,
  NInput,
  NModal,
  NRadio,
  NRadioGroup,
  NSpace,
  NTag,
  NText,
  useDialog,
  useMessage,
} from 'naive-ui'
import {AddOutline, CopyOutline} from '@vicons/ionicons5'
import type {CreateTokenResponse, Token} from '@/services/tokensService'
import tokensService from '@/services/tokensService'
import type {Channel} from "@/types/channel.ts";

const dialog = useDialog()
const message = useMessage()

const isLoading = ref(false)
const isCreating = ref(false)
const showCreateDialog = ref(false)
const showSuccessDialog = ref(false)

const tokens = ref<Token[]>([])
const createdToken = ref<CreateTokenResponse | null>(null)

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

// 表格列定义
const columns: DataTableColumns<Token> = [
  {
    title: 'ID',
    key: 'id',
    width: 100,
  },
  {
    title: '名称',
    key: 'name',
    width: 200,
  },
  {
    title: '令牌',
    key: 'token_preview',
    minWidth: 200,
    render: (row) => {
      return h(NText, {code: true}, {default: () => row.token_preview})
    },
  },
  {
    title: '状态',
    key: 'status',
    align: 'center',
    width: 120,
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
    width: 180,
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
    width: 180,
  },
  {
    title: '最后使用',
    key: 'last_used_at',
    width: 180,
    render: (row) => {
      return row.last_used_at || '-'
    },
  },
  {
    title: '操作',
    key: 'actions',
    width: 300,
    fixed: 'right',
    render: (row) => {
      return h('div', {class: 'action-buttons'}, [
        h(
            NButton,
            {
              size: 'small',
              type: 'info',
              onClick: () => handleCopyTokenFromList(row),
            },
            {default: () => '复制'}
        ),
        h(
            NButton,
            {
              size: 'small',
              type: 'warning',
              onClick: () => handleToggleStatus(row),
              style: 'margin-left: 8px',
            },
            {default: () => (row.status === 'active' ? '禁用' : '启用')}
        ),
        h(
            NButton,
            {
              size: 'small',
              type: 'error',
              style: 'margin-left: 8px',
              onClick: () => handleDelete(row),
            },
            {default: () => '删除'}
        ),
      ])
    },
  },
]


// 加载令牌列表
const loadTokens = async () => {
  isLoading.value = true
  try {
    tokens.value = await tokensService.getAllTokens()
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
    await navigator.clipboard.writeText(token.token_preview)
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
/* ===================
   令牌管理容器
   =================== */
.token-management-container {
  animation: fadeIn 0.4s ease-out;
}

/* ===================
   操作按钮区域
   =================== */
.action-bar {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 24px;
}

.action-bar :deep(.n-button) {
  min-width: 120px;
  height: 40px;
  font-size: 15px;
  font-weight: 600;
  padding: 0 24px;
  color: white;
}

/* ===================
   卡片样式
   =================== */
.token-card {
  background: #ffffff;
  border-radius: var(--radius-xl);
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  border: 1px solid #e5e7eb;
  overflow: hidden;
  transition: all 200ms cubic-bezier(0.4, 0, 0.2, 1);
}

/* ===================
   表格样式
   =================== */
:deep(.n-data-table) {
  border: none;
  border-radius: 0;
}

:deep(.n-data-table .n-data-table-th) {
  background: #f3f4f6;
  font-weight: 600;
  font-size: 13px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  padding: 18px 16px;
  color: #1f2937;
  border-bottom: 2px solid #e5e7eb;
}

:deep(.n-data-table .n-data-table-td) {
  padding: 16px;
  border-bottom: 1px solid #e5e7eb;
  color: #4b5563;
  font-size: 14px;
  vertical-align: middle;
}

:deep(.n-data-table .n-data-table-tr:last-child .n-data-table-td) {
  border-bottom: none;
}

:deep(.n-data-table .n-data-table-tr:hover .n-data-table-td) {
  background: #f9fafb;
}

:deep(.n-data-table .n-data-table-tr) {
  transition: background var(--transition-fast);
}

/* ===================
   操作按钮
   =================== */
.action-buttons {
  display: flex;
  gap: 8px;
}

.action-buttons :deep(.n-button) {
  min-width: auto;
  padding: 6px 14px;
  height: 32px;
  font-size: 13px;
  font-weight: 500;
  border-radius: var(--radius-md);
  transition: all 200ms cubic-bezier(0.4, 0, 0.2, 1);
  color: #1f2937;
}

.action-buttons :deep(.n-button:hover) {
  transform: translateY(-1px);
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

.action-buttons :deep(.n-button--error) {
  background: var(--error-color);
  border-color: var(--error-color);
  color: white;
}

.action-buttons :deep(.n-button--error:hover) {
  background: #dc2626;
  border-color: #dc2626;
  box-shadow: 0 4px 12px rgba(239, 68, 68, 0.3);
  color: white;
}

/* ===================
   标签样式
   =================== */
:deep(.n-tag) {
  border-radius: var(--radius-md);
  padding: 6px 14px;
  font-weight: 600;
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.3px;
  border: none;
}

:deep(.n-tag--success) {
  background: var(--success-light);
  color: var(--success-color);
}

:deep(.n-tag--default) {
  background: var(--gray-200);
  color: var(--gray-600);
}

/* ===================
   令牌信息样式
   =================== */
.token-label {
  font-size: 14px;
  font-weight: 600;
  color: #1f2937;
  margin-bottom: 8px;
}

.token-value {
  font-size: 14px;
  color: #4b5563;
  padding: 12px 16px;
  background: #f3f4f6;
  border-radius: var(--radius-md);
  border: 1px solid #e5e7eb;
  line-height: 1.6;
}

.token-display-container {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
  background: #f8fafc;
  border: 2px solid #e2e8f0;
  border-radius: 8px;
  transition: all 0.2s ease;
}

.token-display-container:hover {
  border-color: #3b82f6;
  background: #eff6ff;
}

.token-display {
  flex: 1;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', 'Consolas', 'source-code-pro', monospace;
  font-size: 13px;
  color: #1e293b;
  word-break: break-all;
  line-height: 1.6;
}

.copy-icon-button {
  flex-shrink: 0;
  width: 36px;
  height: 36px;
  border-radius: 6px;
  color: #64748b;
  transition: all 0.2s ease;
}

.copy-icon-button:hover {
  color: #3b82f6;
  background: rgba(59, 130, 246, 0.1);
}

.copy-icon-button:active {
  transform: scale(0.95);
}


/* ===================
   模态框样式
   =================== */
:deep(.n-modal) {
  border-radius: var(--radius-xl);
  box-shadow: 0 25px 50px rgba(0, 0, 0, 0.25);
}

:deep(.n-dialog) {
  border-radius: var(--radius-xl);
  box-shadow: 0 25px 50px rgba(0, 0, 0, 0.25);
  padding: 32px;
}

:deep(.n-dialog .n-dialog__title) {
  font-size: 20px;
  font-weight: 700;
  color: #1f2937;
}

/* ===================
   警告提示样式
   =================== */
:deep(.n-alert) {
  border-radius: var(--radius-lg);
  border: none;
}

:deep(.n-alert--warning) {
  background: var(--warning-light);
  color: #92400e;
}

/* ===================
   响应式设计
   =================== */
@media (max-width: 768px) {
  .action-bar {
    justify-content: stretch;
  }

  .action-bar :deep(.n-button) {
    width: 100%;
  }

  :deep(.n-data-table .n-data-table-th),
  :deep(.n-data-table .n-data-table-td) {
    padding: 12px 8px;
    font-size: 13px;
  }

  .action-buttons {
    flex-direction: column;
    gap: 4px;
  }

  .action-buttons :deep(.n-button) {
    width: 100%;
  }
}
</style>
