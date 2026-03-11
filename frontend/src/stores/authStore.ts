import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { authApi, type User } from '@/api/authApi'

const TOKEN_KEY = 'tanuki_token'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const token = ref<string | null>(localStorage.getItem(TOKEN_KEY))

  const isAuthenticated = computed(() => !!token.value && !!user.value)
  const isAdmin = computed(() => user.value?.role === 'admin')

  async function login(username: string, password: string) {
    const data = await authApi.login({ username, password })
    token.value = data.token
    user.value = data.user
    localStorage.setItem(TOKEN_KEY, data.token)
  }

  async function register(input: {
    username: string
    email: string
    password: string
    display_name?: string
  }) {
    const newUser = await authApi.register(input)
    // After registration, log the user in automatically
    await login(input.username, input.password)
    return newUser
  }

  function logout() {
    void authApi.logout().catch(() => {})
    user.value = null
    token.value = null
    localStorage.removeItem(TOKEN_KEY)
  }

  async function fetchMe() {
    if (!token.value) return
    try {
      user.value = await authApi.me()
    } catch {
      // Token is invalid or expired – clear it
      logout()
    }
  }

  async function updateProfile(
    body: Partial<Pick<User, 'display_name' | 'email'>> & { password?: string },
  ) {
    user.value = await authApi.updateProfile(body)
  }

  return {
    user,
    token,
    isAuthenticated,
    isAdmin,
    login,
    register,
    logout,
    fetchMe,
    updateProfile,
  }
})
