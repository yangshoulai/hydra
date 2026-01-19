<template>
  <div class="space-y-4 settings-page">
    <!-- 页面头部 -->
    <n-card :bordered="false" class="page-header-card">
      <n-space justify="space-between" align="center">
        <n-space vertical :size="4">
          <n-text class="page-title">系统设置</n-text>
          <n-text depth="3" class="page-subtitle">
            配置系统运行参数和行为策略
          </n-text>
        </n-space>
      </n-space>
    </n-card>

    <div class="settings-content">
      <!-- 熔断器设置 -->
      <n-card class="settings-card" :bordered="true">
        <template #header>
          <div class="card-header">
            <div class="card-title">
              <svg class="card-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor">
                <path d="M13 2L3 14h8l-1 8 10-12h-8l1-8z" stroke-width="2" stroke-linecap="round"
                      stroke-linejoin="round"/>
              </svg>
              <span>熔断器设置</span>
            </div>
            <n-tag size="small" :bordered="false" type="info">Circuit Breaker</n-tag>
          </div>
        </template>
        <n-text class="card-description">保护系统免受级联故障影响，自动隔离异常服务</n-text>
        <n-form :model="formData" label-placement="left" label-width="120px" class="settings-form">
          <div class="form-item-wrapper">
            <n-form-item label="失败阈值" path="circuit_breaker_failure_threshold">
              <n-input-number
                  v-model:value="formData.circuit_breaker_failure_threshold"
                  :min="1"
                  :max="100"
                  style="width: 100%"
              >
                <template #suffix>次</template>
              </n-input-number>
            </n-form-item>
            <n-text class="field-hint">连续失败多少次后触发熔断</n-text>
          </div>

          <div class="form-item-wrapper">
            <n-form-item label="冷却时长" path="circuit_breaker_cooling_duration">
              <n-input-number
                  v-model:value="formData.circuit_breaker_cooling_duration"
                  :min="10"
                  :max="3600"
                  style="width: 100%"
              >
                <template #suffix>秒</template>
              </n-input-number>
            </n-form-item>
            <n-text class="field-hint">熔断后等待多久再尝试恢复</n-text>
          </div>

          <div class="form-item-wrapper">
            <n-form-item label="探测间隔" path="circuit_breaker_probe_interval">
              <n-input-number
                  v-model:value="formData.circuit_breaker_probe_interval"
                  :min="10"
                  :max="300"
                  style="width: 100%"
              >
                <template #suffix>秒</template>
              </n-input-number>
            </n-form-item>
            <n-text class="field-hint">探测半开状态 Key 的时间间隔</n-text>
          </div>

          <div class="form-item-wrapper">
            <n-form-item label="最大并发探测数" path="circuit_breaker_probe_max_concurrent">
              <n-input-number
                  v-model:value="formData.circuit_breaker_probe_max_concurrent"
                  :min="1"
                  :max="50"
                  style="width: 100%"
              >
                <template #suffix>个</template>
              </n-input-number>
            </n-form-item>
            <n-text class="field-hint">每次探测周期最多同时探测的 Key 数量</n-text>
          </div>
        </n-form>
      </n-card>

      <!-- 代理设置 -->
      <n-card class="settings-card" :bordered="true">
        <template #header>
          <div class="card-header">
            <div class="card-title">
              <svg class="card-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor">
                <circle cx="12" cy="12" r="10" stroke-width="2"/>
                <path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"
                      stroke-width="2"/>
                <path d="M2 12h20" stroke-width="2"/>
              </svg>
              <span>代理设置</span>
            </div>
            <n-tag size="small" :bordered="false" type="success">Proxy</n-tag>
          </div>
        </template>
        <n-text class="card-description">控制请求代理行为和性能参数</n-text>

        <n-form label-placement="left" label-width="120px" class="settings-form">

          <div class="form-item-wrapper">
            <n-form-item label="请求超时" path="proxy_request_timeout">
              <n-input-number
                  v-model:value="formData.proxy_request_timeout"
                  :min="10"
                  :max="300"
                  style="width: 100%"
              >
                <template #suffix>秒</template>
              </n-input-number>
            </n-form-item>
            <n-text class="field-hint">上游请求的超时时间</n-text>
          </div>
          <div class="form-item-wrapper">
            <n-form-item label="最大并发数" path="proxy_max_concurrent">
              <n-input-number
                  v-model:value="formData.proxy_max_concurrent"
                  :min="10"
                  :max="10000"
                  style="width: 100%"
              >
                <template #suffix>个</template>
              </n-input-number>
            </n-form-item>
            <n-text class="field-hint">系统允许的最大并发请求数</n-text>
          </div>
          <div class="form-item-wrapper">
            <n-form-item label="最大响应大小" path="proxy_max_response_size">
              <n-input-number
                  v-model:value="formData.proxy_max_response_size"
                  :min="1"
                  :max="100"
                  :precision="0"
                  style="width: 100%"
              >
                <template #suffix>MB</template>
              </n-input-number>
            </n-form-item>
            <n-text class="field-hint">单个响应的最大 Body 大小</n-text>
          </div>

          <div class="form-item-wrapper">
            <n-form-item label="最大重试次数" path="proxy_max_retry">
              <n-input-number
                  v-model:value="formData.proxy_max_retry"
                  :min="0"
                  :max="10"
                  style="width: 100%"
              >
                <template #suffix>次</template>
              </n-input-number>
            </n-form-item>
            <n-text class="field-hint">单个请求失败后的最大重试次数</n-text>
          </div>
        </n-form>
      </n-card>

      <!-- 响应嗅探设置 -->
      <n-card class="settings-card" :bordered="true">
        <template #header>
          <div class="card-header">
            <div class="card-title">
              <svg class="card-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor">
                <path d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" stroke-width="2" stroke-linecap="round"
                      stroke-linejoin="round"/>
              </svg>
              <span>响应嗅探设置</span>
            </div>
            <n-tag size="small" :bordered="false" type="error">Sniffer</n-tag>
          </div>
        </template>
        <n-text class="card-description">配置明文错误关键词，用于识别假 200 响应</n-text>

        <n-form label-placement="left" label-width="120px" class="settings-form">
          <div class="form-item-wrapper">
            <n-form-item label="错误关键词" path="sniffer_plain_text_error_keywords">
              <n-input
                  v-model:value="snifferKeywords"
                  type="textarea"
                  placeholder="每行一个关键词，不区分大小写"
                  :autosize="{ minRows: 10, maxRows: 20 }"
                  style="width: 100%"
              />
            </n-form-item>
            <n-text class="field-hint">响应 Body 包含这些关键词时将被识别为错误响应，每行一个关键词</n-text>
          </div>
        </n-form>
      </n-card>

      <!-- 日志设置 -->
      <n-card class="settings-card" :bordered="true">
        <template #header>
          <div class="card-header">
            <div class="card-title">
              <svg class="card-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor">
                <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" stroke-width="2"/>
                <polyline points="14 2 14 8 20 8" stroke-width="2"/>
                <line x1="16" y1="13" x2="8" y2="13" stroke-width="2"/>
                <line x1="16" y1="17" x2="8" y2="17" stroke-width="2"/>
                <polyline points="10 9 9 9 8 9" stroke-width="2"/>
              </svg>
              <span>日志设置</span>
            </div>
            <n-tag size="small" :bordered="false" type="warning">Logging</n-tag>
          </div>
        </template>
        <n-text class="card-description">控制日志记录行为和存储策略</n-text>

        <n-form label-placement="left" label-width="120px" class="settings-form">

          <div class="form-item-wrapper">
            <n-form-item label="日志保留天数" path="log_retention_days">
              <n-input-number
                  v-model:value="formData.log_retention_days"
                  :min="1"
                  :max="365"
                  style="width: 100%"
              >
                <template #suffix>天</template>
              </n-input-number>
            </n-form-item>
            <n-text class="field-hint">审计日志保留天数</n-text>
          </div>

          <div class="form-item-wrapper">
            <n-form-item label="调试模式" path="debug_mode">
              <div class="switch-wrapper">
                <n-switch v-model:value="debugModeEnabled"/>
                <n-tag v-if="debugModeEnabled" size="small" type="warning" :bordered="false">
                  已启用
                </n-tag>
                <n-tag v-else size="small" type="default" :bordered="false">
                  已关闭
                </n-tag>
              </div>
            </n-form-item>
            <n-text class="field-hint">记录完整请求/响应，仅用于排查问题</n-text>
          </div>
        </n-form>
      </n-card>

      <!-- 操作按钮 -->
      <div class="action-bar">
        <n-space>
          <n-button size="large" @click="handleReset" :disabled="isSaving">
            <template #icon>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" style="width: 16px; height: 16px;">
                <path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8" stroke-width="2"/>
                <path d="M21 3v5h-5" stroke-width="2"/>
                <path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16" stroke-width="2"/>
                <path d="M3 21v-5h5" stroke-width="2"/>
              </svg>
            </template>
            重置为默认值
          </n-button>
          <n-button type="primary" size="large" :loading="isSaving" @click="handleSave">
            <template #icon>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" style="width: 16px; height: 16px;">
                <path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z" stroke-width="2"/>
                <polyline points="17 21 17 13 7 13 7 21" stroke-width="2"/>
                <polyline points="7 3 7 8 15 8" stroke-width="2"/>
              </svg>
            </template>
            保存设置
          </n-button>
        </n-space>
      </div>
    </div>
  </div>

  <!-- 调试模式确认对话框 -->
  <n-modal v-model:show="showDebugModeConfirm" preset="dialog" title="启用调试模式">
    <template #icon>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" style="width: 24px; height: 24px; color: #f0a020;">
        <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"
              stroke-width="2"/>
        <line x1="12" y1="9" x2="12" y2="13" stroke-width="2"/>
        <line x1="12" y1="17" x2="12.01" y2="17" stroke-width="2"/>
      </svg>
    </template>
    <n-space vertical :size="16">
      <n-alert type="warning" :show-icon="false">
        <n-text>您即将启用调试模式，请注意以下事项：</n-text>
      </n-alert>
      <n-ul style="margin: 0; padding-left: 24px;">
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
import {onMounted, ref, watch} from 'vue'
import {NAlert, NButton, NCard, NForm, NFormItem, NInput, NInputNumber, NLi, NModal, NSpace, NSwitch, NTag, NText, NUl, useDialog, useMessage,} from 'naive-ui'
import settingsService from '@/services/settingsService'

