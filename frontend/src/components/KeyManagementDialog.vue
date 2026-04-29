<template>
  <n-modal
      v-model:show="show"
      preset="card"
      :title="`密钥管理 - ${channelName}`"
      style="width: 1200px"
      :mask-closable="false"
      :closable="true"
      @close="handleClose"
      :bordered="false"
      class="key-management-dialog"
  >
    <template #header-extra>
      <n-space>
        <n-button
            type="info"
            @click="handleTestKeys"
            :loading="testing"
            size="medium"
            secondary
            strong
        >
          <template #icon>
            <n-icon>
              <PulseOutline/>
            </n-icon>
          </template>
          批量测试
        </n-button>
        <n-button
            type="primary"
            @click="showAddKeyDialog = true"
            size="medium"
            strong
        >
          <template #icon>
            <n-icon>
              <AddOutline/>
            </n-icon>
          </template>
          添加密钥
        </n-button>
      </n-space>
    </template>

    <n-space vertical :size="24">
      <!-- 说明信息 -->
      <n-alert type="info" :bordered="false" class="info-alert">
        <template #icon>
          <n-icon>
            <InformationCircleOutline/>
          </n-icon>
        </template>
        管理该渠道的所有 API 密钥。支持批量添加密钥，并可进行健康检查测试。
      </n-alert>

      <!-- Key 列表表格 -->
      <KeyHealthTable
          ref="keyHealthTableRef"
          :channel-id="channelId"
          :health-result="healthResult"
          @refresh="handleRefresh"
      />
    </n-space>

    <!-- 添加Key对话框 -->
    <n-modal
        v-model:show="showAddKeyDialog"
        preset="card"
        title="添加密钥"
        :style="{ width: '800px' }"
        :mask-closable="false"
        :closable="true"
        @close="showAddKeyDialog = false"
        @keydown.esc="showAddKeyDialog = false"
        :bordered="false"
        class="add-key-dialog"
    >
      <n-space vertical :size="20">
        <n-alert type="info" :bordered="false">
          <template #icon>
            <n-icon>
              <InformationCircleOutline/>
            </n-icon>
          </template>
          支持批量添加多个密钥，每行一个。所有密钥将使用统一的备注信息。
        </n-alert>

        <n-form
            ref="keyFormRef"
            :model="keyForm"
            :rules="keyRules"
            label-placement="top"
            size="large"
        >
          <n-form-item label="密钥值" path="channel_key_value">
            <n-input
                v-model:value="keyForm.channel_key_value"
                type="textarea"
                placeholder="每行输入一个密钥，支持批量添加多个密钥&#10;例如：&#10;sk-xxxxxxxxxxxxxxxxxxxxxxxx&#10;sk-yyyyyyyyyyyyyyyyyyyyyyyy&#10;sk-zzzzzzzzzzzzzzzzzzzzzzzz"
                :autosize="{ minRows: 6, maxRows: 12 }"
            >
              <template #prefix>
                <n-icon>
                  <KeyOutline/>
                </n-icon>
              </template>
            </n-input>
            <template #feedback>
              每行输入一个密钥，支持批量添加多个密钥
            </template>
          </n-form-item>

          <n-form-item label="密钥分组" path="channel_key_group">
            <n-select
                v-model:value="keyForm.channel_key_group"
                :options="keyGroupOptions"
                placeholder="请选择或输入分组"
                filterable
                tag
            />
            <template #feedback>
              密钥分组用于限制可访问的模型范围，默认分组为 Default
            </template>
          </n-form-item>

          <n-form-item label="备注" path="remark">
            <n-input
                v-model:value="keyForm.remark"
                type="textarea"
                placeholder="请输入备注信息（可选）"
                :autosize="{ minRows: 2, maxRows: 4 }"
            >
              <template #prefix>
                <n-icon>
                  <TextOutline/>
                </n-icon>
              </template>
            </n-input>
            <template #feedback>
              为所有密钥添加统一的备注信息（可选）
            </template>
          </n-form-item>
        </n-form>

        <!-- 统计信息 -->
        <n-card size="small" :bordered="false" class="stats-card">
          <n-space justify="space-between" align="center">
            <n-text depth="3">
              已输入 <n-text strong>{{ keyLineCount }}</n-text> 个密钥
            </n-text>
            <n-tag v-if="keyLineCount > 0" type="info" size="small">
              批量添加
            </n-tag>
          </n-space>
        </n-card>
      </n-space>

      <template #footer>
        <n-space justify="end" :size="12">
          <n-button @click="showAddKeyDialog = false" size="large">
            取消
          </n-button>
          <n-button
              type="primary"
              @click="handleAddKey"
              :loading="adding"
              size="large"
              strong
          >
            <template #icon>
              <n-icon>
                <AddOutline/>
              </n-icon>
            </template>
            添加 ({{ keyLineCount }})
          </n-button>
        </n-space>
      </template>
    </n-modal>
  </n-modal>
