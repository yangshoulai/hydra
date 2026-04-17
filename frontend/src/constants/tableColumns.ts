// 统一的表格列宽常量
// 用于多个页面中出现的"时间 / 状态 / 耗时 / 模型 / 重试 / Tokens / 操作"等列，
// 保证整站表格视觉一致。
export const COL_WIDTH = {
  id: 72,
  time: 180,
  status: 100,
  duration: 100,
  model: 160,
  channelModel: 180,
  channel: 180,
  retry: 80,
  tokens: 120,
  method: 90,
  endpointType: 150,
  count: 110,
  actions: 160,
} as const

export type ColumnKey = keyof typeof COL_WIDTH