const dialog = useDialog()
const message = useMessage()

interface SettingsData {
  circuit_breaker_failure_threshold: number
  circuit_breaker_cooling_duration: number
  circuit_breaker_probe_interval: number
  circuit_breaker_probe_max_concurrent: number
  proxy_request_timeout: number
  proxy_max_concurrent: number
  proxy_max_response_size: number
  proxy_max_retry: number
  log_retention_days: number
  log_debug_enabled: boolean
}

const isLoading = ref(false)
const isSaving = ref(false)
const debugModeEnabled = ref(false)
const originalDebugModeValue = ref(false) // 用于保存原始值
const showDebugModeConfirm = ref(false)
const isConfirming = ref(false) // 标志位：是否正在确认中，防止 watch 循环
const snifferKeywords = ref('') // 嗅探器错误关键词（每行一个）

const formData = ref<SettingsData>({
  circuit_breaker_failure_threshold: 3,
  circuit_breaker_cooling_duration: 60,
  circuit_breaker_probe_interval: 30,
  circuit_breaker_probe_max_concurrent: 10,
  proxy_request_timeout: 60,
  proxy_max_concurrent: 1000,
  proxy_max_response_size: 10,
  proxy_max_retry: 3,
  log_retention_days: 30,
  log_debug_enabled: false
})

