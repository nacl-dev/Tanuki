import client from './client'
import type { Tag } from './mediaApi'

export interface SuggestedTag {
  name: string
  category: 'general' | 'artist' | 'character' | 'parody' | 'genre' | 'meta'
  confidence: number
}

export interface AutoTagResult {
  suggested_tags: SuggestedTag[]
  source: 'saucenao' | 'iqdb' | 'none'
  similarity: number
  source_url?: string
}

export interface ApiResponse<T> {
  data: T
  error?: string
}

export const autotagApi = {
  /**
   * Auto-tag a single media item via reverse image search.
   * Returns suggested tags with confidence scores.
   */
  autotagSingle: (mediaId: string, force = false, applyTags?: SuggestedTag[]) =>
    client
      .post<ApiResponse<AutoTagResult>>(`/media/${mediaId}/autotag`, {
        force,
        ...(applyTags ? { apply_tags: applyTags } : {}),
      })
      .then((r) => r.data),

  /**
   * Queue batch auto-tagging.
   * Pass ids array or set allUntagged=true to tag all pending items.
   */
  autotagBatch: (ids: string[] | 'all_untagged') =>
    client
      .post<ApiResponse<{ queued: number; task_id: string }>>('/media/autotag/batch', {
        ...(ids === 'all_untagged' ? { all_untagged: true } : { ids }),
      })
      .then((r) => r.data),
}

// Re-export Tag for convenience
export type { Tag }
