<template>
  <div class="login-page">
    <div class="login-page__panel">
      <div class="login-page__form-wrap">
        <n-card :bordered="false" class="login-card">
          <template #header>
            <div class="login-card__header">
              <div class="login-brand__logo">
                <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" class="w-6 h-6">
                  <path d="M12 2L2 7L12 12L22 7L12 2Z" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
                  <path d="M2 17L12 22L22 17" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
                  <path d="M2 12L12 17L22 12" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
              </div>
              <div>
                <h2 class="login-card__title">Hydra Console</h2>
                <p class="login-card__desc">管理员登录</p>
              </div>
            </div>
          </template>

          <n-form
            ref="formRef"
            :model="formData"
            :rules="rules"
            label-placement="top"
            size="medium"
            @keydown.enter.prevent="handleLogin"
          >
            <n-form-item path="username" label="用户名">
              <n-input
                v-model:value="formData.username"
                placeholder="请输入用户名"
                :input-props="{ autocomplete: 'username' }"
              >
                <template #prefix>
                  <n-icon>
                    <PersonOutline />
                  </n-icon>
                </template>
              </n-input>
            </n-form-item>

            <n-form-item path="password" label="密码">
              <n-input
                v-model:value="formData.password"
                type="password"
                show-password-on="click"
                placeholder="请输入密码"
                :input-props="{ autocomplete: 'current-password' }"
              >
                <template #prefix>
                  <n-icon>
                    <LockClosedOutline />
                  </n-icon>
                </template>
              </n-input>
            </n-form-item>

            <n-space justify="start" style="margin: -2px 0 10px">
              <n-checkbox v-model:checked="rememberMe">记住登录</n-checkbox>
            </n-space>

            <n-button type="primary" block size="large" :loading="authStore.loading" @click="handleLogin">
              登录
            </n-button>
          </n-form>

          <n-alert v-if="authStore.error" type="error" style="margin-top: 14px" :bordered="false">
            {{ authStore.error }}
          </n-alert>
        </n-card>
      </div>
    </div>

    <p class="login-page__copyright">© 2026 Hydra API Gateway</p>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { type FormInst, NAlert, NButton, NCard, NCheckbox, NForm, NFormItem, NIcon, NInput, NSpace } from 'naive-ui'
import { LockClosedOutline, PersonOutline } from '@vicons/ionicons5'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const authStore = useAuthStore()

const formRef = ref<FormInst | null>(null)

const formData = reactive({
  username: '',
  password: '',
})
const rememberMe = ref(false)

const rules = {
  username: {
    required: true,
    message: '请输入用户名',
    trigger: ['blur', 'input'],
  },
  password: {
    required: true,
    message: '请输入密码',
    trigger: ['blur', 'input'],
  },
}

async function handleLogin() {
  if (!formRef.value) return

  try {
    await formRef.value.validate()
    const result = await authStore.login({
      username: formData.username,
      password: formData.password,
      rememberMe: rememberMe.value,
    })

    if (result.success) {
      const username = formData.username.trim()
      if (rememberMe.value && username) {
        localStorage.setItem('last_username', username)
      } else {
        localStorage.removeItem('last_username')
      }
      router.push('/')
    }
  } catch {
    // 表单校验错误由 Naive UI 自行提示
  }
}

onMounted(() => {
  const lastUsername = localStorage.getItem('last_username') || ''
  if (lastUsername) {
    formData.username = lastUsername
    rememberMe.value = true
  }
})
</script>

<style scoped>
.login-page {
  height: 100vh;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  padding: 28px;
  background:
    radial-gradient(circle at 10% 10%, rgba(17, 17, 17, 0.1), transparent 45%),
    radial-gradient(circle at 90% 90%, rgba(82, 82, 82, 0.08), transparent 40%),
    #f5f5f5;
}

.login-page__panel {
  width: min(420px, 100%);
}

.login-page__form-wrap {
  display: flex;
  align-items: center;
  justify-content: center;
}

.login-card {
  width: 100%;
  border: 1px solid #e2e8f0;
  border-radius: 16px;
  box-shadow: 0 18px 30px rgba(15, 23, 42, 0.08);
}

.login-card__header {
  display: flex;
  align-items: center;
  gap: 12px;
}

.login-brand__logo {
  width: 40px;
  height: 40px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #111111;
  color: #fff;
  box-shadow: 0 8px 20px rgba(0, 0, 0, 0.2);
  flex-shrink: 0;
}

.login-card__title {
  margin: 0;
  font-size: 18px;
  line-height: 1.25;
  color: #111111;
  font-weight: 700;
}

.login-card__desc {
  margin: 4px 0 0;
  font-size: 12px;
  color: #737373;
}

.login-page__copyright {
  margin: 18px 0 0;
  font-size: 12px;
  color: #8a8a8a;
}

@media (max-width: 960px) {
  .login-page {
    padding: 18px;
    justify-content: flex-start;
  }

  .login-page__panel {
    margin-top: 48px;
  }
}
</style>
