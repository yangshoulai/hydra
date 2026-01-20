/**
 * 端点类型定义
 */

export interface EndpointInfo {
  name: string
  type: string
  path: string
  description: string
  test_payload: Record<string, any>
}
