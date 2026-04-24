<template>
  <div class="app-page settings-page">
    <n-alert v-if="loadError" type="error" :bordered="false">
      <n-space align="center" justify="space-between" style="width: 100%">
        <span>{{ loadError }}</span>
        <n-button text type="error" @click="loadSettings">重试</n-button>
      </n-space>
    </n-alert>

    <section v-if="isLoading" class="panel-card">
      <div class="panel-card__body">
        <n-skeleton text :repeat="8" />
      </div>
    </section>

    <template v-else>
      <section class="panel-card">
        <header class="panel-card__header">
          <h3 class="panel-card__title">服务与网络</h3>
        </header>
        <div class="panel-card__body panel-card__body--flush">
          <div class="setting-row">
            <div class="setting-row__info">
              <div class="setting-row__label">
                服务端口
                <n-tag size="tiny" type="warning" :bordered="false">保存后自动重启</n-tag>
              </div>
              <div class="setting-row__desc">保存后服务将自动重启，存在约 1 秒不可用。</div>
            </div>
            <div class="setting-row__control">
              <n-input-number v-model:value="formData.server_port" :min="1" :max="65535" style="width: 100%" placeholder="1-65535" />
            </div>
          </div>

          <div class="setting-row">
            <div class="setting-row__info">
              <div class="setting-row__label">
                读取超时（秒）
                <n-tag size="tiny" type="warning" :bordered="false">保存后自动重启</n-tag>
              </div>
              <div class="setting-row__desc">保存后服务将自动重启，存在约 1 秒不可用。</div>
            </div>
            <div class="setting-row__control">
              <n-input-number v-model:value="formData.server_read_timeout" :min="1" :max="600" style="width: 100%" placeholder="1-600" />
            </div>
          </div>

          <div class="setting-row">
            <div class="setting-row__info">
              <div class="setting-row__label">
                写出超时（秒）
                <n-tag size="tiny" type="warning" :bordered="false">保存后自动重启</n-tag>
              </div>
              <div class="setting-row__desc">保存后服务将自动重启，存在约 1 秒不可用。</div>
            </div>
            <div class="setting-row__control">
              <n-input-number v-model:value="formData.server_write_timeout" :min="0" :max="600" style="width: 100%" placeholder="0-600" />
            </div>
          </div>

          <div class="setting-row">
            <div class="setting-row__info">
              <div class="setting-row__label">代理请求超时（秒）</div>
              <div class="setting-row__desc">调用上游厂商接口的超时时间，超时后触发重试。</div>
            </div>
            <div class="setting-row__control">
              <n-input-number v-model:value="formData.proxy_request_timeout" :min="10" :max="300" style="width: 100%" placeholder="10-300" />
            </div>
          </div>

          <div class="setting-row">
            <div class="setting-row__info">
              <div class="setting-row__label">最大重试次数</div>
              <div class="setting-row__desc">单次请求失败后的最大重试次数，0 表示不重试。</div>
            </div>
            <div class="setting-row__control">
              <n-input-number v-model:value="formData.proxy_max_retry" :min="0" :max="10" style="width: 100%" placeholder="0-10" />
            </div>
          </div>

          <div class="setting-row">
            <div class="setting-row__info">
              <div class="setting-row__label">网络代理地址</div>
              <div class="setting-row__desc">系统级上游代理地址，支持 HTTP / HTTPS / SOCKS5。仅对渠道配置中启用了“系统代理”的渠道生效，留空则所有渠道直连。</div>
            </div>
            <div class="setting-row__control setting-row__control--wide">
              <n-input
                v-model:value="formData.proxy_network_url"
                clearable
                placeholder="例如：http://127.0.0.1:7890 或 socks5://127.0.0.1:1080；仅作用于已启用系统代理的渠道"
                style="width: 100%"
              />
            </div>
          </div>
        </div>
      </section>

      <section class="panel-card">
        <header class="panel-card__header">
          <h3 class="panel-card__title">稳定性与嗅探</h3>
        </header>
        <div class="panel-card__body panel-card__body--flush">
          <div class="setting-row">
            <div class="setting-row__info">
              <div class="setting-row__label">熔断失败阈值</div>
              <div class="setting-row__desc">密钥连续失败达到阈值后触发熔断，隔离故障密钥。</div>
            </div>
            <div class="setting-row__control">
              <n-input-number
                v-model:value="formData.circuit_breaker_failure_threshold"
                :min="1"
                :max="100"
                style="width: 100%"
                placeholder="1-100"
              />
            </div>
          </div>

          <div class="setting-row">
            <div class="setting-row__info">
              <div class="setting-row__label">熔断冷却时长（秒）</div>
              <div class="setting-row__desc">熔断触发后，冷却这段时间再尝试恢复。</div>
            </div>
            <div class="setting-row__control">
              <n-input-number
                v-model:value="formData.circuit_breaker_cooling_duration"
                :min="10"
                :max="3600"
                style="width: 100%"
                placeholder="10-3600"
              />
            </div>
          </div>

          <div class="setting-row">
            <div class="setting-row__info">
              <div class="setting-row__label">响应嗅探</div>
              <div class="setting-row__desc">解析上游响应正文，识别 HTTP 200 但业务失败的情况，计入熔断。</div>
            </div>
            <div class="setting-row__control">
              <n-space align="center">
                <n-switch v-model:value="formData.sniffer_enabled" />
                <n-tag :type="formData.sniffer_enabled ? 'success' : 'default'" :bordered="false" size="small">
                  {{ formData.sniffer_enabled ? '已启用' : '已关闭' }}
                </n-tag>
              </n-space>
            </div>
          </div>

          <div class="setting-row">
            <div class="setting-row__info">
              <div class="setting-row__label">流式探测包数</div>
              <div class="setting-row__desc">流式响应仅检查前 N 个数据包，避免扫描全量 body。</div>
            </div>
            <div class="setting-row__control">
              <n-input-number
                v-model:value="formData.sniffer_stream_packet_count"
                :min="1"
                :max="20"
                :disabled="!formData.sniffer_enabled"
                style="width: 100%"
                placeholder="1-20"
              />
            </div>
          </div>

          <div class="setting-row setting-row--block">
            <div class="setting-row__info">
              <div class="setting-row__label">错误关键词</div>
              <div class="setting-row__desc">
                响应文本中包含任一关键词即判定失败，可用于识别配额超限、鉴权失效等业务错误。每行一个。
              </div>
            </div>
            <div class="setting-row__control setting-row__control--block">
              <n-input
                v-model:value="snifferKeywords"
                type="textarea"
                :disabled="!formData.sniffer_enabled"
                :autosize="{ minRows: 4, maxRows: 10 }"
                placeholder="每行一个关键词，例如：&#10;rate limit&#10;insufficient quota&#10;invalid api key"
                style="width: 100%"
              />
            </div>
          </div>
        </div>
      </section>

      <section class="panel-card">
        <header class="panel-card__header">
          <h3 class="panel-card__title">日志与调试</h3>
        </header>
        <div class="panel-card__body panel-card__body--flush">
          <div class="setting-row">
            <div class="setting-row__info">
              <div class="setting-row__label">日志保留天数</div>
              <div class="setting-row__desc">访问日志在数据库中的保留天数，过期自动清理。</div>
            </div>
            <div class="setting-row__control">
              <n-input-number v-model:value="formData.log_retention_days" :min="1" :max="365" style="width: 100%" placeholder="1-365" />
            </div>
          </div>

          <div class="setting-row">
            <div class="setting-row__info">
              <div class="setting-row__label">调试模式</div>
              <div class="setting-row__desc">记录完整请求与响应正文，便于排障。含敏感信息，排障完成后建议关闭。</div>
            </div>
            <div class="setting-row__control">
              <n-space align="center">
                <n-switch v-model:value="debugModeEnabled" />
                <n-tag :type="debugModeEnabled ? 'warning' : 'default'" :bordered="false" size="small">
                  {{ debugModeEnabled ? '已启用' : '已关闭' }}
                </n-tag>
              </n-space>
            </div>
          </div>
        </div>
      </section>

      <section class="panel-card">
        <header class="panel-card__header">
          <h3 class="panel-card__title">模型测试</h3>
        </header>
        <div class="panel-card__body panel-card__body--flush">
          <div class="setting-row setting-row--block">
            <div class="setting-row__info">
              <div class="setting-row__label">默认测试提示词</div>
              <div class="setting-row__desc">
                渠道模型测试时默认使用的提示词。若某个渠道模型单独配置了测试提示词，将优先使用模型自己的配置。
              </div>
            </div>
            <div class="setting-row__control setting-row__control--block">
              <n-input
                v-model:value="formData.model_test_prompt"
                type="textarea"
                :autosize="{ minRows: 3, maxRows: 8 }"
                placeholder="例如：Hi"
                style="width: 100%"
              />
            </div>
          </div>

          <div class="setting-row setting-row--block">
            <div class="setting-row__info">
              <div class="setting-row__label">测试请求 User-Agent</div>
              <div class="setting-row__desc">用于渠道模型测试、模型同步、渠道健康检查的统一请求头。</div>
            </div>
            <div class="setting-row__control setting-row__control--block">
              <n-input
                v-model:value="formData.model_test_user_agent"
                type="textarea"
                :autosize="{ minRows: 2, maxRows: 5 }"
                placeholder="例如：Mozilla/5.0 ..."
                style="width: 100%"
              />
            </div>
          </div>
        </div>
      </section>

      <section class="settings-footer">
        <n-space justify="end">
          <n-button :disabled="isSaving" @click="handleReset">重置</n-button>
          <n-button type="primary" :loading="isSaving" @click="handleSave">保存设置</n-button>
        </n-space>
      </section>
    </template>

    <n-modal v-model:show="showDebugModeConfirm" preset="dialog" title="启用调试模式">
      <n-space vertical :size="6">
        <n-text>调试模式会记录更完整请求/响应内容，可能包含敏感信息。</n-text>
        <n-text depth="3" style="font-size: 12px">· 仅建议在问题排查阶段临时开启</n-text>
        <n-text depth="3" style="font-size: 12px">· 可能产生大量日志，注意磁盘占用</n-text>
      </n-space>
      <template #action>
        <n-space>
          <n-button @click="cancelDebugMode">取消</n-button>
          <n-button type="warning" @click="confirmDebugMode">确认启用</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import {
  NAlert,
  NButton,
  NInput,
  NInputNumber,
  NModal,
  NSkeleton,
  NSpace,
  NSwitch,
  NTag,
  NText,
  useDialog,
  useMessage,
} from 'naive-ui'
import settingsService from '@/services/settingsService'

