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
  // Auto-tag fields (v0.4)
  auto_tag_status?: 'pending' | 'processing' | 'completed' | 'failed' | 'skipped'
  auto_tag_source?: string
  auto_tag_similarity?: number
  auto_tagged_at?: string
  // Perceptual hash (v0.5)
  phash?: number | null
  tags?: Tag[]
  collections?: CollectionRef[]
  created_at: string
  updated_at: string
}

export interface MediaUpdateBody {
  title?: string
  rating?: number
  favorite?: boolean
  language?: string
  source_url?: string
  created_at?: string
  tag_names?: string[]
  read_progress?: number
  read_total?: number
}

export interface ThumbnailFetchBody {
  url: string
}

export interface Tag {
  id: string
  name: string
  category: 'general' | 'artist' | 'character' | 'parody' | 'genre' | 'meta'
  usage_count: number
}

export interface CollectionRef {
  id: string
  name: string
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
  in_progress?: boolean
  tag?: string
  tags?: string
  sort?: string
  min_rating?: number
}

export interface SearchSuggestion {
  type: 'title' | 'tag'
  value: string
  label: string
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

  update: (id: string, body: MediaUpdateBody) =>
    client.patch<ApiResponse<Media>>(`/media/${id}`, body).then((r) => r.data),

  remove: (id: string, deleteFile = false) =>
    client.delete(`/media/${id}`, { data: { delete_file: deleteFile } }),

  uploadThumbnail: (id: string, file: File) => {
    const form = new FormData()
    form.append('thumbnail', file)
    return client.post<ApiResponse<Media>>(`/media/${id}/thumbnail/upload`, form, {
      headers: { 'Content-Type': 'multipart/form-data' },
    }).then((r) => r.data)
  },

  fetchThumbnail: (id: string, body: ThumbnailFetchBody) =>
    client.post<ApiResponse<Media>>(`/media/${id}/thumbnail/fetch`, body).then((r) => r.data),

  getPages: (id: string) =>
    client.get<ApiResponse<PagesResponse>>(`/media/${id}/pages`).then((r) => r.data),

  suggestions: (q: string) =>
    client.get<ApiResponse<SearchSuggestion[]>>('/media/suggestions', { params: { q } }).then((r) => r.data),
}

function withVersion(pathname: string, version?: string): string {
  const url = new URL(pathname, window.location.origin)
  if (version) {
    url.searchParams.set('v', version)
  }
  return url.pathname + url.search
}

export function mediaAssetUrl(id: string, kind: 'file' | 'thumbnail', version?: string): string {
  return withVersion(`/api/media/${id}/${kind}`, version)
}

export function mediaPageUrl(id: string, page: number): string {
  return withVersion(`/api/media/${id}/pages/${page}`)
}