// 加载系统设置
const loadSettings = async () => {
  isLoading.value = true
  try {
    const settings = await settingsService.getAllSettings()

    // 设置标志位，防止加载时触发 watch
    isConfirming.value = true

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
    if (settings.circuit_breaker_probe_interval) {
      formData.value.circuit_breaker_probe_interval = parseInt(
          settings.circuit_breaker_probe_interval
      )
    }
    if (settings.circuit_breaker_probe_max_concurrent) {
      formData.value.circuit_breaker_probe_max_concurrent = parseInt(
          settings.circuit_breaker_probe_max_concurrent
      )
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
    if (settings.proxy_max_retry) {
      formData.value.proxy_max_retry = parseInt(settings.proxy_max_retry)
    }
    if (settings.log_retention_days) {
      formData.value.log_retention_days = parseInt(
          settings.log_retention_days
      )
    }
    if (settings.log_debug_enabled !== undefined) {
      const debugMode = settings.log_debug_enabled === 'true'
      debugModeEnabled.value = debugMode
      originalDebugModeValue.value = debugMode // 保存原始值
    }

    // 加载嗅探器关键词（从通用设置中获取）
    if (settings.sniffer_plain_text_error_rules) {
      try {
        const keywords = JSON.parse(settings.sniffer_plain_text_error_rules)
        if (keywords && keywords.length > 0) {
          snifferKeywords.value = keywords.join('\n')
        }
      } catch (error) {
        console.error('Failed to parse sniffer keywords:', error)
      }
    }
    // 重置标志位
    setTimeout(() => {
      isConfirming.value = false
    }, 0)
  } catch (error) {
    console.error('Failed to load settings:', error)
    message.error('无法加载系统设置，请稍后重试')
    isConfirming.value = false // 出错时也要重置
  } finally {
    isLoading.value = false
  }
}

// 监听调试模式切换 - 只在从 false 变为 true 时弹窗确认
watch(debugModeEnabled, (newValue, oldValue) => {
  // 如果正在确认中，跳过（防止循环）
  if (isConfirming.value) {
    return
  }

  // 只在用户手动打开时弹窗（从 false 变为 true）
  if (newValue && !oldValue) {
    // 设置确认标志，阻止状态改变
    isConfirming.value = true
    debugModeEnabled.value = false
    // 显示确认对话框
    showDebugModeConfirm.value = true
    // 下一个 tick 后重置标志
    setTimeout(() => {
      isConfirming.value = false
    }, 0)
  }
})

// 确认启用调试模式
const confirmDebugMode = () => {
  isConfirming.value = true // 设置标志，防止触发 watch
  debugModeEnabled.value = true // 确认后真正设置为 true
  showDebugModeConfirm.value = false
  // 重置标志
  setTimeout(() => {
    isConfirming.value = false
  }, 0)
}

// 取消调试模式
const cancelDebugMode = () => {
  // 保持为 false，不需要额外操作
  showDebugModeConfirm.value = false
}

// 保存设置
const handleSave = async () => {
  isSaving.value = true
  try {
    // 准备嗅探器关键词
    const keywords = snifferKeywords.value
        .split('\n')
        .map(k => k.trim())
        .filter(k => k.length > 0)

    // 保存所有设置（包括嗅探器关键词）
    await settingsService.updateSettings({
      settings: {
        circuit_breaker_failure_threshold:
            formData.value.circuit_breaker_failure_threshold.toString(),
        circuit_breaker_cooling_duration:
            formData.value.circuit_breaker_cooling_duration.toString(),
        circuit_breaker_probe_interval:
            formData.value.circuit_breaker_probe_interval.toString(),
        circuit_breaker_probe_max_concurrent:
            formData.value.circuit_breaker_probe_max_concurrent.toString(),
        proxy_request_timeout:
            formData.value.proxy_request_timeout.toString(),
        proxy_max_concurrent: formData.value.proxy_max_concurrent.toString(),
        proxy_max_response_size: (
            formData.value.proxy_max_response_size * 1024 * 1024
        ).toString(), // 转换为字节
        proxy_max_retry: formData.value.proxy_max_retry.toString(),
        log_retention_days: formData.value.log_retention_days.toString(),
        log_debug_enabled: debugModeEnabled.value.toString(),
        sniffer_plain_text_error_rules: JSON.stringify(keywords), // 嗅探器关键词以JSON格式保存
      },
    })

    // 保存成功后更新原始值，避免重复弹窗
    originalDebugModeValue.value = debugModeEnabled.value

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
/* 页面样式 */
.settings-page {
  --primary-color: #18a058;
  --primary-color-hover: #36ad6a;
  --info-color: #2080f0;
  --warning-color: #f0a020;
  --success-color: #18a058;
  --error-color: #d03050;
  animation: fadeIn 0.3s ease-in;
}

@keyframes fadeIn {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* 页面头部卡片 */
.page-header-card {
  background: linear-gradient(135deg, #f5f7fa 0%, #ffffff 100%);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
  border-radius: 12px;
  padding: 24px;
}

.page-title {
  font-size: 24px;
  font-weight: 700;
  color: #333;
  line-height: 1.4;
}

.page-subtitle {
  font-size: 14px;
  line-height: 1.6;
}

.settings-content {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.settings-card {
  border-radius: 12px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.08);
  transition: all 0.3s ease;
  background: #ffffff;
}

.settings-card:hover {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.12);
  transform: translateY(-2px);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

.card-title {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 18px;
  font-weight: 600;
  color: #18181b;
}

.card-icon {
  width: 20px;
  height: 20px;
  color: #6366f1;
}

.card-description {
  display: block;
  margin: -8px 0 24px 0;
  font-size: 13px;
  color: #71717a;
  line-height: 1.6;
}

.settings-form {
  margin-top: 16px;
  max-width: 480px;
}

.form-item-wrapper {
  margin-bottom: 24px;
}

:deep(.form-item-wrapper .n-form-item-feedback-wrapper) {
  min-height: unset;
}

.field-hint {
  display: block;
  margin-top: 6px;
  font-size: 12px;
  color: #a1a1aa;
  line-height: 1.5;
  text-align: left;
  padding-left: 120px;
}

.switch-wrapper {
  display: flex;
  align-items: center;
  gap: 12px;
}

.action-bar {
  display: flex;
  justify-content: flex-end;
}

</style>
