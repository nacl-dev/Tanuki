import client from './client'
import type { Tag, ApiResponse } from './mediaApi'

export interface TagAlias {
  id: string
  alias_name: string
  tag_id: string
  tag: Tag
  created_at: string
}

export interface TagImplication {
  id: string
  tag_id: string
  implied_tag_id: string
  tag: Tag
  implied_tag: Tag
  created_at: string
}

export interface TagMergePreview {
  source: Tag
  target: Tag
  target_created: boolean
  source_media_count: number
  target_media_count: number
  overlapping_media_count: number
  source_alias_count: number
  source_outbound_implications: number
  source_inbound_implications: number
}

export const tagApi = {
  list: (category?: string) =>
    client.get<ApiResponse<Tag[]>>('/tags', { params: { category } }).then((r) => r.data),

  search: (q: string) =>
    client.get<ApiResponse<Tag[]>>('/tags/search', { params: { q } }).then((r) => r.data),

  create: (name: string, category: Tag['category']) =>
    client.post<ApiResponse<Tag>>('/tags', { name, category }).then((r) => r.data),

  update: (id: string, body: Partial<Pick<Tag, 'name' | 'category'>>) =>
    client.patch<ApiResponse<Tag>>(`/tags/${id}`, body).then((r) => r.data),

  remove: (id: string) => client.delete(`/tags/${id}`),

  listAliases: () =>
    client.get<ApiResponse<TagAlias[]>>('/tags/aliases').then((r) => r.data),

  createAlias: (aliasName: string, target: string) =>
    client.post<ApiResponse<TagAlias>>('/tags/aliases', {
      alias_name: aliasName,
      target,
    }).then((r) => r.data),

  removeAlias: (id: string) => client.delete(`/tags/aliases/${id}`),

  listImplications: () =>
    client.get<ApiResponse<TagImplication[]>>('/tags/implications').then((r) => r.data),

  createImplication: (source: string, implied: string) =>
    client.post<ApiResponse<TagImplication>>('/tags/implications', {
      source,
      implied,
    }).then((r) => r.data),

  removeImplication: (id: string) => client.delete(`/tags/implications/${id}`),

  previewMerge: (source: string, target: string) =>
    client.post<ApiResponse<TagMergePreview>>('/tags/merge/preview', {
      source,
      target,
    }).then((r) => r.data),

  merge: (source: string, target: string, createAlias = true) =>
    client.post<ApiResponse<{
      source: Tag
      target: Tag
      preview: TagMergePreview
      created_alias: boolean
      moved_media_tags: number
    }>>('/tags/merge', {
      source,
      target,
      create_alias: createAlias,
    }).then((r) => r.data),
}
