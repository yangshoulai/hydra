<template>
  <n-modal
    :show="show"
    preset="card"
    :title="dialogTitle"
    style="width: 560px"
    :mask-closable="false"
    @update:show="handleShowUpdate"
  >
    <n-form label-placement="top" size="medium">
      <n-form-item v-if="hasTextEndpoint" label="文本端点测试提示词（可选）">
        <n-input
          v-model:value="form.textPrompt"
          type="textarea"
          :autosize="{ minRows: 2, maxRows: 4 }"
          placeholder="为空时使用系统设置中的默认测试提示词"
        />
      </n-form-item>

      <n-form-item v-if="hasGenerationEndpoint" label="图像生成提示词">
        <n-input
          v-model:value="form.generationPrompt"
          type="textarea"
          :autosize="{ minRows: 2, maxRows: 4 }"
          placeholder="例如：请生成一只戴着耳机的柯基"
        />
      </n-form-item>

      <n-form-item v-if="hasEditEndpoint" label="图像编辑提示词">
        <n-input
          v-model:value="form.editPrompt"
          type="textarea"
          :autosize="{ minRows: 2, maxRows: 4 }"
          placeholder="例如：将背景改成雪山日落，并保持主体不变"
        />
      </n-form-item>

      <div v-if="hasImageEndpoint" class="image-test-option-grid">
        <n-form-item label="图片分辨率 size">
          <div class="image-test-field">
            <n-select
              v-model:value="form.size"
              :options="sizeOptions"
              placeholder="请选择图片分辨率"
            />
            <p class="form-hint">测试请求会将该值写入上游接口的 size 参数。</p>
          </div>
        </n-form-item>
        <n-form-item label="图片质量 quality">
          <div class="image-test-field">
            <n-select
              v-model:value="form.quality"
              :options="qualityOptions"
              placeholder="请选择图片质量"
            />
            <p class="form-hint">默认使用 low，便于降低模型测试成本与等待时间。</p>
          </div>
        </n-form-item>
      </div>

      <n-form-item v-if="hasEditEndpoint" label="测试图片">
        <div class="image-test-upload">
          <input
            class="image-test-upload__input"
            type="file"
            accept="image/*"
            @change="handleFileChange"
          />
          <n-text depth="3" style="font-size: 12px">
            {{ form.fileName || '请选择一张用于图像编辑测试的图片' }}
          </n-text>
        </div>
      </n-form-item>
    </n-form>

    <template #footer>
      <n-space justify="end">
        <n-button @click="handleCancel">取消</n-button>
        <n-button type="primary" :loading="loading" @click="handleConfirm">开始测试</n-button>
      </n-space>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { NButton, NForm, NFormItem, NInput, NModal, NSelect, NSpace, NText } from 'naive-ui'
import { feedback } from '@/services/feedback'

type ImageTestMode = 'generation' | 'edit' | 'mixed'

const defaultImageSize = '1024x1024'
const defaultImageQuality = 'low'

interface Props {
  show: boolean
  initialPrompt?: string
  initialTextPrompt?: string
  initialGenerationPrompt?: string
  initialEditPrompt?: string
  initialSize?: string
  initialQuality?: string
  loading?: boolean
  mode?: ImageTestMode
  endpointTypes?: string[]
}

const props = withDefaults(defineProps<Props>(), {
  initialPrompt: '',
  initialTextPrompt: '',
  initialGenerationPrompt: '',
  initialEditPrompt: '',
  initialSize: defaultImageSize,
  initialQuality: defaultImageQuality,
  loading: false,
  mode: 'edit',
  endpointTypes: () => [],
})

const emit = defineEmits<{
  'update:show': [value: boolean]
  submit: [payload: {
    prompt: string
    textPrompt: string
    generationPrompt: string
    editPrompt: string
    imageData: string
    size: string
    quality: string
  }]
}>()

const sizeOptions = [
  { label: '1024x1024', value: '1024x1024' },
  { label: '1024x1536', value: '1024x1536' },
  { label: '1536x1024', value: '1536x1024' },
  { label: '1024x1792', value: '1024x1792' },
  { label: '1792x1024', value: '1792x1024' },
  { label: '512x512', value: '512x512' },
  { label: '256x256', value: '256x256' },
]

