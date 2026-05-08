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
              <div class="setting-row__desc">调用上游厂商接口的超时时间，0 表示不超时；大于 0 时超时后触发重试。</div>
            </div>
            <div class="setting-row__control">
              <n-input-number v-model:value="formData.proxy_request_timeout" :min="0" :max="300" style="width: 100%" placeholder="0-300" />
            </div>
          </div>

          <div class="setting-row">
            <div class="setting-row__info">
              <div class="setting-row__label">流式保活间隔（秒）</div>
              <div class="setting-row__desc">0 表示禁用；大于 0 时仅对流式响应生效，间隔内未收到渠道数据则发送 <code>: keepalive</code> 注释帧。</div>
            </div>
            <div class="setting-row__control">
              <n-input-number v-model:value="formData.proxy_keepalive_interval" :min="0" :max="120" style="width: 100%" placeholder="0-120" />
            </div>
          </div>

          <div class="setting-row">
            <div class="setting-row__info">
              <div class="setting-row__label">非流式保活</div>
              <div class="setting-row__desc">默认关闭；启用后会在长时间无上游响应时写出 JSON 空白心跳，避免中间代理读超时。</div>
            </div>
            <div class="setting-row__control">
              <n-space align="center">
                <n-switch v-model:value="formData.proxy_non_stream_keepalive_enabled" />
                <n-tag :type="formData.proxy_non_stream_keepalive_enabled ? 'warning' : 'default'" :bordered="false" size="small">
                  {{ formData.proxy_non_stream_keepalive_enabled ? '已启用' : '已关闭' }}
                </n-tag>
              </n-space>
            </div>
          </div>

          <div class="setting-row">
            <div class="setting-row__info">
              <div class="setting-row__label">非流式首个保活延迟（秒）</div>
              <div class="setting-row__desc">从请求开始计时，超过该秒数仍未完成响应时写出第一个 JSON 空白心跳；写出后状态码锁定且不再重试。</div>
            </div>
            <div class="setting-row__control">
              <n-input-number
                v-model:value="formData.proxy_non_stream_keepalive_first_delay"
                :min="1"
                :max="120"
                :disabled="!formData.proxy_non_stream_keepalive_enabled"
                style="width: 100%"
                placeholder="建议 80-90"
              />
            </div>
          </div>

          <div class="setting-row">
            <div class="setting-row__info">
              <div class="setting-row__label">非流式保活间隔（秒）</div>
              <div class="setting-row__desc">第一个心跳写出后，按该间隔持续写出 JSON 空白心跳；建议小于 Cloudflare 读超时时间并留足余量。</div>
            </div>
            <div class="setting-row__control">
              <n-input-number
                v-model:value="formData.proxy_non_stream_keepalive_interval"
                :min="1"
                :max="120"
                :disabled="!formData.proxy_non_stream_keepalive_enabled"
                style="width: 100%"
                placeholder="建议 25-30"
              />
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
              <div class="setting-row__label">渠道负载策略（兼容保留）</div>
              <div class="setting-row__desc">当前代理会把所有可用渠道模型放入同一个候选池，并只按渠道模型权重加权随机；渠道权重仅作为新建渠道模型的初始权重。</div>
            </div>
            <div class="setting-row__control">
              <n-select
                v-model:value="formData.proxy_load_balance_strategy"
                :options="loadBalanceStrategyOptions"
                disabled
                placeholder="请选择负载策略"
              />
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
          <h3 class="panel-card__title">安全与流量控制</h3>
        </header>
        <div class="panel-card__body panel-card__body--flush">
          <div class="setting-row setting-row--block">
            <div class="setting-row__info">
              <div class="setting-row__label">
                JWT 签名密钥
                <n-tag size="tiny" type="warning" :bordered="false">保存后自动重启</n-tag>
              </div>
              <div class="setting-row__desc">用于管理后台 JWT 签名。初始化时会自动生成；修改后当前登录状态会失效，需要重新登录。</div>
            </div>
            <div class="setting-row__control setting-row__control--block">
              <div class="secret-row">
                <n-input
                  v-model:value="formData.security_jwt_secret"
                  type="password"
                  show-password-on="click"
                  placeholder="至少 32 个字符"
                  style="width: 100%"
                />
                <n-button @click="generateJWTSecret">重新生成</n-button>
              </div>
            </div>
          </div>

          <div class="setting-row">
            <div class="setting-row__info">
              <div class="setting-row__label">代理请求体上限（MB）</div>
              <div class="setting-row__desc">限制代理入口读取的请求体大小，0 表示不限制。保存后对新请求立即生效。</div>
            </div>
            <div class="setting-row__control">
              <n-input-number v-model:value="formData.proxy_max_body_mb" :min="0" :max="1024" style="width: 100%" placeholder="0-1024" />
            </div>
          </div>

          <div class="setting-row">
            <div class="setting-row__info">
              <div class="setting-row__label">代理限流</div>
              <div class="setting-row__desc">启用后按全局与访问令牌两个维度进行令牌桶限流；保存后立即生效。</div>
            </div>
            <div class="setting-row__control">
              <n-space align="center">
                <n-switch v-model:value="formData.proxy_rate_limit_enabled" />
                <n-tag :type="formData.proxy_rate_limit_enabled ? 'success' : 'default'" :bordered="false" size="small">
                  {{ formData.proxy_rate_limit_enabled ? '已启用' : '已关闭' }}
                </n-tag>
              </n-space>
            </div>
          </div>

          <div class="setting-row">
            <div class="setting-row__info">
              <div class="setting-row__label">全局限流（RPS / 突发）</div>
              <div class="setting-row__desc">全站代理请求共享的速率限制；RPS 为 0 表示不限制全局维度。</div>
            </div>
            <div class="setting-row__control setting-row__control--wide">
              <n-space style="width: 100%" :wrap="false">
                <n-input-number
                  v-model:value="formData.proxy_rate_limit_global_rps"
                  :min="0"
                  :max="100000"
                  :disabled="!formData.proxy_rate_limit_enabled"
                  placeholder="RPS"
                />
                <n-input-number
                  v-model:value="formData.proxy_rate_limit_global_burst"
                  :min="0"
                  :max="100000"
                  :disabled="!formData.proxy_rate_limit_enabled"
                  placeholder="突发"
                />
              </n-space>
            </div>
          </div>

          <div class="setting-row">
            <div class="setting-row__info">
              <div class="setting-row__label">单令牌限流（RPS / 突发）</div>
              <div class="setting-row__desc">按访问令牌隔离请求速率；RPS 为 0 表示不限制单令牌维度。</div>
            </div>
            <div class="setting-row__control setting-row__control--wide">
              <n-space style="width: 100%" :wrap="false">
                <n-input-number
                  v-model:value="formData.proxy_rate_limit_token_rps"
                  :min="0"
                  :max="100000"
                  :disabled="!formData.proxy_rate_limit_enabled"
                  placeholder="RPS"
                />
                <n-input-number
                  v-model:value="formData.proxy_rate_limit_token_burst"
                  :min="0"
                  :max="100000"
                  :disabled="!formData.proxy_rate_limit_enabled"
                  placeholder="突发"
                />
              </n-space>
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
              <div class="setting-row__label">非流式响应嗅探</div>
              <div class="setting-row__desc">解析非流式响应正文，识别 HTTP 200 但业务失败的情况，计入熔断。</div>
            </div>
            <div class="setting-row__control">
              <n-space align="center">
                <n-switch v-model:value="formData.sniffer_non_stream_enabled" />
                <n-tag :type="formData.sniffer_non_stream_enabled ? 'success' : 'default'" :bordered="false" size="small">
                  {{ formData.sniffer_non_stream_enabled ? '已启用' : '已关闭' }}
                </n-tag>
              </n-space>
            </div>
          </div>

          <div class="setting-row">
            <div class="setting-row__info">
              <div class="setting-row__label">流式响应嗅探</div>
              <div class="setting-row__desc">预读前几个流式数据包，识别空流、假 200 和业务错误。与流式保活互斥。</div>
            </div>
            <div class="setting-row__control">
              <n-space align="center">
                <n-switch v-model:value="formData.sniffer_stream_enabled" />
                <n-tag :type="formData.sniffer_stream_enabled ? 'success' : 'default'" :bordered="false" size="small">
                  {{ formData.sniffer_stream_enabled ? '已启用' : '已关闭' }}
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
                :disabled="!formData.sniffer_stream_enabled"
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
                :disabled="!hasAnySnifferEnabled"
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

    <n-modal v-model:show="showNonStreamKeepaliveConfirm" preset="dialog" title="启用非流式保活">
      <n-space vertical :size="6">
        <n-text>非流式保活会提前向客户端写出 JSON 空白字符，用于绕过中间代理长时间无响应导致的读超时。</n-text>
        <n-text depth="3" style="font-size: 12px">· 首个心跳写出后，HTTP 状态码会被锁定为 200</n-text>
        <n-text depth="3" style="font-size: 12px">· 状态码锁定后，响应嗅探仍可记录失败，但不能再切换渠道重试</n-text>
        <n-text depth="3" style="font-size: 12px">· 上游失败时只能在 200 响应体内返回错误 JSON，可能影响部分客户端 SDK</n-text>
      </n-space>
      <template #action>
        <n-space>
          <n-button @click="cancelNonStreamKeepalive">取消</n-button>
          <n-button type="warning" @click="confirmNonStreamKeepalive">确认启用</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import {
  NAlert,
  NButton,
  NInput,
  NInputNumber,
  NModal,
  NSelect,
  NSkeleton,
  NSpace,
  NSwitch,
  NTag,
  NText,
  useDialog,
  useMessage,
} from 'naive-ui'
import { settingsService } from '@/services/settingsService'