const dialog = useDialog()
const message = useMessage()

interface SettingsData {
  server_port: number
  server_read_timeout: number
  server_write_timeout: number
  circuit_breaker_failure_threshold: number
  circuit_breaker_cooling_duration: number
  proxy_network_url: string
  proxy_request_timeout: number
  proxy_max_retry: number
  sniffer_enabled: boolean
  sniffer_stream_packet_count: number
  log_retention_days: number
  log_debug_enabled: boolean
  model_test_prompt: string
  model_test_user_agent: string
}

const DEFAULT_MODEL_TEST_USER_AGENT =
  'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) CherryStudio/1.7.13 Chrome/140.0.7339.249 Electron/38.7.0 Safari/537.36'

const isLoading = ref(false)
const isSaving = ref(false)
const loadError = ref('')
const debugModeEnabled = ref(false)
const showDebugModeConfirm = ref(false)
const isConfirming = ref(false)
const snifferKeywords = ref('')

const formData = ref<SettingsData>({
  server_port: 8080,
  server_read_timeout: 120,
  server_write_timeout: 0,
  circuit_breaker_failure_threshold: 3,
  circuit_breaker_cooling_duration: 60,
  proxy_network_url: '',
  proxy_request_timeout: 120,
  proxy_max_retry: 3,
  sniffer_enabled: true,
  sniffer_stream_packet_count: 1,
  log_retention_days: 30,
  log_debug_enabled: false,
  model_test_prompt: 'Hi',
  model_test_user_agent: DEFAULT_MODEL_TEST_USER_AGENT,
})

