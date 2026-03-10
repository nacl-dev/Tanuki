import client from './client'

export interface DuplicateItem {
  id: string
  title: string
  type: string
  file_size: number
  thumbnail_path: string
  similarity: number
  distance: number
}

export interface DuplicateGroup {
  group_id: number
  reference: DuplicateItem
  matches: DuplicateItem[]
  count: number
}

export interface ApiResponse<T> {
  data: T
  error?: string
  meta?: { total: number }
}

export const dedupApi = {
  /** Get duplicate items for a specific media item. */
  getDuplicatesForMedia: (mediaId: string) =>
    client
      .get<ApiResponse<DuplicateItem[]>>(`/media/${mediaId}/duplicates`)
      .then((r) => r.data),

  /** List all duplicate groups across the library. */
  getAllDuplicateGroups: () =>
    client
      .get<ApiResponse<DuplicateGroup[]>>('/duplicates')
      .then((r) => r.data),

  /** Resolve a duplicate group: keep one item, soft-delete the rest. */
  resolveDuplicates: (keepId: string, deleteIds: string[], mergeTags: boolean) =>
    client
      .post<ApiResponse<{ deleted: number; kept: string }>>('/duplicates/resolve', {
        keep_id: keepId,
        delete_ids: deleteIds,
        merge_tags: mergeTags,
      })
      .then((r) => r.data),

  /** Trigger pHash computation for a single item. */
  computePHash: (mediaId: string) =>
    client
      .post<ApiResponse<{ id: string; phash: number | null }>>(`/media/${mediaId}/phash`, {})
      .then((r) => r.data),
}
