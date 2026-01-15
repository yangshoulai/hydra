<template>
  <n-modal
      v-model:show="showModal"
      preset="card"
      :title="isEdit ? '编辑渠道' : '新建渠道'"
      :style="{ width: '800px' }"
      :mask-closable="false"
      :closable="true"
      @close="handleCancel"
      @keydown.esc="handleCancel"
  >
    <n-form
        ref="formRef"
        :model="formData"
        :rules="rules"
        label-placement="left"
        label-width="120"
        require-mark-placement="right-hanging"
    >
      <n-grid :cols="2" :x-gap="24" responsive="screen">
        <n-grid-item span="2">
          <n-form-item label="渠道名称" path="name">
            <n-input
                v-model:value="formData.name"
                placeholder="例如：OpenAI 官方渠道"
            />
          </n-form-item>
          <n-text depth="3"
                  style="font-size: 12px; margin-top: -16px; margin-bottom: 16px; display: block; padding-left: 120px;">
            为渠道取一个易于识别的名称
          </n-text>
        </n-grid-item>

        <n-grid-item span="2">
          <n-form-item label="Base URL" path="base_url">
            <n-input
                v-model:value="formData.base_url"
                placeholder="https://api.openai.com"
            />
          </n-form-item>
          <n-text depth="3"
                  style="font-size: 12px; margin-top: -16px; margin-bottom: 16px; display: block; padding-left: 120px;">
            API 的完整地址，例如：https://api.openai.com
          </n-text>
        </n-grid-item>

        <n-grid-item>
          <n-form-item label="优先级" path="priority">
            <n-input-number
                v-model:value="formData.priority"
                :min="1"
                :max="1000"
                placeholder="1-1000"
                style="width: 100%"
            />
          </n-form-item>
          <n-text depth="3"
                  style="font-size: 12px; margin-top: -16px; margin-bottom: 16px; display: block; padding-left: 120px;">
            数值越大优先级越高，范围 1-1000
          </n-text>
        </n-grid-item>

        <n-grid-item>
          <n-form-item label="权重" path="weight">
            <n-input-number
                v-model:value="formData.weight"
                :min="1"
                :max="100"
                placeholder="1-100"
                style="width: 100%"
            />
          </n-form-item>
          <n-text depth="3" style="font-size: 12px; margin-top: -16px; margin-bottom: 16px; display: block; padding-left: 120px;">
            用于同优先级渠道的负载均衡，范围 1-100
          </n-text>
        </n-grid-item>

        <n-grid-item>
          <n-form-item label="状态" path="status">
            <n-select
                v-model:value="formData.status"
                :options="statusOptions"
                placeholder="选择状态"
            />
          </n-form-item>
        </n-grid-item>

        <n-grid-item>
          <!-- 空白列，保持布局对齐 -->
        </n-grid-item>

        <n-grid-item span="2">
          <n-form-item label="描述" path="description">
            <n-input
                v-model:value="formData.description"
                type="textarea"
                placeholder="请输入渠道描述（可选）"
                :autosize="{ minRows: 3, maxRows: 6 }"
            />
          </n-form-item>
        </n-grid-item>
      </n-grid>
    </n-form>

    <template #footer>
      <n-space justify="end">
        <n-button @click="handleCancel">
          取消
        </n-button>
        <n-button type="primary" @click="handleSubmit">
          {{ isEdit ? '保存' : '创建' }}
        </n-button>
      </n-space>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import {computed, reactive, ref, watch} from 'vue'
import {
  type FormInst,
  type FormRules,
  NButton,
  NForm,
  NFormItem,
  NGrid,
  NGridItem,
  NInput,
  NInputNumber,
  NModal,
  NSelect,
  NSpace,
  NText
} from 'naive-ui'
import type {Channel, CreateChannelRequest, UpdateChannelRequest} from '../types/channel'

interface Props {
  channel?: Channel | null
}

interface Emits {
  (e: 'confirm', data: CreateChannelRequest | UpdateChannelRequest): void

  (e: 'cancel'): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const formRef = ref<FormInst | null>(null)
const showModal = ref(true)

// 是否为编辑模式
const isEdit = computed(() => !!props.channel)

// 表单数据
const formData = reactive({
  name: '',
  base_url: '',
  priority: 100,
  weight: 100,
  status: 'active' as 'active' | 'disabled',
  description: ''
})

// 状态选项
const statusOptions = [
  {label: '激活', value: 'active'},
  {label: '禁用', value: 'disabled'}
]

// 表单验证规则
const rules: FormRules = {
  name: {
    required: true,
    message: '请输入渠道名称',
    trigger: ['blur', 'input']
  },
  base_url: {
    required: true,
    message: '请输入Base URL',
    trigger: ['blur', 'input']
  },
  priority: {
    type: 'number',
    required: true,
    message: '请输入优先级',
    trigger: ['blur', 'change']
  },
  weight: {
    type: 'number',
    required: true,
    message: '请输入权重',
    trigger: ['blur', 'change']
  }
}

// 监听channel变化，填充表单
watch(
    () => props.channel,
    (newChannel) => {
      if (newChannel) {
        formData.name = newChannel.name
        formData.base_url = newChannel.base_url
        formData.priority = newChannel.priority
        formData.weight = newChannel.weight
        formData.status = newChannel.status
        formData.description = newChannel.description
      } else {
        // 重置表单
        formData.name = ''
        formData.base_url = ''
        formData.priority = 100
        formData.weight = 100
        formData.status = 'active'
        formData.description = ''
      }
      // 每次打开时重置显示状态
      showModal.value = true
    },
    {immediate: true}
)

// 提交表单
async function handleSubmit() {
  if (!formRef.value) return

  try {
    await formRef.value.validate()

    const data = isEdit.value
        ? ({
          name: formData.name,
          base_url: formData.base_url,
          priority: formData.priority,
          weight: formData.weight,
          status: formData.status,
          description: formData.description
        } as UpdateChannelRequest)
        : ({
          name: formData.name,
          base_url: formData.base_url,
          priority: formData.priority,
          weight: formData.weight,
          description: formData.description
        } as CreateChannelRequest)

    emit('confirm', data)
    showModal.value = false
  } catch (error) {
    console.error('Form validation failed:', error)
  }
}

// 取消
function handleCancel() {
  showModal.value = false
  setTimeout(() => {
    emit('cancel')
  }, 100)
}
</script>
