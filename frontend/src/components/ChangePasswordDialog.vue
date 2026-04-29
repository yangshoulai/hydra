<template>
  <n-modal v-model:show="showDialog" preset="card" title="修改密码" :style="{ width: '460px' }">
    <n-form ref="formRef" :model="formData" :rules="rules" label-placement="top" size="medium">
      <n-form-item label="当前密码" path="oldPassword">
        <n-input
          v-model:value="formData.oldPassword"
          type="password"
          placeholder="请输入当前密码"
          show-password-on="click"
        />
      </n-form-item>
      <n-form-item label="新密码" path="newPassword">
        <n-input
          v-model:value="formData.newPassword"
          type="password"
          placeholder="至少 8 位新密码"
          show-password-on="click"
        />
      </n-form-item>
      <n-form-item label="确认新密码" path="confirmPassword">
        <n-input
          v-model:value="formData.confirmPassword"
          type="password"
          placeholder="请再次输入新密码"
          show-password-on="click"
        />
      </n-form-item>
    </n-form>

    <template #footer>
      <n-space justify="end">
        <n-button @click="handleCancel">取消</n-button>
        <n-button type="primary" :loading="loading" @click="handleSubmit">确认修改</n-button>
      </n-space>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { NButton, NForm, NFormItem, NInput, NModal, NSpace, type FormInst, type FormRules } from 'naive-ui'
import { authApi } from '@/services/authService'
import { toastApiError } from '@/utils/error'
import { feedback } from '@/services/feedback'

const props = defineProps<{
  show: boolean
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
}>()

const showDialog = computed({
  get: () => props.show,
  set: (value) => emit('update:show', value),
})

const formRef = ref<FormInst | null>(null)
const loading = ref(false)

const formData = reactive({
  oldPassword: '',
  newPassword: '',
  confirmPassword: '',
})

const rules: FormRules = {
  oldPassword: [{ required: true, message: '请输入当前密码', trigger: 'blur' }],
  newPassword: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 8, message: '密码长度至少 8 位', trigger: 'blur' },
  ],
  confirmPassword: [
    { required: true, message: '请再次输入新密码', trigger: 'blur' },
    {
      validator: (_rule, value) => value === formData.newPassword,
      message: '两次输入的密码不一致',
      trigger: 'blur',
    },
  ],
}

function resetForm() {
  formData.oldPassword = ''
  formData.newPassword = ''
  formData.confirmPassword = ''
}

function handleCancel() {
  showDialog.value = false
  resetForm()
}

async function handleSubmit() {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }

  loading.value = true
  try {
    await authApi.changePassword({
      old_password: formData.oldPassword,
      new_password: formData.newPassword,
    })

    feedback.message?.success('密码修改成功')
    showDialog.value = false
    resetForm()
  } catch (err) {
    toastApiError(err, '密码修改失败')
  } finally {
    loading.value = false
  }
}
</script>
