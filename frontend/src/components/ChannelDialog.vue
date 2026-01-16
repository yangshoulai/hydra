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
      :bordered="false"
      class="channel-dialog"
  >
    <n-space vertical :size="24">
      <!-- 提示信息 -->
      <n-alert type="info" :bordered="false" class="info-alert">
        <template #icon>
          <n-icon>
            <InformationCircleOutline/>
          </n-icon>
        </template>
        {{ isEdit ? '修改渠道配置信息。优先级和权重将影响渠道的调用顺序和负载分配。' : '配置新的 API 渠道。渠道将按照优先级和权重进行负载均衡。' }}
      </n-alert>

      <n-form
          ref="formRef"
          :model="formData"
          :rules="rules"
          label-placement="left"
          label-width="120"
          require-mark-placement="right-hanging"
          size="large"
      >
        <n-grid :cols="2" :x-gap="24" responsive="screen">
          <n-grid-item span="2">
            <n-form-item label="渠道名称" path="name">
              <n-input
                  v-model:value="formData.name"
                  placeholder="例如：OpenAI 官方渠道"
              >
                <template #prefix>
                  <n-icon>
                    <BookmarkOutline/>
                  </n-icon>
                </template>
              </n-input>
              <template #feedback>
                为渠道取一个易于识别的名称
              </template>
            </n-form-item>
          </n-grid-item>

          <n-grid-item span="2">
            <n-form-item label="Base URL" path="base_url">
              <n-input
                  v-model:value="formData.base_url"
                  placeholder="https://api.openai.com"
              >
                <template #prefix>
                  <n-icon>
                    <GlobeOutline/>
                  </n-icon>
                </template>
              </n-input>
              <template #feedback>
                API 的完整地址，例如：https://api.openai.com
              </template>
            </n-form-item>
          </n-grid-item>

          <n-grid-item>
            <n-form-item label="优先级" path="priority">
              <n-input-number
                  v-model:value="formData.priority"
                  :min="1"
                  :max="1000"
                  placeholder="1-1000"
                  style="width: 100%"
              >
                <template #prefix>
                  <n-icon>
                    <TrendingUpOutline/>
                  </n-icon>
                </template>
              </n-input-number>
              <template #feedback>
                数值越大优先级越高，范围 1-1000
              </template>
            </n-form-item>
          </n-grid-item>

          <n-grid-item>
            <n-form-item label="权重" path="weight">
              <n-input-number
                  v-model:value="formData.weight"
                  :min="1"
                  :max="100"
                  placeholder="1-100"
                  style="width: 100%"
              >
                <template #prefix>
                  <n-icon>
                    <ScaleOutline/>
                  </n-icon>
                </template>
              </n-input-number>
              <template #feedback>
                用于同优先级渠道的负载均衡，范围 1-100
              </template>
            </n-form-item>
          </n-grid-item>

          <n-grid-item>
            <n-form-item label="状态" path="status">
              <n-select
                  v-model:value="formData.status"
                  :options="statusOptions"
                  placeholder="选择状态"
              >
                <template #prefix>
                  <n-icon>
                    <PowerOutline/>
                  </n-icon>
                </template>
              </n-select>
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
              >
                <template #prefix>
                  <n-icon>
                    <DocumentTextOutline/>
                  </n-icon>
                </template>
              </n-input>
            </n-form-item>
          </n-grid-item>
        </n-grid>
      </n-form>
    </n-space>

    <template #footer>
      <n-space justify="end" :size="12">
        <n-button @click="handleCancel" size="large">
          取消
        </n-button>
        <n-button type="primary" @click="handleSubmit" size="large" strong>
          <template #icon>
            <n-icon>
              <SaveOutline/>
            </n-icon>
          </template>
          {{ isEdit ? '保存' : '创建' }}
        </n-button>
      </n-space>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
// @ts-nocheck
import {computed, reactive, ref, watch} from 'vue'
import {
  type FormInst,
  type FormRules,
  NAlert,
  NButton,
  NForm,
  NFormItem,
  NGrid,
  NGridItem,
  NIcon,
  NInput,
  NInputNumber,
  NModal,
  NSelect,
  NSpace
} from 'naive-ui'
import {
  BookmarkOutline,
  DocumentTextOutline,
  GlobeOutline,
  InformationCircleOutline,
  PowerOutline,
  SaveOutline,
  ScaleOutline,
  TrendingUpOutline
} from '@vicons/ionicons5'
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

<style scoped>
/* 对话框样式 */
.channel-dialog {
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

/* 表单项样式优化 */
.channel-dialog :deep(.n-form-item-label) {
  font-weight: 600;
  color: #333;
  font-size: 14px;
  padding-bottom: 8px;
}

.channel-dialog :deep(.n-form-item-feedback) {
  font-size: 12px;
  color: #999;
  margin-top: 4px;
}

/* 输入框样式优化 */
.channel-dialog :deep(.n-input),
.channel-dialog :deep(.n-input-number),
.channel-dialog :deep(.n-base-selection) {
  border-radius: 8px;
  transition: all 0.3s;
}

.channel-dialog :deep(.n-input:focus),
.channel-dialog :deep(.n-input-number:focus),
.channel-dialog :deep(.n-base-selection:focus) {
  box-shadow: 0 0 0 2px rgba(24, 160, 88, 0.1);
}

/* 图标样式 */
.channel-dialog :deep(.n-input__prefix),
.channel-dialog :deep(.n-input-number__prefix) {
  color: #999;
  margin-right: 8px;
}

.channel-dialog :deep(.n-input:focus .n-input__prefix),
.channel-dialog :deep(.n-input-number:focus .n-input-number__prefix) {
  color: var(--primary-color);
}

/* 按钮样式优化 */
.channel-dialog :deep(.n-button) {
  transition: all 0.3s;
  border-radius: 8px;
}

.channel-dialog :deep(.n-button:hover) {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.channel-dialog :deep(.n-button--primary) {
  background: linear-gradient(135deg, var(--primary-color) 0%, var(--primary-color-hover) 100%);
  border: none;
}

.channel-dialog :deep(.n-button--primary:hover) {
  background: linear-gradient(135deg, var(--primary-color-hover) 0%, #40c478 100%);
}

/* Alert 样式优化 */
.channel-dialog :deep(.n-alert) {
  border-radius: 8px;
  padding: 16px 20px;
}

.channel-dialog :deep(.n-alert .n-alert-body) {
  font-size: 13px;
  line-height: 1.6;
}

/* 响应式调整 */
@media (max-width: 768px) {
  .channel-dialog {
    width: 95vw !important;
  }

  .channel-dialog :deep(.n-grid-item) {
    min-width: 100%;
  }
}
</style>
