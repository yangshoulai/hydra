<template>
  <div class="space-y-6 animate-fade-in">
    <!-- 熔断器设置 -->
    <n-card title="熔断器设置" size="small">
      <template #header-extra>
        <n-text depth="3" style="font-size: 12px; font-weight: normal;">
          保护系统免受级联故障影响
        </n-text>
      </template>
      <n-form ref="circuitFormRef" :model="formData" label-placement="left" label-width="180px">
        <n-grid :cols="2" :x-gap="24" responsive="screen">
          <n-grid-item span="2">
            <n-form-item label="失败阈值" path="circuit_breaker_failure_threshold">
              <n-input-number
                v-model:value="formData.circuit_breaker_failure_threshold"
                :min="1"
                :max="100"
                style="width: 100%"
              />
            </n-form-item>
            <n-text depth="3" style="font-size: 12px; margin-top: -16px; margin-bottom: 16px; display: block; text-align: right;">
              连续失败多少次后触发熔断，范围 1-100
            </n-text>
          </n-grid-item>

          <n-grid-item>
            <n-form-item label="冷却时长（秒）" path="circuit_breaker_cooling_duration">
              <n-input-number
                v-model:value="formData.circuit_breaker_cooling_duration"
                :min="10"
                :max="3600"
                style="width: 100%"
              />
            </n-form-item>
            <n-text depth="3" style="font-size: 12px; margin-top: -16px; margin-bottom: 16px; display: block; text-align: right;">
              熔断后等待多久再尝试恢复，范围 10-3600 秒
            </n-text>
          </n-grid-item>

          <n-grid-item>
            <n-form-item label="最大重试次数" path="proxy_max_retry">
              <n-input-number
                v-model:value="formData.proxy_max_retry"
                :min="0"
                :max="10"
                style="width: 100%"
              />
            </n-form-item>
            <n-text depth="3" style="font-size: 12px; margin-top: -16px; margin-bottom: 16px; display: block; text-align: right;">
              单个请求失败后的最大重试次数，范围 0-10
            </n-text>
          </n-grid-item>
        </n-grid>
      </n-form>
    </n-card>

    <!-- 代理设置 -->
    <n-card title="代理设置" size="small">
      <template #header-extra>
        <n-text depth="3" style="font-size: 12px; font-weight: normal;">
          控制请求代理行为
        </n-text>
      </template>
      <n-form label-placement="left" label-width="180px">
        <n-grid :cols="2" :x-gap="24" responsive="screen">
          <n-grid-item>
            <n-form-item label="请求超时（秒）" path="proxy_request_timeout">
              <n-input-number
                v-model:value="formData.proxy_request_timeout"
                :min="10"
                :max="300"
                style="width: 100%"
              />
            </n-form-item>
            <n-text depth="3" style="font-size: 12px; margin-top: -16px; margin-bottom: 16px; display: block; text-align: right;">
              上游请求的超时时间，范围 10-300 秒
            </n-text>
          </n-grid-item>

          <n-grid-item>
            <n-form-item label="最大并发数" path="proxy_max_concurrent">
              <n-input-number
                v-model:value="formData.proxy_max_concurrent"
                :min="10"
                :max="10000"
                style="width: 100%"
              />
            </n-form-item>
            <n-text depth="3" style="font-size: 12px; margin-top: -16px; margin-bottom: 16px; display: block; text-align: right;">
              系统允许的最大并发请求数，范围 10-10000
            </n-text>
          </n-grid-item>

          <n-grid-item span="2">
            <n-form-item label="最大响应大小（MB）" path="proxy_max_response_size">
              <n-input-number
                v-model:value="formData.proxy_max_response_size"
                :min="1"
                :max="100"
                :precision="0"
                style="width: 100%"
              />
            </n-form-item>
            <n-text depth="3" style="font-size: 12px; margin-top: -16px; margin-bottom: 16px; display: block; text-align: right;">
              单个响应的最大 Body 大小，超过将拒绝，范围 1-100 MB
            </n-text>
          </n-grid-item>
        </n-grid>
      </n-form>
    </n-card>

    <!-- 日志设置 -->
    <n-card title="日志设置" size="small">
      <template #header-extra>
        <n-text depth="3" style="font-size: 12px; font-weight: normal;">
          控制日志记录行为
        </n-text>
      </template>
      <n-form label-placement="left" label-width="180px">
        <n-grid :cols="2" :x-gap="24" responsive="screen">
          <n-grid-item>
            <n-form-item label="日志保留天数" path="log_retention_days">
              <n-input-number
                v-model:value="formData.log_retention_days"
                :min="1"
                :max="365"
                style="width: 100%"
              />
            </n-form-item>
            <n-text depth="3" style="font-size: 12px; margin-top: -16px; margin-bottom: 16px; display: block; text-align: right;">
              审计日志保留天数，范围 1-365 天
            </n-text>
          </n-grid-item>

          <n-grid-item>
            <n-form-item label="调试模式" path="debug_mode">
              <n-switch
                v-model:value="debugModeEnabled"
              />
            </n-form-item>
            <n-text depth="3" style="font-size: 12px; margin-top: -16px; margin-bottom: 16px; display: block; text-align: right;">
              启用后将记录完整的请求/响应 Body，仅用于问题排查
            </n-text>
          </n-grid-item>
        </n-grid>
      </n-form>
    </n-card>

    <!-- 操作按钮 -->
    <n-card size="small" :bordered="false">
      <n-space justify="end">
        <n-button @click="handleReset">
          重置为默认值
        </n-button>
        <n-button type="primary" :loading="isSaving" @click="handleSave">
          保存设置
        </n-button>
      </n-space>
    </n-card>
  </div>

  <!-- 调试模式确认对话框 -->
  <n-modal v-model:show="showDebugModeConfirm" preset="dialog" title="启用调试模式" :negative-text="null" positive-text="我知道风险">
    <n-space vertical :size="12">
      <n-alert type="warning" :show-icon="false">
        <n-text>您即将启用调试模式，请注意以下事项：</n-text>
      </n-alert>
      <n-ul>
        <n-li>完整的请求/响应 Body 将被记录到日志文件</n-li>
        <n-li>可能会产生大量日志文件，占用磁盘空间</n-li>
        <n-li>可能包含敏感信息，请勿在生产环境长期启用</n-li>
        <n-li>建议仅在排查问题时临时启用</n-li>
      </n-ul>
    </n-space>
    <template #action>
      <n-space>
        <n-button @click="cancelDebugMode">取消</n-button>
        <n-button type="warning" @click="confirmDebugMode">确定启用</n-button>
      </n-space>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import {
  NCard,
  NForm,
  NFormItem,
  NGrid,
  NGridItem,
  NInputNumber,
  NSwitch,
  NButton,
  NSpace,
  NText,
  NModal,
  NAlert,
  NUl,
  useDialog,
  useMessage,
} from 'naive-ui'
import settingsService from '@/services/settingsService'