const loadSettings = async () => {
  isLoading.value = true
  loadError.value = ''
  try {
    const settings = await settingsService.getAllSettings()

    isConfirming.value = true

    if (settings.server_port) formData.value.server_port = parseInt(settings.server_port)
    if (settings.server_read_timeout) formData.value.server_read_timeout = parseInt(settings.server_read_timeout)
    if (settings.server_write_timeout !== undefined) formData.value.server_write_timeout = parseInt(settings.server_write_timeout)
    if (settings.circuit_breaker_failure_threshold) {
      formData.value.circuit_breaker_failure_threshold = parseInt(settings.circuit_breaker_failure_threshold)
    }
    if (settings.circuit_breaker_cooling_duration) {
      formData.value.circuit_breaker_cooling_duration = parseInt(settings.circuit_breaker_cooling_duration)
    }
    if (settings.proxy_request_timeout) formData.value.proxy_request_timeout = parseInt(settings.proxy_request_timeout)
    if (settings.proxy_network_url !== undefined) formData.value.proxy_network_url = settings.proxy_network_url || ''
    if (settings.proxy_max_retry) formData.value.proxy_max_retry = parseInt(settings.proxy_max_retry)
    if (settings.sniffer_enabled !== undefined) formData.value.sniffer_enabled = settings.sniffer_enabled === 'true'
    if (settings.sniffer_stream_packet_count) {
      formData.value.sniffer_stream_packet_count = Math.max(1, parseInt(settings.sniffer_stream_packet_count))
    }
    if (settings.log_retention_days) formData.value.log_retention_days = parseInt(settings.log_retention_days)
    if (settings.log_debug_enabled !== undefined) {
      debugModeEnabled.value = settings.log_debug_enabled === 'true'
    }
    if (settings.model_test_prompt !== undefined) {
      formData.value.model_test_prompt = settings.model_test_prompt || 'Hi'
    }
    if (settings.model_test_user_agent !== undefined) {
      formData.value.model_test_user_agent = settings.model_test_user_agent || DEFAULT_MODEL_TEST_USER_AGENT
    }

    if (settings.sniffer_plain_text_error_rules) {
      try {
        const keywords = JSON.parse(settings.sniffer_plain_text_error_rules)
        snifferKeywords.value = Array.isArray(keywords) ? keywords.join('\n') : ''
      } catch {
        snifferKeywords.value = ''
      }
    } else {
      snifferKeywords.value = ''
    }

    setTimeout(() => {
      isConfirming.value = false
    }, 0)
  } catch {
    loadError.value = '加载系统设置失败'
    message.error(loadError.value)
    isConfirming.value = false
  } finally {
    isLoading.value = false
  }
}

