import { createRouter, createWebHistory } from 'vue-router'
import LibraryPage from '@/pages/LibraryPage.vue'
import { useAuthStore } from '@/stores/authStore'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    // ─── Public auth routes ───────────────────────────────────────────────────
    {
      path: '/login',
      name: 'login',
      component: () => import('@/pages/LoginPage.vue'),
      meta: { requiresAuth: false },
    },
    {
      path: '/register',
      name: 'register',
      component: () => import('@/pages/RegisterPage.vue'),
      meta: { requiresAuth: false },
    },

    // ─── Protected routes ─────────────────────────────────────────────────────
    {
      path: '/',
      name: 'library',
      component: LibraryPage,
      meta: { requiresAuth: true },
    },
    {
      path: '/media/:id',
      name: 'media-detail',
      component: () => import('@/pages/MediaDetailPage.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/downloads',
      name: 'downloads',
      component: () => import('@/pages/DownloadsPage.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/tags',
      name: 'tags',
      component: () => import('@/pages/TagsPage.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/collections',
      name: 'collections',
      component: () => import('@/pages/CollectionsPage.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/duplicates',
      name: 'duplicates',
      component: () => import('@/pages/DuplicatesPage.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/plugins',
      name: 'plugins',
      component: () => import('@/pages/PluginsPage.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/settings',
      name: 'settings',
      component: () => import('@/pages/SettingsPage.vue'),
      meta: { requiresAuth: true },
    },
  ],
})

// ─── Navigation guard ─────────────────────────────────────────────────────────
router.beforeEach(async (to) => {
  const authStore = useAuthStore()

  // Wait for the store to finish rehydrating on first load
  if (authStore.token && !authStore.user) {
    await authStore.fetchMe()
  }

  const requiresAuth = to.meta.requiresAuth !== false

  if (requiresAuth && !authStore.isAuthenticated) {
    return { name: 'login' }
  }

  if (!requiresAuth && authStore.isAuthenticated) {
    return { name: 'library' }
  }
})

export default router
