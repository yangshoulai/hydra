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
        :scroll-x="1680"
        :single-line="false"
        striped
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
        <n-text depth="3"
                style="font-size: 12px; margin-top: -16px; margin-bottom: 16px; display: block; padding-left: 120px;">
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
        <n-text v-if="expireType === 'custom'" depth="3"
                style="font-size: 12px; margin-top: -16px; margin-bottom: 16px; display: block; padding-left: 120px;">
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
    width: 120,
    align: 'left'
  },
  {
    title: '名称',
    key: 'name',
    width: 200,
  },
  {
    title: '令牌',
    key: 'token_preview',
    width: 280,
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
  },
  {
    title: '最后使用',
    key: 'last_used_at',
    width: 200,
    align: 'center',
    render: (row) => {
      return row.last_used_at || '-'
    },
  },
  {
    title: '操作',
    key: 'actions',
    width: 200,
    align: 'center',
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
</style>
