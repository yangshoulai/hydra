import type { TestModeResult, TestModelResponse } from '@/types/channel'
import type { ModelTestResultItem } from '@/types/modelTest'

function buildFallbackNonStream(result: TestModelResponse): TestModeResult {
  return {
    tested: true,
    success: result.success,
    message: result.message,
    latency: result.latency,
  }
}

function buildFallbackStream(): TestModeResult {
  return {
    tested: false,
    success: false,
    message: '未执行流式测试',
  }
}

export function normalizeTestModes(result: TestModelResponse): { nonStream: TestModeResult; stream: TestModeResult } {
  return {
    nonStream: result.non_stream || buildFallbackNonStream(result),
    stream: result.stream || buildFallbackStream(),
  }
}

export function formatModeResultSummary(label: string, modeResult: TestModeResult): string {
  if (!modeResult.tested) {
    return `${label}:跳过`
  }

  const status = modeResult.success ? '成功' : '失败'
  const latency = modeResult.latency ? `(${modeResult.latency})` : ''
  return `${label}:${status}${latency}`
}

export function formatTestResultSummary(result: TestModelResponse): string {
  const { nonStream, stream } = normalizeTestModes(result)
  return `${formatModeResultSummary('非流式', nonStream)}，${formatModeResultSummary('流式', stream)}`
}

export function buildTestResultDetail(result: TestModelResponse): string {
  const { nonStream, stream } = normalizeTestModes(result)

  const lines = [`整体结果：${result.message || (result.success ? '测试通过' : '测试失败')}`]

  lines.push(buildModeDetail('非流式', nonStream))
  lines.push(buildModeDetail('流式', stream))

  return lines.join('\n')
}

function buildModeDetail(label: string, modeResult: TestModeResult): string {
  if (!modeResult.tested) {
    return `${label}：未执行`
  }

  const parts = [`${label}：${modeResult.success ? '通过' : '失败'}`]

  if (modeResult.latency) {
    parts.push(`耗时 ${modeResult.latency}`)
  }
  if (modeResult.message) {
    parts.push(modeResult.message)
  }

  return parts.join('，')
}

export function createModelTestResultItem(input: {
  id: string
  channelName?: string
  modelName?: string
  channelModel: string
  endpointType: string
  result?: TestModelResponse
  errorMessage?: string
}): ModelTestResultItem {
  if (input.result) {
    const { nonStream, stream } = normalizeTestModes(input.result)
    return {
      id: input.id,
      channelName: input.channelName,
      modelName: input.modelName,
      channelModel: input.channelModel,
      endpointType: input.endpointType,
      success: input.result.success,
      nonStream,
      stream,
      detail: buildTestResultDetail(input.result),
    }
  }

  const errorMessage = input.errorMessage || '测试失败'
  const nonStream: TestModeResult = {
    tested: true,
    success: false,
    message: errorMessage,
  }
  const stream = buildFallbackStream()

  return {
    id: input.id,
    channelName: input.channelName,
    modelName: input.modelName,
    channelModel: input.channelModel,
    endpointType: input.endpointType,
    success: false,
    nonStream,
    stream,
    detail: [`整体结果：${errorMessage}`, buildModeDetail('非流式', nonStream), buildModeDetail('流式', stream)].join('\n'),
  }
}
