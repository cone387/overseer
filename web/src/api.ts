const API_KEY_STORAGE_KEY = 'overseer_api_key'

export function getApiKey(): string {
  return localStorage.getItem(API_KEY_STORAGE_KEY) || ''
}

export function setApiKey(key: string): void {
  localStorage.setItem(API_KEY_STORAGE_KEY, key)
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

async function request<T>(
  path: string,
  options: RequestInit = {}
): Promise<ApiResponse<T>> {
  const apiKey = getApiKey()
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(apiKey ? { 'X-API-Key': apiKey } : {}),
    ...(options.headers as Record<string, string> || {}),
  }

  const res = await fetch(path, {
    ...options,
    headers,
  })

  const json = await res.json()
  if (!res.ok) {
    throw new ApiError(json.code || res.status, json.message || res.statusText)
  }
  return json as ApiResponse<T>
}

export class ApiError extends Error {
  code: number
  constructor(code: number, message: string) {
    super(message)
    this.code = code
    this.name = 'ApiError'
  }
}

// Stats
export interface ChannelStat {
  channel: string
  total: number
  success: number
  failed: number
  percentage: number
}

export function fetchStats(from: string, to: string): Promise<ApiResponse<ChannelStat[]>> {
  return request<ChannelStat[]>(`/api/stats?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`)
}

// Messages
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

// Reminders
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

export function cancelReminder(id: string): Promise<ApiResponse<null>> {
  return request<null>(`/api/reminders/${id}`, {
    method: 'DELETE',
  })
}

// Push
export interface PushRequest {
  title: string
  body: string
  channel?: string
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
