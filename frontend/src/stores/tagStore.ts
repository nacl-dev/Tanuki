import { defineStore } from 'pinia'
import { ref } from 'vue'
import { tagApi } from '@/api/tagApi'
import type { Tag } from '@/api/mediaApi'

export const useTagStore = defineStore('tag', () => {
  const tags = ref<Tag[]>([])
  const autocompleteCache = ref<Map<string, Tag[]>>(new Map())
  const loading = ref(false)

  async function fetchAll(category?: string) {
    loading.value = true
    try {
      const res = await tagApi.list(category)
      tags.value = res.data ?? []
    } finally {
      loading.value = false
    }
  }

  async function search(q: string): Promise<Tag[]> {
    if (!q) return []
    const cached = autocompleteCache.value.get(q)
    if (cached) return cached

    const res = await tagApi.search(q)
    const items = res.data ?? []
    autocompleteCache.value.set(q, items)
    return items
  }

  async function create(name: string, category: Tag['category']) {
    const res = await tagApi.create(name, category)
    tags.value.push(res.data)
    return res.data
  }

  async function remove(id: string) {
    await tagApi.remove(id)
    tags.value = tags.value.filter((t) => t.id !== id)
  }

  return { tags, loading, fetchAll, search, create, remove }
})