</template>

<script setup lang="ts">
import {computed, reactive, ref, watch} from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NForm,
  NFormItem,
  NIcon,
  NInput,
  NModal,
  NSelect,
  NSpace,
  NTag,
  NText
} from 'naive-ui'
import {
  AddOutline,
  InformationCircleOutline,
  KeyOutline,
  PulseOutline,
  TextOutline
} from '@vicons/ionicons5'
import {channelApi} from '../services/channelService'
import KeyHealthTable from './KeyHealthTable.vue'
import type {ChannelHealthCheckResult} from '../types/channel'
import {toastApiError} from '@/utils/error'
import { feedback } from '@/services/feedback'

interface Props {
  channelId: number
  channelName: string
  modelValue: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  refresh: []
}>()

const show = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit('update:modelValue', value)
})

const showAddKeyDialog = ref(false)
const testing = ref(false)
const adding = ref(false)
const healthResult = ref<ChannelHealthCheckResult | undefined>(undefined)
const keyHealthTableRef = ref<InstanceType<typeof KeyHealthTable> | null>(null)

// Key表单
const keyForm = reactive({
  channel_key_value: '',
  remark: '',
  channel_key_group: 'Default'
})

// 计算密钥行数（用于显示添加数量）
const keyLineCount = computed(() => {
  if (!keyForm.channel_key_value.trim()) return 0
  return keyForm.channel_key_value
      .split('\n')
      .map(line => line.trim())
      .filter(line => line.length > 0)
      .length
})

const keyRules = {
  channel_key_value: {
    required: true,
    message: '请输入密钥值',
    trigger: ['blur', 'input']
  },
  channel_key_group: {
    required: true,
    message: '请选择密钥分组',
    trigger: ['blur', 'change']
  }
}

const keyGroupOptions = ref<{ label: string; value: string }[]>([
  { label: 'Default', value: 'Default' }
])

// 关闭对话框
function handleClose() {
  emit('update:modelValue', false)
}

// 刷新
function handleRefresh() {
  emit('refresh')
}

// 批量测试Keys
async function handleTestKeys() {
  testing.value = true
  try {
    // 设置所有 Key 为测试中状态
    keyHealthTableRef.value?.setTestingAll()

    // 执行测试
    const result = await channelApi.testChannelKeys(props.channelId)
    healthResult.value = result

    // 更新测试结果
    keyHealthTableRef.value?.updateTestResults(result)

    // 刷新列表
    await keyHealthTableRef.value?.refresh()

    // 通知父组件刷新
    emit('refresh')

    feedback.message?.success(`测活完成: 健康 ${result.healthy_channel_keys}/${result.total_channel_keys}`)
  } catch (err) {
    toastApiError(err, '测试Keys失败')
  } finally {
    testing.value = false
  }
}

// 添加Key（支持批量添加）
async function handleAddKey() {
  if (!keyForm.channel_key_value.trim()) {
    feedback.message?.error('请输入密钥值')
    return
  }

  const keyGroup = keyForm.channel_key_group.trim() || 'Default'

  // 分割多行密钥，去除空行
  const channelKeys = keyForm.channel_key_value
      .split('\n')
      .map(line => line.trim())
      .filter(line => line.length > 0)

  if (channelKeys.length === 0) {
    feedback.message?.error('请输入有效的密钥值')
    return
  }

  adding.value = true

  try {
    // 使用批量添加接口
    const result = await channelApi.batchAddChannelKeys(
        props.channelId,
        channelKeys,
        keyForm.remark,
        keyGroup
    )

    // 显示结果
    if (result.failed_count === 0) {
      feedback.message?.success(`成功添加 ${result.success_count} 个密钥`)
      showAddKeyDialog.value = false
      keyForm.channel_key_value = ''
      keyForm.remark = ''
      keyForm.channel_key_group = 'Default'

      // 刷新Key列表
      await keyHealthTableRef.value?.refresh()

      // 通知父组件刷新
      emit('refresh')
    } else if (result.success_count === 0) {
      feedback.message?.error(`添加失败，请检查密钥格式`)
    } else {
      feedback.message?.warning(`添加完成：成功 ${result.success_count} 个，失败 ${result.failed_count} 个`)
      // 如果有成功的，也关闭对话框并刷新
      showAddKeyDialog.value = false
      keyForm.channel_key_value = ''
      keyForm.remark = ''
      keyForm.channel_key_group = 'Default'

      // 刷新Key列表
      await keyHealthTableRef.value?.refresh()

      // 通知父组件刷新
      emit('refresh')
    }
  } catch (err) {
    toastApiError(err, '添加失败')
  } finally {
    adding.value = false
  }
}