watch(debugModeEnabled, (newValue, oldValue) => {
  if (isConfirming.value) return

  if (newValue && !oldValue) {
    isConfirming.value = true
    debugModeEnabled.value = false
    showDebugModeConfirm.value = true
    setTimeout(() => {
      isConfirming.value = false
    }, 0)
  }
})

const confirmDebugMode = () => {
  isConfirming.value = true
  debugModeEnabled.value = true
  showDebugModeConfirm.value = false
  setTimeout(() => {
    isConfirming.value = false
  }, 0)
}

const cancelDebugMode = () => {
  showDebugModeConfirm.value = false
}

const handleSave = async () => {
  if (!formData.value.log_retention_days || formData.value.log_retention_days < 1) {
    message.error('日志保留天数必须大于 0')
    return
  }

  isSaving.value = true
  try {
    const keywords = snifferKeywords.value
      .split('\n')
      .map((item) => item.trim())
      .filter((item) => item.length > 0)

    await settingsService.updateSettings({
      settings: {
        server_port: formData.value.server_port.toString(),
        server_read_timeout: formData.value.server_read_timeout.toString(),
        server_write_timeout: formData.value.server_write_timeout.toString(),
        circuit_breaker_failure_threshold: formData.value.circuit_breaker_failure_threshold.toString(),
        circuit_breaker_cooling_duration: formData.value.circuit_breaker_cooling_duration.toString(),
        proxy_network_url: formData.value.proxy_network_url.trim(),
        proxy_request_timeout: formData.value.proxy_request_timeout.toString(),
        proxy_max_retry: formData.value.proxy_max_retry.toString(),
        sniffer_enabled: formData.value.sniffer_enabled.toString(),
        sniffer_stream_packet_count: Math.max(1, formData.value.sniffer_stream_packet_count).toString(),
        log_retention_days: formData.value.log_retention_days.toString(),
        log_debug_enabled: debugModeEnabled.value.toString(),
        model_test_prompt: formData.value.model_test_prompt.trim(),
        model_test_user_agent: formData.value.model_test_user_agent.trim(),
        sniffer_plain_text_error_rules: JSON.stringify(keywords),
      },
    })

    message.success('设置已保存')
  } catch {
    message.error('保存设置失败')
  } finally {
    isSaving.value = false
  }
}

const handleReset = () => {
  dialog.warning({
    title: '重置确认',
    content: '确定重新加载并恢复当前系统设置吗？',
    positiveText: '确认',
    negativeText: '取消',
    onPositiveClick: async () => {
      await loadSettings()
      message.info('已恢复当前系统设置')
    },
  })
}

onMounted(() => {
  loadSettings()
})
</script>

<style scoped>
.settings-page {
  max-width: 1100px;
}

.settings-footer {
  display: flex;
  justify-content: flex-end;
  padding: 4px 0 8px;
}

.panel-card__body--flush {
  padding: 0 18px;
}

.setting-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 320px;
  gap: 32px;
  align-items: center;
  padding: 16px 0;
  border-bottom: 1px solid var(--hydra-border);
}

.setting-row:last-child {
  border-bottom: none;
}

.setting-row__info {
  min-width: 0;
}

.setting-row__label {
  font-size: 13px;
  font-weight: 620;
  color: var(--hydra-text);
  line-height: 1.3;
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.setting-row__desc {
  margin-top: 4px;
  font-size: 12px;
  line-height: 1.55;
  color: var(--hydra-text-tertiary);
}

.setting-row__control {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  min-width: 0;
}

.setting-row__control--wide {
  width: 100%;
}

/* textarea 这类高控件：上下结构（信息在上，控件在下占满宽度） */
.setting-row--block {
  grid-template-columns: minmax(0, 1fr);
  align-items: stretch;
  gap: 10px;
}

.setting-row--block .setting-row__control {
  justify-content: stretch;
  width: 100%;
}

.setting-row__control--block {
  width: 100%;
}

@media (max-width: 820px) {
  .setting-row {
    grid-template-columns: 1fr;
    gap: 10px;
  }

  .setting-row__control {
    justify-content: stretch;
    width: 100%;
  }
}
</style>
