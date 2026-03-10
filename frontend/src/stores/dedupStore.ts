import { defineStore } from 'pinia'
import { ref } from 'vue'
import { dedupApi, type DuplicateGroup } from '@/api/dedupApi'

export const useDedupStore = defineStore('dedup', () => {
  const groups = ref<DuplicateGroup[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetchGroups() {
    loading.value = true
    error.value = null
    try {
      const res = await dedupApi.getAllDuplicateGroups()
      groups.value = res.data ?? []
    } catch (e: unknown) {
      error.value = e instanceof Error ? e.message : 'Failed to load duplicates'
    } finally {
      loading.value = false
    }
  }

  async function resolve(keepId: string, deleteIds: string[], mergeTags: boolean) {
    await dedupApi.resolveDuplicates(keepId, deleteIds, mergeTags)
    // Remove resolved group from local state
    groups.value = groups.value.filter((g) => {
      const allIds = [g.reference.id, ...g.matches.map((m) => m.id)]
      return !deleteIds.some((d) => allIds.includes(d))
    })
  }

  return { groups, loading, error, fetchGroups, resolve }
})
