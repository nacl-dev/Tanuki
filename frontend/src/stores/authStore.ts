import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { authApi, type User } from '@/api/authApi'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const hydrated = ref(false)

  const isAuthenticated = computed(() => !!user.value)
  const isAdmin = computed(() => user.value?.role === 'admin')

  async function login(username: string, password: string) {
    const data = await authApi.login({ username, password })
    user.value = data.user
    hydrated.value = true
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

  async function logout() {
    await authApi.logout()
    user.value = null
    hydrated.value = true
  }

  async function fetchMe(force = false) {
    if (hydrated.value && !force) return
    try {
      user.value = await authApi.me()
    } catch {
      user.value = null
    } finally {
      hydrated.value = true
    }
  }

  async function updateProfile(
    body: Partial<Pick<User, 'display_name' | 'email'>> & { password?: string },
  ) {
    user.value = await authApi.updateProfile(body)
  }

  return {
    user,
    hydrated,
    isAuthenticated,
    isAdmin,
    login,
    register,
    logout,
    fetchMe,
    updateProfile,
  }
})
