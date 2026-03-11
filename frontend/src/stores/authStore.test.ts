import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAuthStore } from './authStore'
import { authApi } from '@/api/authApi'

vi.mock('@/api/authApi', () => ({
  authApi: {
    login: vi.fn(),
    register: vi.fn(),
    logout: vi.fn(),
    me: vi.fn(),
    updateProfile: vi.fn(),
  },
}))

describe('authStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('clears local auth state only after a successful logout response', async () => {
    vi.mocked(authApi.logout).mockResolvedValue({ logged_out: true })
    const store = useAuthStore()
    store.user = {
      id: 'user-1',
      username: 'tanuki',
      email: 'tanuki@example.com',
      display_name: 'Tanuki',
      role: 'user',
      is_active: true,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    }

    await store.logout()

    expect(authApi.logout).toHaveBeenCalledOnce()
    expect(store.user).toBeNull()
    expect(store.hydrated).toBe(true)
  })

  it('keeps the local session intact when logout fails', async () => {
    vi.mocked(authApi.logout).mockRejectedValue(new Error('network down'))
    const store = useAuthStore()
    store.user = {
      id: 'user-1',
      username: 'tanuki',
      email: 'tanuki@example.com',
      display_name: 'Tanuki',
      role: 'user',
      is_active: true,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    }

    await expect(store.logout()).rejects.toThrow('network down')
    expect(store.user?.id).toBe('user-1')
  })
})
