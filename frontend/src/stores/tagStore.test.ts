import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useTagStore } from './tagStore'
import { tagApi } from '@/api/tagApi'

vi.mock('@/api/tagApi', () => ({
  tagApi: {
    list: vi.fn(),
    search: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    remove: vi.fn(),
    listAliases: vi.fn(),
    createAlias: vi.fn(),
    removeAlias: vi.fn(),
    listImplications: vi.fn(),
    createImplication: vi.fn(),
    removeImplication: vi.fn(),
    previewMerge: vi.fn(),
    merge: vi.fn(),
  },
}))

describe('tagStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    vi.mocked(tagApi.list).mockResolvedValue({ data: [] })
    vi.mocked(tagApi.listAliases).mockResolvedValue({ data: [] })
    vi.mocked(tagApi.listImplications).mockResolvedValue({ data: [] })
  })

  it('stores merge preview impact counts', async () => {
    const store = useTagStore()
    vi.mocked(tagApi.previewMerge).mockResolvedValue({
      data: {
        source: { id: 'src', name: 'old', category: 'general', usage_count: 4 },
        target: { id: 'dst', name: 'new', category: 'artist', usage_count: 10 },
        target_created: false,
        source_media_count: 4,
        target_media_count: 10,
        overlapping_media_count: 1,
        source_alias_count: 2,
        source_outbound_implications: 1,
        source_inbound_implications: 3,
      },
    })

    const preview = await store.previewMerge('old', 'artist:new')

    expect(tagApi.previewMerge).toHaveBeenCalledWith('old', 'artist:new')
    expect(preview.overlapping_media_count).toBe(1)
    expect(store.mergePreview?.source_alias_count).toBe(2)
  })

  it('clears preview and refreshes state after merge', async () => {
    const store = useTagStore()
    store.mergePreview = {
      source: { id: 'src', name: 'old', category: 'general', usage_count: 4 },
      target: { id: 'dst', name: 'new', category: 'artist', usage_count: 10 },
      target_created: false,
      source_media_count: 4,
      target_media_count: 10,
      overlapping_media_count: 1,
      source_alias_count: 2,
      source_outbound_implications: 1,
      source_inbound_implications: 3,
    }
    vi.mocked(tagApi.merge).mockResolvedValue({
      data: {
        source: { id: 'src', name: 'old', category: 'general', usage_count: 4 },
        target: { id: 'dst', name: 'new', category: 'artist', usage_count: 10 },
        preview: store.mergePreview,
        created_alias: true,
        moved_media_tags: 3,
      },
    })

    const result = await store.merge('old', 'artist:new', true)

    expect(tagApi.merge).toHaveBeenCalledWith('old', 'artist:new', true)
    expect(result.moved_media_tags).toBe(3)
    expect(store.mergePreview).toBeNull()
  })
})
