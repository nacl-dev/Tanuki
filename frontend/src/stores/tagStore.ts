import { defineStore } from 'pinia'
import { ref } from 'vue'
import { tagApi, type TagAlias, type TagImplication, type TagMergePreview } from '@/api/tagApi'
import type { Tag } from '@/api/mediaApi'

export const useTagStore = defineStore('tag', () => {
  const tags = ref<Tag[]>([])
  const aliases = ref<TagAlias[]>([])
  const implications = ref<TagImplication[]>([])
  const mergePreview = ref<TagMergePreview | null>(null)
  const autocompleteCache = ref<Map<string, Tag[]>>(new Map())
  const loading = ref(false)
  const rulesLoading = ref(false)
  const mergeLoading = ref(false)

  function resetAutocomplete() {
    autocompleteCache.value = new Map()
  }

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
    resetAutocomplete()
    return res.data
  }

  async function fetchRules() {
    rulesLoading.value = true
    try {
      const [aliasRes, implicationRes] = await Promise.all([
        tagApi.listAliases(),
        tagApi.listImplications(),
      ])
      aliases.value = aliasRes.data ?? []
      implications.value = implicationRes.data ?? []
    } finally {
      rulesLoading.value = false
    }
  }

  async function createAlias(aliasName: string, target: string) {
    const res = await tagApi.createAlias(aliasName, target)
    aliases.value = [...aliases.value, res.data].sort((a, b) => a.alias_name.localeCompare(b.alias_name))
    resetAutocomplete()
    return res.data
  }

  async function removeAlias(id: string) {
    await tagApi.removeAlias(id)
    aliases.value = aliases.value.filter((rule) => rule.id !== id)
    resetAutocomplete()
  }

  async function createImplication(source: string, implied: string) {
    const res = await tagApi.createImplication(source, implied)
    implications.value = [res.data, ...implications.value.filter((rule) => rule.id !== res.data.id)]
    return res.data
  }

  async function removeImplication(id: string) {
    await tagApi.removeImplication(id)
    implications.value = implications.value.filter((rule) => rule.id !== id)
  }

  async function remove(id: string) {
    await tagApi.remove(id)
    tags.value = tags.value.filter((t) => t.id !== id)
    aliases.value = aliases.value.filter((rule) => rule.tag_id !== id)
    implications.value = implications.value.filter((rule) => rule.tag_id !== id && rule.implied_tag_id !== id)
    resetAutocomplete()
  }

  async function previewMerge(source: string, target: string) {
    mergeLoading.value = true
    try {
      const res = await tagApi.previewMerge(source, target)
      mergePreview.value = res.data
      return res.data
    } finally {
      mergeLoading.value = false
    }
  }

  function clearMergePreview() {
    mergePreview.value = null
  }

  async function merge(source: string, target: string, createAlias = true) {
    mergeLoading.value = true
    try {
      const res = await tagApi.merge(source, target, createAlias)
      mergePreview.value = null
      await Promise.all([
        fetchAll(),
        fetchRules(),
      ])
      resetAutocomplete()
      return res.data
    } finally {
      mergeLoading.value = false
    }
  }

  return {
    tags,
    aliases,
    implications,
    mergePreview,
    loading,
    rulesLoading,
    mergeLoading,
    fetchAll,
    fetchRules,
    search,
    create,
    createAlias,
    removeAlias,
    createImplication,
    removeImplication,
    remove,
    previewMerge,
    clearMergePreview,
    merge,
  }
})
