import client, { appPath } from './client'

export type BackgroundTaskStatus = 'queued' | 'running' | 'completed' | 'failed'

export interface BackgroundTask {
  id: string
  kind: string
  status: BackgroundTaskStatus
  message?: string
  error?: string
  requested_by?: string
  completed: number
  total: number
  percent: number
  metadata?: Record<string, unknown>
  result?: Record<string, unknown> | null
  created_at: string
  started_at?: string | null
  finished_at?: string | null
}

export const taskApi = {
  list: (limit = 20) =>
    client.get<{ data: BackgroundTask[] }>(`/tasks?limit=${limit}`).then((r) => r.data.data),

  streamUrl: (limit = 20) => appPath(`/api/tasks/stream?limit=${limit}`),

  get: (id: string) =>
    client.get<{ data: BackgroundTask }>(`/tasks/${id}`).then((r) => r.data.data),
}
