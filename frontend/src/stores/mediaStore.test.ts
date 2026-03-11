import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useMediaStore } from './mediaStore'
import { mediaApi } from '@/api/mediaApi'

vi.mock('@/api/mediaApi', () => ({
  mediaApi: {
    list: vi.fn(),
    update: vi.fn(),
  },
}))

describe('mediaStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    vi.mocked(mediaApi.list).mockResolvedValue({
      data: [],
      meta: { total: 0 },
    })
  })

  it('resets the page and fetches when a filter changes', async () => {
    const store = useMediaStore()
    store.filters.page = 4

    store.setFilter('type', 'video')

    expect(store.filters.type).toBe('video')
    expect(store.filters.page).toBe(1)
    expect(mediaApi.list).toHaveBeenCalledWith(expect.objectContaining({
      type: 'video',
      page: 1,
    }))
  })

  it('ignores stale list responses from earlier requests', async () => {
    let resolveFirst: ((value: { data: any[]; meta: { total: number } }) => void) | undefined
    let resolveSecond: ((value: { data: any[]; meta: { total: number } }) => void) | undefined

    vi.mocked(mediaApi.list)
      .mockImplementationOnce(
        () => new Promise((resolve) => { resolveFirst = resolve }),
      )
      .mockImplementationOnce(
        () => new Promise((resolve) => { resolveSecond = resolve }),
      )

    const store = useMediaStore()
    const firstRequest = store.fetchList()
    store.filters.q = 'latest'
    const secondRequest = store.fetchList()

    resolveSecond?.({
      data: [
        {
          id: 'newest',
          title: 'Newest',
          type: 'video',
          file_path: '/media/newest.mp4',
          file_size: 1,
          checksum: 'checksum-new',
          rating: 0,
          favorite: false,
          view_count: 0,
          language: '',
          source_url: '',
          thumbnail_path: '',
          read_progress: 0,
          read_total: 0,
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        },
      ],
      meta: { total: 1 },
    })
    await secondRequest

    resolveFirst?.({
      data: [
        {
          id: 'stale',
          title: 'Stale',
          type: 'video',
          file_path: '/media/stale.mp4',
          file_size: 1,
          checksum: 'checksum-old',
          rating: 0,
          favorite: false,
          view_count: 0,
          language: '',
          source_url: '',
          thumbnail_path: '',
          read_progress: 0,
          read_total: 0,
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        },
      ],
      meta: { total: 1 },
    })
    await firstRequest

    expect(store.items).toHaveLength(1)
    expect(store.items[0]?.id).toBe('newest')
    expect(store.total).toBe(1)
  })
})