const qualityOptions = [
  { label: 'low', value: 'low' },
  { label: 'medium', value: 'medium' },
  { label: 'high', value: 'high' },
  { label: 'auto', value: 'auto' },
]

const form = reactive({
  textPrompt: '',
  generationPrompt: '',
  editPrompt: '',
  imageData: '',
  fileName: '',
  size: defaultImageSize,
  quality: defaultImageQuality,
})

const explicitEndpointTypes = computed(() => props.endpointTypes || [])
const hasExplicitEndpointTypes = computed(() => explicitEndpointTypes.value.length > 0)
const hasGenerationEndpoint = computed(() => {
  if (hasExplicitEndpointTypes.value) return explicitEndpointTypes.value.includes('OpenAIImagesGenerations')
  return props.mode === 'generation' || props.mode === 'mixed'
})
const hasEditEndpoint = computed(() => {
  if (hasExplicitEndpointTypes.value) return explicitEndpointTypes.value.includes('OpenAIImagesEdits')
  return props.mode === 'edit' || props.mode === 'mixed'
})
const hasTextEndpoint = computed(() => {
  if (!hasExplicitEndpointTypes.value) return false
  return explicitEndpointTypes.value.some((type) => type !== 'OpenAIImagesGenerations' && type !== 'OpenAIImagesEdits')
})
const hasImageEndpoint = computed(() => hasGenerationEndpoint.value || hasEditEndpoint.value)

const dialogTitle = computed(() => {
  if (hasTextEndpoint.value && hasImageEndpoint.value) return '多端点模型测试'
  if (hasGenerationEndpoint.value && hasEditEndpoint.value) return '图像模型测试'
  if (hasGenerationEndpoint.value) return '图像生成模型测试'
  if (hasEditEndpoint.value) return '图像编辑模型测试'
  return '模型测试'
})

watch(
  () => props.show,
  (val) => {
    if (val) {
      const commonPrompt = props.initialPrompt || ''
      form.textPrompt = props.initialTextPrompt || commonPrompt
      form.generationPrompt = props.initialGenerationPrompt || commonPrompt
      form.editPrompt = props.initialEditPrompt || commonPrompt
      form.imageData = ''
      form.fileName = ''
      form.size = props.initialSize || defaultImageSize
      form.quality = props.initialQuality || defaultImageQuality
    }
  },
  { immediate: true },
)

function handleShowUpdate(value: boolean) {
  emit('update:show', value)
}

function handleCancel() {
  emit('update:show', false)
}

function handleFileChange(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  if (!file) {
    form.imageData = ''
    form.fileName = ''
    return
  }
  const reader = new FileReader()
  reader.onload = () => {
    form.imageData = typeof reader.result === 'string' ? reader.result : ''
    form.fileName = file.name
  }
  reader.onerror = () => {
    form.imageData = ''
    form.fileName = ''
    feedback.message?.error('读取测试图片失败')
  }
  reader.readAsDataURL(file)
}

function handleConfirm() {
  if (hasGenerationEndpoint.value && !form.generationPrompt.trim()) {
    feedback.message?.warning('请输入图像生成提示词')
    return
  }
  if (hasEditEndpoint.value && !form.editPrompt.trim()) {
    feedback.message?.warning('请输入图像编辑提示词')
    return
  }
  if (hasEditEndpoint.value && !form.imageData) {
    feedback.message?.warning('请上传测试图片')
    return
  }
  const textPrompt = form.textPrompt.trim()
  const generationPrompt = form.generationPrompt.trim()
  const editPrompt = form.editPrompt.trim()
  emit('submit', {
    prompt: generationPrompt || editPrompt || textPrompt,
    textPrompt,
    generationPrompt,
    editPrompt,
    imageData: hasEditEndpoint.value ? form.imageData : '',
    size: form.size || defaultImageSize,
    quality: form.quality || defaultImageQuality,
  })
}
</script>

<style scoped>
.image-test-option-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  column-gap: 12px;
}

.image-test-field {
  width: 100%;
}

.image-test-upload {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.image-test-upload__input {
  font-size: 12px;
}

@media (max-width: 640px) {
  .image-test-option-grid {
    grid-template-columns: 1fr;
  }
}
</style>
