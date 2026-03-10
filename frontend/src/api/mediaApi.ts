import client from './client'

export interface Media {
  id: string
  title: string
  type: 'video' | 'image' | 'manga' | 'comic' | 'doujinshi'
  file_path: string
  file_size: number
  checksum: string
  rating: number
  favorite: boolean
  view_count: number
  language: string
  source_url: string
  thumbnail_path: string
  read_progress: number
  read_total: number
  tags?: Tag[]
  created_at: string
  updated_at: string
}

export interface Tag {
  id: string
  name: string
  category: 'general' | 'artist' | 'character' | 'parody' | 'genre' | 'meta'
  usage_count: number
}

export interface PageInfo {
  index: number
  filename: string
}

export interface PagesResponse {
  total_pages: number
  pages: PageInfo[]
}

export interface MediaListParams {
  page?: number
  limit?: number
  type?: string
  q?: string
  favorite?: boolean
  tag?: string
  tags?: string
  sort?: string
  min_rating?: number
}

export interface ApiResponse<T> {
  data: T
  error?: string
  meta?: { page?: number; total: number }
}

export const mediaApi = {
  list: (params: MediaListParams = {}) =>
    client.get<ApiResponse<Media[]>>('/media', { params }).then((r) => r.data),

  get: (id: string) =>
    client.get<ApiResponse<Media>>(`/media/${id}`).then((r) => r.data),

  update: (id: string, body: Partial<Pick<Media, 'title' | 'rating' | 'favorite' | 'language' | 'source_url' | 'read_progress' | 'read_total'>>) =>
    client.patch<ApiResponse<Media>>(`/media/${id}`, body).then((r) => r.data),

  remove: (id: string) => client.delete(`/media/${id}`),

  getPages: (id: string) =>
    client.get<ApiResponse<PagesResponse>>(`/media/${id}/pages`).then((r) => r.data),
}
