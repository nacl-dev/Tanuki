import client from './client'

export interface User {
  id: string
  username: string
  email: string
  display_name: string
  role: 'admin' | 'user'
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface LoginResponse {
  token: string
  user: User
}

export const authApi = {
  register: (input: {
    username: string
    email: string
    password: string
    display_name?: string
  }) =>
    client
      .post<{ data: User }>('/auth/register', input)
      .then((r) => r.data.data),

  login: (input: { username: string; password: string }) =>
    client
      .post<{ data: LoginResponse }>('/auth/login', input)
      .then((r) => r.data.data),

  logout: () =>
    client.post<{ data: { logged_out: boolean } }>('/auth/logout').then((r) => r.data.data),

  me: () =>
    client.get<{ data: User }>('/auth/me').then((r) => r.data.data),

  updateProfile: (
    body: Partial<Pick<User, 'display_name' | 'email'>> & { password?: string },
  ) =>
    client
      .patch<{ data: User }>('/auth/me', body)
      .then((r) => r.data.data),
}
