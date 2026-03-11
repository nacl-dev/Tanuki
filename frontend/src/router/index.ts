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
      path: '/capture',
      alias: ['/downloads'],
      name: 'capture',
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
      redirect: { name: 'settings', query: { section: 'duplicates' } },
      meta: { requiresAuth: true },
    },
    {
      path: '/plugins',
      name: 'plugins',
      redirect: { name: 'settings', query: { section: 'plugins' } },
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

  if (!authStore.hydrated) {
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