// 监听对话框打开，刷新数据
watch(() => props.modelValue, (newVal) => {
  if (newVal) {
    // 对话框打开时，刷新健康检查结果
    healthResult.value = undefined
    loadKeyGroups()
  }
})

async function loadKeyGroups() {
  try {
    const channel = await channelApi.get(props.channelId)
    const groups = new Set<string>()
    channel.channel_keys?.forEach((channelKey) => {
      if (channelKey.channel_key_group) {
        groups.add(channelKey.channel_key_group)
      }
    })
    if (groups.size === 0) {
      groups.add('Default')
    }
    keyGroupOptions.value = Array.from(groups).sort().map((group) => ({
      label: group,
      value: group
    }))
  } catch (error) {
    keyGroupOptions.value = [{ label: 'Default', value: 'Default' }]
  }
}
</script>

<style scoped>
/* 对话框样式 */
.key-management-dialog {
  --primary-color: #18a058;
  --primary-color-hover: #36ad6a;
  --info-color: #2080f0;
  --warning-color: #f0a020;
  --success-color: #18a058;
  --error-color: #d03050;
}

/* 提示信息样式 */
.info-alert {
  background: linear-gradient(135deg, #f0f9ff 0%, #e0f2fe 100%);
  border-left: 4px solid var(--info-color);
}

/* 添加密钥对话框样式 */
.add-key-dialog {
  --n-border-radius: 12px;
}

.add-key-dialog :deep(.n-card__content) {
  padding: 24px;
}

/* 统计卡片样式 */
.stats-card {
  background: linear-gradient(135deg, #f5f7fa 0%, #ffffff 100%);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
  border-radius: 8px;
}

/* 表单项样式优化 */
.add-key-dialog :deep(.n-form-item-label) {
  font-weight: 600;
  color: #333;
  font-size: 14px;
  padding-bottom: 8px;
}

.add-key-dialog :deep(.n-form-item-feedback) {
  font-size: 12px;
  color: #999;
  margin-top: 4px;
}

/* 输入框样式优化 */
.add-key-dialog :deep(.n-input),
.add-key-dialog :deep(.n-base-selection) {
  border-radius: 8px;
  transition: all 0.3s;
}

.add-key-dialog :deep(.n-input:focus),
.add-key-dialog :deep(.n-base-selection:focus) {
  box-shadow: 0 0 0 2px rgba(24, 160, 88, 0.1);
}

/* 图标样式 */
.add-key-dialog :deep(.n-input__prefix) {
  color: #999;
  margin-right: 8px;
}

.add-key-dialog :deep(.n-input:focus .n-input__prefix) {
  color: var(--primary-color);
}

/* 按钮样式优化 */
.key-management-dialog :deep(.n-button),
.add-key-dialog :deep(.n-button) {
  transition: all 0.3s;
  border-radius: 8px;
}

.key-management-dialog :deep(.n-button:hover),
.add-key-dialog :deep(.n-button:hover) {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.key-management-dialog :deep(.n-button--primary),
.add-key-dialog :deep(.n-button--primary) {
  background: linear-gradient(135deg, var(--primary-color) 0%, var(--primary-color-hover) 100%);
  border: none;
}

.key-management-dialog :deep(.n-button--primary:hover),
.add-key-dialog :deep(.n-button--primary:hover) {
  background: linear-gradient(135deg, var(--primary-color-hover) 0%, #40c478 100%);
}

/* Alert 样式优化 */
.key-management-dialog :deep(.n-alert),
.add-key-dialog :deep(.n-alert) {
  border-radius: 8px;
  padding: 16px 20px;
}

.key-management-dialog :deep(.n-alert .n-alert-body),
.add-key-dialog :deep(.n-alert .n-alert-body) {
  font-size: 13px;
  line-height: 1.6;
}

/* 标签样式优化 */
.key-management-dialog :deep(.n-tag),
.add-key-dialog :deep(.n-tag) {
  border-radius: 6px;
  font-weight: 500;
  padding: 4px 12px;
}

/* 响应式调整 */
@media (max-width: 1200px) {
  .key-management-dialog {
    width: 95vw !important;
  }
}

@media (max-width: 768px) {
  .add-key-dialog {
    width: 95vw !important;
  }
}
</style>
