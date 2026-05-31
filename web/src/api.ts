// ─── Auth Types ───────────────────────────────────────────────────────────────

export interface AuthStatus {
  initialized: boolean
  authenticated: boolean
}

export interface AuthUser {
  id: string
  username: string
}

export interface LoginResponse {
  token: string
  user: AuthUser
}

export interface APIKey {
  id: string
  name: string
  key?: string
  prefix: string
  last_used?: string
  created_at: string
}

// ─── Core Request Helper ─────────────────────────────────────────────────────

export class ApiError extends Error {
  code: number
  constructor(code: number, message: string) {
    super(message)
    this.code = code
    this.name = 'ApiError'
  }
}

interface ApiResponse<T = unknown> {
  code: number
  message: string
  data: T
}

interface PagedData<T> {
  total: number
  page: number
  page_size: number
  data: T[]
}

export type { ApiResponse, PagedData }

export async function request<T>(
  path: string,
  options: RequestInit = {}
): Promise<ApiResponse<T>> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string> || {}),
  }

  const res = await fetch(path, {
    ...options,
    headers,
    credentials: 'same-origin',
  })

  const json = await res.json()

  // Token expired or invalid — redirect to login
  if (res.status === 401 && !path.includes('/auth/')) {
    window.dispatchEvent(new CustomEvent('auth-expired'))
    throw new ApiError(401, '登录已过期，请重新登录')
  }

  if (!res.ok) {
    throw new ApiError(json.code || res.status, json.message || res.statusText)
  }
  return json as ApiResponse<T>
}

// ─── Auth API ────────────────────────────────────────────────────────────────

export async function checkAuthStatus(): Promise<AuthStatus> {
  const res = await fetch('/api/auth/status', { credentials: 'same-origin' })
  const json = await res.json()
  return json as AuthStatus
}

export async function register(username: string, password: string): Promise<LoginResponse> {
  const res = await fetch('/api/auth/register', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'same-origin',
    body: JSON.stringify({ username, password }),
  })
  const json = await res.json()
  if (!res.ok) {
    throw new ApiError(json.code || res.status, json.message || '注册失败')
  }
  return json as LoginResponse
}

export async function login(username: string, password: string): Promise<LoginResponse> {
  const res = await fetch('/api/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'same-origin',
    body: JSON.stringify({ username, password }),
  })
  const json = await res.json()
  if (!res.ok) {
    throw new ApiError(json.code || res.status, json.message || '登录失败')
  }
  return json as LoginResponse
}

export async function logout(): Promise<void> {
  await fetch('/api/auth/logout', {
    method: 'POST',
    credentials: 'same-origin',
  })
}

export async function changePassword(old_password: string, new_password: string): Promise<void> {
  const res = await fetch('/api/auth/change-password', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'same-origin',
    body: JSON.stringify({ old_password, new_password }),
  })
  if (!res.ok) {
    const json = await res.json().catch(() => null)
    throw new ApiError(res.status, json?.message || '修改密码失败')
  }
}

export async function getMe(): Promise<AuthUser> {
  const res = await fetch('/api/auth/me', { credentials: 'same-origin' })
  const json = await res.json()
  if (!res.ok) {
    throw new ApiError(json.code || res.status, json.message || '获取用户信息失败')
  }
  return json as AuthUser
}

// ─── API Key Management ──────────────────────────────────────────────────────

export async function listAPIKeys(): Promise<APIKey[]> {
  const res = await fetch('/api/keys', { credentials: 'same-origin' })
  const json = await res.json()
  if (!res.ok) {
    throw new ApiError(json.code || res.status, json.message || '获取 API Key 列表失败')
  }
  return (json as { keys: APIKey[] }).keys
}

export async function createAPIKey(name: string): Promise<APIKey> {
  const res = await fetch('/api/keys', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'same-origin',
    body: JSON.stringify({ name }),
  })
  const json = await res.json()
  if (!res.ok) {
    throw new ApiError(json.code || res.status, json.message || '创建 API Key 失败')
  }
  return json as APIKey
}

export async function deleteAPIKey(id: string): Promise<void> {
  const res = await fetch(`/api/keys/${id}`, {
    method: 'DELETE',
    credentials: 'same-origin',
  })
  if (!res.ok) {
    const json = await res.json().catch(() => null)
    throw new ApiError(res.status, json?.message || '删除 API Key 失败')
  }
}

// ─── Stats ───────────────────────────────────────────────────────────────────

export interface ChannelStat {
  channel: string
  total: number
  success_count: number
  failed_count: number
  success_percent: number
  failed_percent: number
}

export function fetchStats(from: string, to: string): Promise<ApiResponse<ChannelStat[]>> {
  return request<ChannelStat[]>(`/api/stats?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`)
}

// ─── Messages ────────────────────────────────────────────────────────────────

export interface Message {
  id: string
  source: string
  channel: string
  title: string
  body: string
  status: string
  fail_reason?: string
  received_at: string
  pushed_at?: string
  ack_at?: string
}

export interface MessageFilter {
  page?: number
  page_size?: number
  channel?: string
  status?: string
  from?: string
  to?: string
}

export function fetchMessages(filter: MessageFilter): Promise<ApiResponse<PagedData<Message>>> {
  const params = new URLSearchParams()
  if (filter.page) params.set('page', String(filter.page))
  if (filter.page_size) params.set('page_size', String(filter.page_size))
  if (filter.channel) params.set('channel', filter.channel)
  if (filter.status) params.set('status', filter.status)
  if (filter.from) params.set('from', filter.from)
  if (filter.to) params.set('to', filter.to)
  return request<PagedData<Message>>(`/api/messages?${params.toString()}`)
}

