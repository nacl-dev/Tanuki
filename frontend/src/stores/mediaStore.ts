import { defineStore } from 'pinia'
import { ref, reactive, computed } from 'vue'
import { mediaApi, type Media, type MediaListParams } from '@/api/mediaApi'

export const useMediaStore = defineStore('media', () => {
  const items = ref<Media[]>([])
  const total = ref(0)
  const loading = ref(false)
  const error = ref<string | null>(null)

  const filters = reactive<MediaListParams>({
    page: 1,
    limit: 50,
    type: '',
    q: '',
    favorite: undefined,
    tag: '',
    sort: 'newest',
    min_rating: undefined,
  })

  const totalPages = computed(() => Math.max(1, Math.ceil(total.value / (filters.limit ?? 50))))
  const currentPage = computed(() => filters.page ?? 1)

  async function fetchList() {
    loading.value = true
    error.value = null
    try {
      const res = await mediaApi.list(filters)
      items.value = res.data ?? []
      total.value = res.meta?.total ?? 0
    } catch (e: any) {
      error.value = e.message
    } finally {
      loading.value = false
    }
  }

  async function toggleFavorite(id: string) {
    const item = items.value.find((m) => m.id === id)
    if (!item) return
    const updated = await mediaApi.update(id, { favorite: !item.favorite })
    const idx = items.value.findIndex((m) => m.id === id)
    if (idx !== -1) items.value[idx] = updated.data
  }

  async function setRating(id: string, rating: number) {
    const updated = await mediaApi.update(id, { rating })
    const idx = items.value.findIndex((m) => m.id === id)
    if (idx !== -1) items.value[idx] = updated.data
  }

  function setFilter<K extends keyof MediaListParams>(key: K, value: MediaListParams[K]) {
    filters[key] = value
    filters.page = 1
    fetchList()
  }

  function nextPage() {
    if ((filters.page ?? 1) < totalPages.value) {
      filters.page = (filters.page ?? 1) + 1
      fetchList()
    }
  }

  function prevPage() {
    if ((filters.page ?? 1) > 1) {
      filters.page = (filters.page ?? 1) - 1
      fetchList()
    }
  }

  async function saveProgress(id: string, progress: number, total: number) {
    await mediaApi.update(id, { read_progress: progress, read_total: total })
  }

  return {
    items, total, loading, error, filters,
    totalPages, currentPage,
    fetchList, toggleFavorite, setRating, setFilter, nextPage, prevPage, saveProgress,
  }
})
