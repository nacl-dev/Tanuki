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
})