const dialog = useDialog()
const message = useMessage()

interface SettingsData {
  security_jwt_secret: string
  server_port: number
  server_read_timeout: number
  server_write_timeout: number
  circuit_breaker_failure_threshold: number
  circuit_breaker_cooling_duration: number
  proxy_network_url: string
  proxy_request_timeout: number
  proxy_keepalive_interval: number
  proxy_non_stream_keepalive_enabled: boolean
  proxy_non_stream_keepalive_first_delay: number
  proxy_non_stream_keepalive_interval: number
  proxy_max_retry: number
  proxy_load_balance_strategy: 'weighted_random' | 'round_robin'
  proxy_max_body_mb: number
  proxy_rate_limit_enabled: boolean
  proxy_rate_limit_global_rps: number
  proxy_rate_limit_global_burst: number
  proxy_rate_limit_token_rps: number
  proxy_rate_limit_token_burst: number
  sniffer_non_stream_enabled: boolean
  sniffer_stream_enabled: boolean
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
const showNonStreamKeepaliveConfirm = ref(false)
const isConfirming = ref(false)
const isConflictAdjusting = ref(false)
const snifferKeywords = ref('')

const formData = ref<SettingsData>({
  security_jwt_secret: '',
  server_port: 8080,
  server_read_timeout: 120,
  server_write_timeout: 0,
  circuit_breaker_failure_threshold: 3,
  circuit_breaker_cooling_duration: 60,
  proxy_network_url: '',
  proxy_request_timeout: 120,
  proxy_keepalive_interval: 0,
  proxy_non_stream_keepalive_enabled: false,
  proxy_non_stream_keepalive_first_delay: 80,
  proxy_non_stream_keepalive_interval: 30,
  proxy_max_retry: 3,
  proxy_load_balance_strategy: 'weighted_random',
  proxy_max_body_mb: 50,
  proxy_rate_limit_enabled: true,
  proxy_rate_limit_global_rps: 300,
  proxy_rate_limit_global_burst: 600,
  proxy_rate_limit_token_rps: 60,
  proxy_rate_limit_token_burst: 120,
  sniffer_non_stream_enabled: true,
  sniffer_stream_enabled: true,
  sniffer_stream_packet_count: 1,
  log_retention_days: 30,
  log_debug_enabled: false,
  model_test_prompt: 'Hi',
  model_test_user_agent: DEFAULT_MODEL_TEST_USER_AGENT,
})

const hasAnySnifferEnabled = computed(() => formData.value.sniffer_non_stream_enabled || formData.value.sniffer_stream_enabled)

const loadBalanceStrategyOptions = [
  { label: '兼容保留：加权随机', value: 'weighted_random' },
  { label: '兼容保留：轮询', value: 'round_robin' },
]

const runSilentFormUpdate = (updater: () => void) => {
  isConflictAdjusting.value = true
  updater()
  setTimeout(() => {
    isConflictAdjusting.value = false
  }, 0)
}

const loadSettings = async () => {
  isLoading.value = true
  loadError.value = ''
  try {
    const settings = await settingsService.getAllSettings()

    isConfirming.value = true

    if (settings.security_jwt_secret !== undefined) {
      formData.value.security_jwt_secret = settings.security_jwt_secret || ''
    }
    if (settings.server_port) formData.value.server_port = parseInt(settings.server_port)
    if (settings.server_read_timeout) formData.value.server_read_timeout = parseInt(settings.server_read_timeout)
    if (settings.server_write_timeout !== undefined) formData.value.server_write_timeout = parseInt(settings.server_write_timeout)
    if (settings.circuit_breaker_failure_threshold) {
      formData.value.circuit_breaker_failure_threshold = parseInt(settings.circuit_breaker_failure_threshold)
    }
    if (settings.circuit_breaker_cooling_duration) {
      formData.value.circuit_breaker_cooling_duration = parseInt(settings.circuit_breaker_cooling_duration)
    }
    if (settings.proxy_request_timeout !== undefined) {
      formData.value.proxy_request_timeout = Math.max(0, parseInt(settings.proxy_request_timeout) || 0)
    }
    if (settings.proxy_keepalive_interval !== undefined) {
      formData.value.proxy_keepalive_interval = Math.min(120, Math.max(0, parseInt(settings.proxy_keepalive_interval) || 0))
    }
    if (settings.proxy_non_stream_keepalive_enabled !== undefined) {
      formData.value.proxy_non_stream_keepalive_enabled = settings.proxy_non_stream_keepalive_enabled === 'true'
    }
    if (settings.proxy_non_stream_keepalive_first_delay !== undefined) {
      formData.value.proxy_non_stream_keepalive_first_delay = Math.min(
        120,
        Math.max(0, parseInt(settings.proxy_non_stream_keepalive_first_delay) || 0),
      )
    }
    if (settings.proxy_non_stream_keepalive_interval !== undefined) {
      formData.value.proxy_non_stream_keepalive_interval = Math.min(
        120,
        Math.max(0, parseInt(settings.proxy_non_stream_keepalive_interval) || 0),
      )
    }
    if (settings.proxy_network_url !== undefined) formData.value.proxy_network_url = settings.proxy_network_url || ''
    if (settings.proxy_max_retry !== undefined) {
      formData.value.proxy_max_retry = Math.max(0, parseInt(settings.proxy_max_retry) || 0)
    }
    if (settings.proxy_load_balance_strategy !== undefined) {
      formData.value.proxy_load_balance_strategy =
        settings.proxy_load_balance_strategy === 'round_robin' ? 'round_robin' : 'weighted_random'
    }
    if (settings.proxy_max_body_bytes !== undefined) {
      const bytes = Math.max(0, parseInt(settings.proxy_max_body_bytes) || 0)
      formData.value.proxy_max_body_mb = Math.round(bytes / 1024 / 1024)
    }
    if (settings.proxy_rate_limit_enabled !== undefined) {
      formData.value.proxy_rate_limit_enabled = settings.proxy_rate_limit_enabled === 'true'
    }
    if (settings.proxy_rate_limit_global_rps !== undefined) {
      formData.value.proxy_rate_limit_global_rps = Math.max(0, parseInt(settings.proxy_rate_limit_global_rps) || 0)
    }
    if (settings.proxy_rate_limit_global_burst !== undefined) {
      formData.value.proxy_rate_limit_global_burst = Math.max(0, parseInt(settings.proxy_rate_limit_global_burst) || 0)
    }
    if (settings.proxy_rate_limit_token_rps !== undefined) {
      formData.value.proxy_rate_limit_token_rps = Math.max(0, parseInt(settings.proxy_rate_limit_token_rps) || 0)
    }
    if (settings.proxy_rate_limit_token_burst !== undefined) {
      formData.value.proxy_rate_limit_token_burst = Math.max(0, parseInt(settings.proxy_rate_limit_token_burst) || 0)
    }
    const legacySnifferEnabled = settings.sniffer_enabled !== undefined ? settings.sniffer_enabled === 'true' : true
    if (settings.sniffer_non_stream_enabled !== undefined) {
      formData.value.sniffer_non_stream_enabled = settings.sniffer_non_stream_enabled === 'true'
    } else {
      formData.value.sniffer_non_stream_enabled = legacySnifferEnabled
    }
    if (settings.sniffer_stream_enabled !== undefined) {
      formData.value.sniffer_stream_enabled = settings.sniffer_stream_enabled === 'true'
    } else {
      formData.value.sniffer_stream_enabled = legacySnifferEnabled
    }
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

watch(
  () => formData.value.proxy_non_stream_keepalive_enabled,
  (newValue, oldValue) => {
    if (isConfirming.value || isConflictAdjusting.value) return
    if (!newValue || oldValue === undefined) return

    isConfirming.value = true
    formData.value.proxy_non_stream_keepalive_enabled = false
    showNonStreamKeepaliveConfirm.value = true
    setTimeout(() => {
      isConfirming.value = false
    }, 0)
  },
)

watch(
  () => formData.value.sniffer_stream_enabled,
  (newValue, oldValue) => {
    if (isConfirming.value || isConflictAdjusting.value) return
    if (!newValue || oldValue === undefined || formData.value.proxy_keepalive_interval <= 0) return

    dialog.warning({
      title: '启用流式响应嗅探',
      content: '流式响应嗅探需要在转发前预读上游数据，与流式保活互斥。继续后将自动把“流式保活间隔”设置为 0。',
      positiveText: '继续启用',
      negativeText: '取消',
      onPositiveClick: () => {
        runSilentFormUpdate(() => {
          formData.value.proxy_keepalive_interval = 0
        })
      },
      onNegativeClick: () => {
        runSilentFormUpdate(() => {
          formData.value.sniffer_stream_enabled = oldValue
        })
      },
    })
  },
)

watch(
  () => formData.value.proxy_keepalive_interval,
  (newValue, oldValue) => {
    if (isConfirming.value || isConflictAdjusting.value) return
    if (newValue <= 0 || !formData.value.sniffer_stream_enabled) return

    dialog.warning({
      title: '启用流式保活',
      content: '流式保活会提前向客户端发送保活帧，这会影响流式响应嗅探的预读与错误判定。继续后将自动关闭“流式响应嗅探”。',
      positiveText: '继续启用',
      negativeText: '取消',
      onPositiveClick: () => {
        runSilentFormUpdate(() => {
          formData.value.sniffer_stream_enabled = false
        })
      },
      onNegativeClick: () => {
        runSilentFormUpdate(() => {
          formData.value.proxy_keepalive_interval = oldValue ?? 0
        })
      },
    })
  },
)

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

const confirmNonStreamKeepalive = () => {
  isConfirming.value = true
  if (formData.value.proxy_non_stream_keepalive_first_delay <= 0) {
    formData.value.proxy_non_stream_keepalive_first_delay = 80
  }
  if (formData.value.proxy_non_stream_keepalive_interval <= 0) {
    formData.value.proxy_non_stream_keepalive_interval = 30
  }
  formData.value.proxy_non_stream_keepalive_enabled = true
  showNonStreamKeepaliveConfirm.value = false
  setTimeout(() => {
    isConfirming.value = false
  }, 0)
}

const cancelNonStreamKeepalive = () => {
  showNonStreamKeepaliveConfirm.value = false
}

const generateJWTSecret = () => {
  const bytes = new Uint8Array(32)
  window.crypto.getRandomValues(bytes)
  formData.value.security_jwt_secret = btoa(String.fromCharCode(...bytes))
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/g, '')
}

const handleSave = async () => {
  if (formData.value.security_jwt_secret.trim().length < 32) {
    message.error('JWT 签名密钥至少需要 32 个字符')
    return
  }
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
        security_jwt_secret: formData.value.security_jwt_secret.trim(),
        server_port: formData.value.server_port.toString(),
        server_read_timeout: formData.value.server_read_timeout.toString(),
        server_write_timeout: formData.value.server_write_timeout.toString(),
        circuit_breaker_failure_threshold: formData.value.circuit_breaker_failure_threshold.toString(),
        circuit_breaker_cooling_duration: formData.value.circuit_breaker_cooling_duration.toString(),
        proxy_network_url: formData.value.proxy_network_url.trim(),
        proxy_request_timeout: formData.value.proxy_request_timeout.toString(),
        proxy_keepalive_interval: formData.value.proxy_keepalive_interval.toString(),
        proxy_non_stream_keepalive_enabled: formData.value.proxy_non_stream_keepalive_enabled.toString(),
        proxy_non_stream_keepalive_first_delay: Math.max(0, formData.value.proxy_non_stream_keepalive_first_delay).toString(),
        proxy_non_stream_keepalive_interval: Math.max(0, formData.value.proxy_non_stream_keepalive_interval).toString(),
        proxy_max_retry: formData.value.proxy_max_retry.toString(),
        proxy_load_balance_strategy: formData.value.proxy_load_balance_strategy,
        proxy_max_body_bytes: Math.round(Math.max(0, formData.value.proxy_max_body_mb) * 1024 * 1024).toString(),
        proxy_rate_limit_enabled: formData.value.proxy_rate_limit_enabled.toString(),
        proxy_rate_limit_global_rps: Math.max(0, formData.value.proxy_rate_limit_global_rps).toString(),
        proxy_rate_limit_global_burst: Math.max(0, formData.value.proxy_rate_limit_global_burst).toString(),
        proxy_rate_limit_token_rps: Math.max(0, formData.value.proxy_rate_limit_token_rps).toString(),
        proxy_rate_limit_token_burst: Math.max(0, formData.value.proxy_rate_limit_token_burst).toString(),
        sniffer_non_stream_enabled: formData.value.sniffer_non_stream_enabled.toString(),
        sniffer_stream_enabled: formData.value.sniffer_stream_enabled.toString(),
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

.secret-row {
  display: flex;
  align-items: center;
  gap: 8px;
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

  .secret-row {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>
