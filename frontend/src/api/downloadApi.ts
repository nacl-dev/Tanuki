import client from './client'
import type { ApiResponse } from './mediaApi'

export interface DownloadJob {
  id: string
  url: string
  source_type: string
  status: 'queued' | 'downloading' | 'processing' | 'completed' | 'failed' | 'paused'
  progress: number
  total_files: number
  downloaded_files: number
  total_bytes: number
  downloaded_bytes: number
  target_directory: string
  error_message?: string
  retry_count: number
  created_at: string
  updated_at: string
  completed_at?: string
}

export interface DownloadSchedule {
  id: string
  name: string
  url_pattern: string
  source_type: string
  cron_expression: string
  enabled: boolean
  target_directory: string
  last_run?: string
  next_run?: string
  created_at: string
}

export interface CreateDownloadInput {
  url: string
  target_directory?: string
  auto_tags?: string[]
}

export const downloadApi = {
  list: (status?: string) =>
    client.get<ApiResponse<DownloadJob[]>>('/downloads', { params: { status } }).then((r) => r.data),

  get: (id: string) =>
    client.get<ApiResponse<DownloadJob>>(`/downloads/${id}`).then((r) => r.data),

  create: (input: CreateDownloadInput) =>
    client.post<ApiResponse<DownloadJob>>('/downloads', input).then((r) => r.data),

  batch: (urls: string[], target_directory?: string) =>
    client.post<ApiResponse<{ created: string[] }>>('/downloads/batch', { urls, target_directory }).then((r) => r.data),

  update: (id: string, action: 'pause' | 'resume' | 'cancel' | 'retry') =>
    client.patch<ApiResponse<DownloadJob>>(`/downloads/${id}`, { action }).then((r) => r.data),

  remove: (id: string) => client.delete(`/downloads/${id}`),

  // Schedules
  listSchedules: () =>
    client.get<ApiResponse<DownloadSchedule[]>>('/schedules').then((r) => r.data),

  createSchedule: (input: Omit<DownloadSchedule, 'id' | 'created_at' | 'last_run' | 'next_run'>) =>
    client.post<ApiResponse<DownloadSchedule>>('/schedules', input).then((r) => r.data),

  updateSchedule: (id: string, body: Partial<DownloadSchedule>) =>
    client.patch<ApiResponse<DownloadSchedule>>(`/schedules/${id}`, body).then((r) => r.data),

  removeSchedule: (id: string) => client.delete(`/schedules/${id}`),
}
