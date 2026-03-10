import client from './client'
import type { ApiResponse, Media } from './mediaApi'

export interface Collection {
  id: string
  user_id: string
  name: string
  description: string
  auto_type?: '' | 'video' | 'image' | 'manga' | 'comic' | 'doujinshi'
  auto_title?: string
  auto_tag?: string
  auto_favorite?: boolean | null
  auto_min_rating?: number | null
  created_at: string
  updated_at: string
  item_count: number
  items?: Media[]
}

export const collectionApi = {
  list: () =>
    client.get<ApiResponse<Collection[]>>('/collections').then((r) => r.data),

  get: (id: string) =>
    client.get<ApiResponse<Collection>>(`/collections/${id}`).then((r) => r.data),

  create: (body: { name: string; description?: string; auto_type?: string; auto_title?: string; auto_tag?: string; auto_favorite?: boolean | null; auto_min_rating?: number | null }) =>
    client.post<ApiResponse<Collection>>('/collections', body).then((r) => r.data),

  update: (id: string, body: { name?: string; description?: string; auto_type?: string; auto_title?: string; auto_tag?: string; auto_favorite?: boolean | null; auto_min_rating?: number | null }) =>
    client.patch<ApiResponse<Collection>>(`/collections/${id}`, body).then((r) => r.data),

  remove: (id: string) =>
    client.delete(`/collections/${id}`),

  addMedia: (id: string, mediaId: string) =>
    client.post<ApiResponse<Collection>>(`/collections/${id}/media`, { media_id: mediaId }).then((r) => r.data),

  removeMedia: (id: string, mediaId: string) =>
    client.delete<ApiResponse<Collection>>(`/collections/${id}/media/${mediaId}`).then((r) => r.data),

  listForMedia: (mediaId: string) =>
    client.get<ApiResponse<Collection[]>>(`/media/${mediaId}/collections`).then((r) => r.data),
}
