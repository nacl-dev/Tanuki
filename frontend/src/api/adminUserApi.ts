import client from './client'
import type { User } from './authApi'

export const adminUserApi = {
  list: () =>
    client.get<{ data: User[] }>('/admin/users').then((r) => r.data.data),

  update: (
    id: string,
    body: Partial<Pick<User, 'display_name' | 'email' | 'role' | 'is_active'>>,
  ) =>
    client.patch<{ data: User }>(`/admin/users/${id}`, body).then((r) => r.data.data),

  remove: (id: string) =>
    client.delete<{ data: { deleted: boolean } }>(`/admin/users/${id}`).then((r) => r.data.data),
}
