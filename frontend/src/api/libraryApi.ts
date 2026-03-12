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

export interface InboxUploadFile {
  file: File
  relativePath?: string
}

export interface InboxUploadResult {
  batch_name: string
  source_path: string
  file_count: number
  total_bytes: number
  default_tags?: string[]
}

export const libraryApi = {
  scan: () =>
    client.post<ApiResponse<{ message: string; task_id: string }>>('/library/scan').then((r) => r.data),

  organize: (sourcePath: string, mode: 'move' | 'copy' = 'move', preview = false) =>
    client.post<ApiResponse<OrganizeLibraryResult | { message: string; task_id: string }>>('/library/organize', {
      source_path: sourcePath,
      mode,
      preview,
    }).then((r) => r.data),

  uploadInbox: (files: InboxUploadFile[], batchName?: string, defaultTags?: string[]) => {
    const formData = new FormData()
    if (batchName?.trim()) {
      formData.append('batch_name', batchName.trim())
    }
    for (const tag of defaultTags ?? []) {
      if (tag.trim()) {
        formData.append('default_tags', tag.trim())
      }
    }

    for (const entry of files) {
      formData.append('files', entry.file, entry.file.name)
      formData.append('paths', entry.relativePath?.trim() || entry.file.name)
    }

    return client.post<ApiResponse<InboxUploadResult>>('/library/inbox/upload', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    }).then((r) => r.data)
  },
}
