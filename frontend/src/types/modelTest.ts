import type { TestModeResult } from './channel'

export interface ModelTestResultItem {
  id: string
  channelName?: string
  modelName?: string
  channelModel: string
  endpointType: string
  success: boolean
  nonStream: TestModeResult
  stream: TestModeResult
  detail: string
}
