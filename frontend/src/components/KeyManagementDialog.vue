<template>
  <n-modal
      v-model:show="show"
      preset="card"
      :title="`密钥管理 - ${channelName}`"
      style="width: 1600px"
      :mask-closable="false"
      :closable="true"
      @close="handleClose"
  >
    <n-space>
      <n-space>
        <n-button type="info" @click="handleTestKeys" :loading="testing" size="small">
          <template #icon>
            <n-icon>
              <PulseOutline/>
            </n-icon>
          </template>
          批量测试
        </n-button>
        <n-button type="primary" @click="showAddKeyDialog = true" size="small">
          添加密钥
        </n-button>
      </n-space>

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
    >
      <n-form ref="keyFormRef" :model="keyForm" :rules="keyRules" label-placement="left" label-width="120">
        <n-form-item label="密钥值" path="key_value">
          <n-input
              v-model:value="keyForm.key_value"
              type="textarea"
              placeholder="每行输入一个密钥，支持批量添加多个密钥&#10;例如：&#10;sk-xxxxxxxxxxxxxxxxxxxxxxxx&#10;sk-yyyyyyyyyyyyyyyyyyyyyyyy&#10;sk-zzzzzzzzzzzzzzzzzzzzzzzz"
              :autosize="{ minRows: 6, maxRows: 12 }"
          />
        </n-form-item>
        <n-text depth="3" style="font-size: 12px; margin-top: -16px; margin-bottom: 16px; display: block; padding-left: 120px;">
          每行输入一个密钥，支持批量添加多个密钥
        </n-text>

        <n-form-item label="备注" path="remark">
          <n-input
              v-model:value="keyForm.remark"
              type="textarea"
              placeholder="请输入备注信息（可选）"
              :autosize="{ minRows: 2, maxRows: 4 }"
          />
        </n-form-item>
        <n-text depth="3" style="font-size: 12px; margin-top: -16px; margin-bottom: 16px; display: block; padding-left: 120px;">
          为所有密钥添加统一的备注信息（可选）
        </n-text>
      </n-form>

      <template #footer>
        <n-space justify="end">
          <n-button @click="showAddKeyDialog = false">取消</n-button>
          <n-button type="primary" @click="handleAddKey" :loading="adding">
            添加 ({{ keyLineCount }})
          </n-button>
        </n-space>
      </template>
    </n-modal>
  </n-modal>
</template>

<script setup lang="ts">
import {computed, reactive, ref, watch} from 'vue'
import {NButton, NForm, NFormItem, NIcon, NInput, NModal, NSpace, NText} from 'naive-ui'
import {PulseOutline} from '@vicons/ionicons5'
import {channelApi} from '../services/channelService'
import KeyHealthTable from './KeyHealthTable.vue'
import type {ChannelHealthCheckResult} from '../types/channel'

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
  key_value: '',
  remark: ''
})

// 计算密钥行数（用于显示添加数量）
const keyLineCount = computed(() => {
  if (!keyForm.key_value.trim()) return 0
  return keyForm.key_value
    .split('\n')
    .map(line => line.trim())
    .filter(line => line.length > 0)
    .length
})

const keyRules = {
  key_value: {
    required: true,
    message: '请输入密钥值',
    trigger: ['blur', 'input']
  }
}

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
    // 先设置所有 Key 为测试中状态
    keyHealthTableRef.value?.setTesting(0)

    // 执行测试
    const result = await channelApi.testKeys(props.channelId)
    healthResult.value = result

    // 将测试结果传递给 KeyHealthTable
    keyHealthTableRef.value?.updateHealthResults(result)

    // 刷新列表
    await keyHealthTableRef.value?.refresh()

    // 通知父组件刷新
    emit('refresh')

    window.$message?.success('测活完成')
  } catch (error: any) {
    console.error('Failed to test keys:', error)
    window.$message?.error(error.response?.data?.error || '测试Keys失败')
    // 测试失败，清除测试中状态
    keyHealthTableRef.value?.clearTesting()
  } finally {
    testing.value = false
  }
}

// 添加Key（支持批量添加）
async function handleAddKey() {
  if (!keyForm.key_value.trim()) {
    window.$message?.error('请输入密钥值')
    return
  }

  // 分割多行密钥，去除空行
  const keys = keyForm.key_value
    .split('\n')
    .map(line => line.trim())
    .filter(line => line.length > 0)

  if (keys.length === 0) {
    window.$message?.error('请输入有效的密钥值')
    return
  }

  adding.value = true

  try {
    // 使用批量添加接口
    const result = await channelApi.batchAddKeys(
      props.channelId,
      keys,
      keyForm.remark
    )

    // 显示结果
    if (result.failed_count === 0) {
      window.$message?.success(`成功添加 ${result.success_count} 个密钥`)
      showAddKeyDialog.value = false
      keyForm.key_value = ''
      keyForm.remark = ''

      // 刷新Key列表
      await keyHealthTableRef.value?.refresh()

      // 通知父组件刷新
      emit('refresh')
    } else if (result.success_count === 0) {
      window.$message?.error(`添加失败，请检查密钥格式`)
    } else {
      window.$message?.warning(`添加完成：成功 ${result.success_count} 个，失败 ${result.failed_count} 个`)
      // 如果有成功的，也关闭对话框并刷新
      showAddKeyDialog.value = false
      keyForm.key_value = ''
      keyForm.remark = ''

      // 刷新Key列表
      await keyHealthTableRef.value?.refresh()

      // 通知父组件刷新
      emit('refresh')
    }
  } catch (error: any) {
    console.error('Failed to add keys:', error)
    window.$message?.error(error.response?.data?.error || '添加失败')
  } finally {
    adding.value = false
  }
}

// 监听对话框打开，刷新数据
watch(() => props.modelValue, (newVal) => {
  if (newVal) {
    // 对话框打开时，刷新健康检查结果
    healthResult.value = undefined
  }
})
</script>

<style scoped>
/* 继承父组件样式 */
</style>
