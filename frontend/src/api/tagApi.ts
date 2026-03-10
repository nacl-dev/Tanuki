import client from './client'
import type { Tag, ApiResponse } from './mediaApi'

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
}