const dialog = useDialog()
const message = useMessage()

interface SettingsData {
  circuit_breaker_failure_threshold: number
  circuit_breaker_cooling_duration: number
  proxy_max_retry: number
  proxy_request_timeout: number
  proxy_max_concurrent: number
  proxy_max_response_size: number
  log_retention_days: number
}

const circuitFormRef = ref()
const isLoading = ref(false)
const isSaving = ref(false)
const debugModeEnabled = ref(false)
const pendingDebugModeValue = ref(false)
const showDebugModeConfirm = ref(false)

const formData = ref<SettingsData>({
  circuit_breaker_failure_threshold: 3,
  circuit_breaker_cooling_duration: 60,
  proxy_max_retry: 3,
  proxy_request_timeout: 60,
  proxy_max_concurrent: 1000,
  proxy_max_response_size: 10,
  log_retention_days: 30,
})

// 加载系统设置
const loadSettings = async () => {
  isLoading.value = true
  try {
    const settings = await settingsService.getAllSettings()

    // 更新表单数据
    if (settings.circuit_breaker_failure_threshold) {
      formData.value.circuit_breaker_failure_threshold = parseInt(
        settings.circuit_breaker_failure_threshold
      )
    }
    if (settings.circuit_breaker_cooling_duration) {
      formData.value.circuit_breaker_cooling_duration = parseInt(
        settings.circuit_breaker_cooling_duration
      )
    }
    if (settings.proxy_max_retry) {
      formData.value.proxy_max_retry = parseInt(settings.proxy_max_retry)
    }
    if (settings.proxy_request_timeout) {
      formData.value.proxy_request_timeout = parseInt(
        settings.proxy_request_timeout
      )
    }
    if (settings.proxy_max_concurrent) {
      formData.value.proxy_max_concurrent = parseInt(
        settings.proxy_max_concurrent
      )
    }
    if (settings.proxy_max_response_size) {
      formData.value.proxy_max_response_size = parseInt(
        settings.proxy_max_response_size
      ) / (1024 * 1024) // 转换为 MB
    }
    if (settings.log_retention_days) {
      formData.value.log_retention_days = parseInt(
        settings.log_retention_days
      )
    }
    if (settings.debug_mode !== undefined) {
      debugModeEnabled.value = settings.debug_mode === 'true'
    }
  } catch (error) {
    console.error('Failed to load settings:', error)
    message.error('无法加载系统设置，请稍后重试')
  } finally {
    isLoading.value = false
  }
}

// 监听调试模式切换
watch(debugModeEnabled, (newValue) => {
  if (newValue) {
    pendingDebugModeValue.value = true
    showDebugModeConfirm.value = true
  }
})

// 确认启用调试模式
const confirmDebugMode = () => {
  showDebugModeConfirm.value = false
}

// 取消调试模式
const cancelDebugMode = () => {
  debugModeEnabled.value = false
  showDebugModeConfirm.value = false
}

// 保存设置
const handleSave = async () => {
  isSaving.value = true
  try {
    await settingsService.updateSettings({
      settings: {
        circuit_breaker_failure_threshold:
          formData.value.circuit_breaker_failure_threshold.toString(),
        circuit_breaker_cooling_duration:
          formData.value.circuit_breaker_cooling_duration.toString(),
        proxy_max_retry: formData.value.proxy_max_retry.toString(),
        proxy_request_timeout:
          formData.value.proxy_request_timeout.toString(),
        proxy_max_concurrent: formData.value.proxy_max_concurrent.toString(),
        proxy_max_response_size: (
          formData.value.proxy_max_response_size * 1024 * 1024
        ).toString(), // 转换为字节
        log_retention_days: formData.value.log_retention_days.toString(),
        debug_mode: debugModeEnabled.value.toString(),
      },
    })

    message.success('系统设置已更新，配置已立即生效')
  } catch (error) {
    console.error('Failed to save settings:', error)
    message.error('无法保存系统设置，请稍后重试')
  } finally {
    isSaving.value = false
  }
}

// 重置设置
const handleReset = () => {
  dialog.warning({
    title: '重置确认',
    content: '确定要重置所有设置为默认值吗？',
    positiveText: '确定',
    negativeText: '取消',
    onPositiveClick: async () => {
      await loadSettings()
      message.info('设置已重置为默认值')
    },
  })
}

onMounted(() => {
  loadSettings()
})
</script>

<style scoped>
/* 使用 Tailwind CSS，无需自定义样式 */
</style>