// ─── Reminders ───────────────────────────────────────────────────────────────

export interface Reminder {
  id: string
  title: string
  body?: string
  channel: string
  trigger_at: string
  repeat_type: string
  repeat_rule?: string
  status: string
  next_trigger?: string
  last_triggered?: string
  created_at: string
  updated_at: string
}

export interface CreateReminderRequest {
  title: string
  body?: string
  trigger_at: string
  channel?: string
  repeat?: string
  repeat_rule?: string
}

export function fetchReminders(status?: string): Promise<ApiResponse<Reminder[]>> {
  const params = status ? `?status=${encodeURIComponent(status)}` : ''
  return request<Reminder[]>(`/api/reminders${params}`)
}

export function createReminder(data: CreateReminderRequest): Promise<ApiResponse<Reminder>> {
  return request<Reminder>('/api/reminders', {
    method: 'POST',
    body: JSON.stringify(data),
  })
}

export function updateReminder(id: string, data: Partial<CreateReminderRequest>): Promise<ApiResponse<null>> {
  return request<null>(`/api/reminders/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  })
}

export function cancelReminder(id: string): Promise<ApiResponse<null>> {
  return request<null>(`/api/reminders/${id}`, {
    method: 'DELETE',
  })
}

// ─── Push ────────────────────────────────────────────────────────────────────

export interface PushRequest {
  title: string
  body: string
  channel?: string
  sound?: string
  icon?: string
  group?: string
  level?: string
  url?: string
  device_keys?: string[]
  extra?: Record<string, string>
}

export interface PushResult {
  id: string
  status: string
}

export function sendPush(data: PushRequest): Promise<ApiResponse<PushResult>> {
  return request<PushResult>('/api/push', {
    method: 'POST',
    body: JSON.stringify(data),
  })
}

export function sendTestPush(data: PushRequest): Promise<ApiResponse<PushResult>> {
  return request<PushResult>('/api/push/test', {
    method: 'POST',
    body: JSON.stringify(data),
  })
}

// ─── Devices ─────────────────────────────────────────────────────────────────

export interface Device {
  id: string
  name: string
  device_key: string
  type?: string
  is_default: boolean
  online?: boolean
  created_at: string
  updated_at: string
}

export interface CreateDeviceRequest {
  name: string
  device_key: string
  is_default?: boolean
}

export function fetchDevices(): Promise<ApiResponse<Device[]>> {
  return request<Device[]>('/api/devices')
}

export function fetchDeviceStatus(): Promise<ApiResponse<Device[]>> {
  return request<Device[]>('/api/devices/status')
}

export function createDevice(data: CreateDeviceRequest): Promise<ApiResponse<Device>> {
  return request<Device>('/api/devices', {
    method: 'POST',
    body: JSON.stringify(data),
  })
}

export function updateDevice(id: string, data: Partial<CreateDeviceRequest>): Promise<ApiResponse<null>> {
  return request<null>(`/api/devices/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  })
}

export function deleteDevice(id: string): Promise<ApiResponse<null>> {
  return request<null>(`/api/devices/${id}`, {
    method: 'DELETE',
  })
}

export function setDefaultDevice(id: string): Promise<ApiResponse<null>> {
  return request<null>(`/api/devices/${id}/default`, {
    method: 'POST',
  })
}

// ─── Schedule Parsing (LLM) ──────────────────────────────────────────────────

export interface LLMSettings {
  base_url: string
  api_key: string
  model: string
  configured: boolean
}

export function getLLMSettings(): Promise<ApiResponse<LLMSettings>> {
  return request<LLMSettings>('/api/settings/llm')
}

export function saveLLMSettings(data: { base_url?: string; api_key?: string; model?: string }): Promise<ApiResponse<null>> {
  return request<null>('/api/settings/llm', {
    method: 'POST',
    body: JSON.stringify(data),
  })
}

export function fetchLLMModels(baseUrl?: string, apiKey?: string): Promise<ApiResponse<string[]>> {
  const params = new URLSearchParams()
  if (baseUrl) params.set('base_url', baseUrl)
  if (apiKey) params.set('api_key', apiKey)
  const qs = params.toString()
  return request<string[]>(`/api/settings/llm/models${qs ? '?' + qs : ''}`)
}

export interface ScheduleConfig {
  type: string
  config: Record<string, unknown>
  timezone?: string
  start_time?: string
  end_time?: string
  raw_input?: string
}

export interface ScheduleParseResult {
  title: string
  schedule: ScheduleConfig
}

export function parseSchedule(input: string, timezone?: string): Promise<ApiResponse<ScheduleParseResult>> {
  return request<ScheduleParseResult>('/api/schedule/parse', {
    method: 'POST',
    body: JSON.stringify({ input, timezone: timezone || 'Asia/Shanghai' }),
  })
}

// ─── Channels ────────────────────────────────────────────────────────────────

export interface Channel {
  id: string
  name: string
  sound: string
  group: string
  icon: string
  level: string
  device_keys: string[]
  created_at: string
  updated_at: string
}

export function fetchChannels(): Promise<ApiResponse<Channel[]>> {
  return request<Channel[]>('/api/channels')
}

export function createChannel(data: Partial<Channel>): Promise<ApiResponse<Channel>> {
  return request<Channel>('/api/channels', {
    method: 'POST',
    body: JSON.stringify(data),
  })
}

export function updateChannel(id: string, data: Partial<Channel>): Promise<ApiResponse<null>> {
  return request<null>(`/api/channels/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  })
}

export function deleteChannel(id: string): Promise<ApiResponse<null>> {
  return request<null>(`/api/channels/${id}`, {
    method: 'DELETE',
  })
}
