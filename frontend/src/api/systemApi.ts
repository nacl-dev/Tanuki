import client from './client'

export interface SystemInfo {
  version: string
  media_count: number
  plugin_count: number
  downloads_total: number
  downloads_active: number
  downloads_failed: number
  schedules_total: number
  schedules_enabled: number
  autotag_pending: number
  background_tasks_active: number
  background_tasks_failed: number
  last_completed_download?: string | null
  media_path: string
  downloads_path: string
  thumbnails_path: string
  inbox_path: string
  scan_interval: number
  max_concurrent_downloads: number
  rate_limit_delay: number
  plugins_enabled: boolean
  registration_enabled: boolean
}

export const systemApi = {
  info: () =>
    client.get<{ data: SystemInfo }>('/system/info').then((r) => r.data.data),
}
