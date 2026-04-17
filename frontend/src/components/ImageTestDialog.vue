<template>
  <n-modal
    :show="show"
    preset="card"
    title="图像编辑模型测试"
    style="width: 560px"
    :mask-closable="false"
    @update:show="handleShowUpdate"
  >
    <n-form label-placement="top" size="medium">
      <n-form-item label="编辑提示词">
        <n-input
          v-model:value="form.prompt"
          type="textarea"
          :autosize="{ minRows: 2, maxRows: 4 }"
          placeholder="例如：将背景改成雪山日落，并保持主体不变"
        />
      </n-form-item>
      <n-form-item label="测试图片">
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
import { reactive, watch } from 'vue'
import { NButton, NForm, NFormItem, NInput, NModal, NSpace, NText } from 'naive-ui'

interface Props {
  show: boolean
  initialPrompt?: string
  loading?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  initialPrompt: '',
  loading: false,
})

const emit = defineEmits<{
  'update:show': [value: boolean]
  submit: [payload: { prompt: string; imageData: string }]
}>()

const form = reactive({
  prompt: '',
  imageData: '',
  fileName: '',
})

watch(
  () => props.show,
  (val) => {
    if (val) {
      form.prompt = props.initialPrompt || ''
      form.imageData = ''
      form.fileName = ''
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
    window.$message?.error('读取测试图片失败')
  }
  reader.readAsDataURL(file)
}

function handleConfirm() {
  if (!form.prompt.trim()) {
    window.$message?.warning('请输入图像编辑提示词')
    return
  }
  if (!form.imageData) {
    window.$message?.warning('请上传测试图片')
    return
  }
  emit('submit', {
    prompt: form.prompt.trim(),
    imageData: form.imageData,
  })
}
</script>

<style scoped>
.image-test-upload {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.image-test-upload__input {
  font-size: 12px;
}
</style>
