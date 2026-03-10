import client from './client'
import type { ApiResponse } from './mediaApi'

export interface OrganizeLibraryResult {
  source_path: string
  mode: 'move' | 'copy'
  moved: number
  skipped: number
  preview: boolean
  items?: OrganizePreviewItem[]
}

export interface OrganizePreviewItem {
  source_path: string
  target_path: string
  media_type: string
  action: 'move' | 'copy'
  skipped: boolean
  reason?: string
}

export const libraryApi = {
  scan: () =>
    client.post<ApiResponse<{ message: string }>>('/library/scan').then((r) => r.data),

  organize: (sourcePath: string, mode: 'move' | 'copy' = 'move', preview = false) =>
    client.post<ApiResponse<OrganizeLibraryResult>>('/library/organize', {
      source_path: sourcePath,
      mode,
      preview,
    }).then((r) => r.data),
}
