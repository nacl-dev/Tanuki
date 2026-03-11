import client from './client'

export interface PathHealth {
  path: string
  exists: boolean
  is_dir?: boolean
  writable?: boolean
  error?: string
}

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
  media_path?: string
  downloads_path?: string
  thumbnails_path?: string
  inbox_path?: string
  path_health: Record<string, PathHealth>
  scan_interval?: number
  max_concurrent_downloads?: number
  rate_limit_delay?: number
  plugins_enabled: boolean
  registration_enabled: boolean
  runtime_details_visible: boolean
  library_scope: string
  tag_scope: string
  collection_scope: string
  download_scope: string
  schedule_scope: string
  owner_mode: string
}

export const systemApi = {
  info: () =>
    client.get<{ data: SystemInfo }>('/system/info').then((r) => r.data.data),
}
