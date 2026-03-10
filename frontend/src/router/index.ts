import { createRouter, createWebHistory } from 'vue-router'
import LibraryPage from '@/pages/LibraryPage.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'library',
      component: LibraryPage,
    },
    {
      path: '/media/:id',
      name: 'media-detail',
      component: () => import('@/pages/MediaDetailPage.vue'),
    },
    {
      path: '/downloads',
      name: 'downloads',
      component: () => import('@/pages/DownloadsPage.vue'),
    },
    {
      path: '/tags',
      name: 'tags',
      component: () => import('@/pages/TagsPage.vue'),
    },
    {
      path: '/settings',
      name: 'settings',
      component: () => import('@/pages/SettingsPage.vue'),
    },
  ],
})

export default router
