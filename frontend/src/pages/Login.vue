<template>
  <div class="w-full h-screen flex items-center justify-center relative overflow-hidden">
    <!-- 动态背景 -->
    <div class="absolute inset-0 bg-gradient-to-br from-primary-500 via-primary-600 to-secondary-700 z-0">
      <div class="absolute inset-0 overflow-hidden">
        <div class="shape shape-1"></div>
        <div class="shape shape-2"></div>
        <div class="shape shape-3"></div>
      </div>
    </div>

    <!-- 内容区域 -->
    <div class="relative z-10 flex flex-col items-center">
      <!-- 登录卡片 -->
      <div class="w-[420px] bg-white/98 backdrop-blur-xl rounded-3xl p-12 shadow-2xl shadow-black/30 animate-slide-up">
        <!-- 头部 -->
        <div class="text-center mb-10">
          <div
              class="inline-flex items-center justify-center w-16 h-16 bg-gradient-to-br from-primary-500 to-secondary-600 rounded-2xl mb-5 shadow-lg shadow-primary-500/40">
            <svg class="w-8 h-8 text-white" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M12 2L2 7L12 12L22 7L12 2Z" stroke="currentColor" stroke-width="2" stroke-linecap="round"
                    stroke-linejoin="round"/>
              <path d="M2 17L12 22L22 17" stroke="currentColor" stroke-width="2" stroke-linecap="round"
                    stroke-linejoin="round"/>
              <path d="M2 12L12 17L22 12" stroke="currentColor" stroke-width="2" stroke-linecap="round"
                    stroke-linejoin="round"/>
            </svg>
          </div>
          <h1 class="text-3xl font-bold text-gray-900 mb-2 tracking-tight">Hydra</h1>
          <p class="text-sm text-gray-600">API Gateway</p>
        </div>

        <!-- 登录表单 -->
        <n-form
            ref="formRef"
            :model="formData"
            :rules="rules"
            label-placement="left"
            label-width="0"
            require-mark-placement="right-hanging"
            class="space-y-5 mb-6"
        >
          <n-form-item path="username">
            <n-input
                v-model:value="formData.username"
                placeholder="用户名"
                size="large"
                @keydown.enter="handleLogin"
            >
              <template #prefix>
                <n-icon>
                  <PersonOutline/>
                </n-icon>
              </template>
            </n-input>
          </n-form-item>

          <n-form-item path="password">
            <n-input
                v-model:value="formData.password"
                type="password"
                show-password-on="click"
                placeholder="密码"
                size="large"
                @keydown.enter="handleLogin"
            >
              <template #prefix>
                <n-icon>
                  <LockClosedOutline/>
                </n-icon>
              </template>
            </n-input>
          </n-form-item>

          <n-form-item :show-label="false">
            <n-button
                type="primary"
                block
                size="large"
                :loading="authStore.loading"
                @click="handleLogin"
                class="!h-12 !text-base !font-semibold !rounded-xl !mt-2"
            >
              登录
            </n-button>
          </n-form-item>
        </n-form>

        <!-- 错误提示 -->
        <n-alert v-if="authStore.error" type="error" :show-icon="false" class="!rounded-xl">
          {{ authStore.error }}
        </n-alert>
      </div>

      <!-- 页脚 -->
      <div class="mt-8 text-center">
        <p class="text-sm text-white/80">© 2026 Hydra API Gateway. All rights reserved.</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import {reactive, ref} from 'vue'
import {useRouter} from 'vue-router'
import {type FormInst, NAlert, NButton, NForm, NFormItem, NIcon, NInput} from 'naive-ui'
import {LockClosedOutline, PersonOutline} from '@vicons/ionicons5'
import {useAuthStore} from '../stores/auth'

const router = useRouter()
const authStore = useAuthStore()

// 表单引用
const formRef = ref<FormInst | null>(null)

// 表单数据
const formData = reactive({
  username: '',
  password: ''
})

// 表单验证规则
const rules = {
  username: {
    required: true,
    message: '请输入用户名',
    trigger: ['blur', 'input']
  },
  password: {
    required: true,
    message: '请输入密码',
    trigger: ['blur', 'input']
  }
}

// 登录处理
async function handleLogin() {
  if (!formRef.value) return

  try {
    await formRef.value.validate()

    const result = await authStore.login({
      username: formData.username,
      password: formData.password
    })

    if (result.success) {
      // 登录成功，跳转到首页
      router.push('/')
    }
  } catch (error) {
    console.error('Login validation failed:', error)
  }
}
</script>

<style scoped>
/* 浮动背景形状 */
.shape {
  position: absolute;
  background: rgba(255, 255, 255, 0.1);
  border-radius: 9999px;
  animation: spin 3s linear infinite;
}

.shape-1 {
  width: 500px;
  height: 500px;
  top: -15rem;
  right: -10rem;
  animation: float 20s infinite ease-in-out;
}

.shape-2 {
  width: 24rem;
  height: 24rem;
  bottom: -12rem;
  left: -6rem;
  animation: float 20s 5s infinite ease-in-out;
}

.shape-3 {
  width: 18rem;
  height: 18rem;
  top: 50%;
  left: 50%;
  animation: float 20s 10s infinite ease-in-out;
}

@keyframes float {
  0%, 100% {
    transform: translate(0, 0) rotate(0deg);
  }
  33% {
    transform: translate(30px, -30px) rotate(120deg);
  }
  66% {
    transform: translate(-20px, 20px) rotate(240deg);
  }
}

/* 表单样式定制 */
:deep(.n-form-item) {
  margin-bottom: 1.25rem;
}

:deep(.n-form-item:last-child) {
  margin-bottom: 0;
}

:deep(.n-input) {
  border-radius: 0.75rem;
}

:deep(.n-input__input-el) {
  font-size: 15px;
}
</style>
