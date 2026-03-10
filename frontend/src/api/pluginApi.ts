import client from './client'
import type { ApiResponse } from './mediaApi'

export interface Plugin {
  id: string
  name: string
  source_name: string
  source_url: string
  file_path: string
  enabled: boolean
  version: string
  created_at: string
  updated_at: string
}

export const pluginApi = {
  list: () =>
    client.get<ApiResponse<Plugin[]>>('/plugins').then((r) => r.data),

  scan: () =>
    client.post<ApiResponse<Plugin[]>>('/plugins/scan').then((r) => r.data),

  toggle: (id: string, enabled: boolean) =>
    client.patch<ApiResponse<{ id: string; enabled: boolean }>>(`/plugins/${id}`, { enabled }).then((r) => r.data),

  remove: (id: string) =>
    client.delete(`/plugins/${id}`),
}
