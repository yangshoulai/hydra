export interface ModelTestClientHeaderProfile {
  id: string
  name: string
  headers: Record<string, string>
}

export const DEFAULT_MODEL_TEST_CLIENT_HEADER_PROFILES = [
  {
    id: 'codex_cli',
    name: 'Codex CLI',
    headers: {},
  },
  {
    id: 'claude_code',
    name: 'Claude Code',
    headers: {},
  },
] satisfies ModelTestClientHeaderProfile[]

export const DEFAULT_MODEL_TEST_CLIENT_HEADER_PROFILES_JSON = JSON.stringify(
  DEFAULT_MODEL_TEST_CLIENT_HEADER_PROFILES,
  null,
  2,
)

export function parseModelTestClientHeaderProfiles(value?: string): ModelTestClientHeaderProfile[] {
  if (!value?.trim()) return []
  try {
    const parsed = JSON.parse(value)
    if (!Array.isArray(parsed)) return []
    return parsed
      .map((item) => ({
        id: typeof item?.id === 'string' ? item.id.trim() : '',
        name: typeof item?.name === 'string' ? item.name.trim() : '',
        headers: normalizeHeaders(item?.headers),
      }))
      .filter((item) => item.id && item.name)
  } catch {
    return []
  }
}

export function buildModelTestClientHeaderProfileOptions(profiles: ModelTestClientHeaderProfile[]) {
  return [
    { label: '默认（不附加客户端请求头）', value: '' },
    ...profiles.map((profile) => ({
      label: `${profile.name}（${Object.keys(profile.headers).length} 个请求头）`,
      value: profile.id,
    })),
  ]
}

function normalizeHeaders(value: unknown): Record<string, string> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return {}
  const result: Record<string, string> = {}
  Object.entries(value as Record<string, unknown>).forEach(([key, val]) => {
    if (!key.trim()) return
    result[key.trim()] = typeof val === 'string' ? val.trim() : String(val ?? '').trim()
  })
  return result
}
