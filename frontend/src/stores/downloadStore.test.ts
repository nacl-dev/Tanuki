import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useDownloadStore } from './downloadStore'
import { downloadApi } from '@/api/downloadApi'

vi.mock('@/api/downloadApi', () => ({
  downloadApi: {
    list: vi.fn(),
    listSchedules: vi.fn(),
    createSchedule: vi.fn(),
    updateSchedule: vi.fn(),
    removeSchedule: vi.fn(),
    create: vi.fn(),
    batch: vi.fn(),
    update: vi.fn(),
    remove: vi.fn(),
  },
}))

describe('downloadStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    vi.mocked(downloadApi.list).mockResolvedValue({ data: [] })
  })

  it('treats processing jobs as active for polling', () => {
    const store = useDownloadStore()
    store.jobs = [
      {
        id: 'job-1',
        url: 'https://example.com/a',
        source_type: 'auto',
        status: 'processing',
        progress: 90,
        total_files: 1,
        downloaded_files: 1,
        total_bytes: 1,
        downloaded_bytes: 1,
        target_directory: '/media',
        retry_count: 0,
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      },
    ]

    expect(store.activeJobs()).toHaveLength(1)
    expect(store.activeJobs()[0]?.status).toBe('processing')
  })

  it('replaces jobs from a live snapshot', () => {
    const store = useDownloadStore()
    store.replaceJobs([
      {
        id: 'job-live',
        url: 'https://example.com/live',
        source_type: 'auto',
        status: 'queued',
        progress: 0,
        total_files: 0,
        downloaded_files: 0,
        total_bytes: 0,
        downloaded_bytes: 0,
        target_directory: '/media',
        retry_count: 0,
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      },
    ])

    expect(store.jobs).toHaveLength(1)
    expect(store.jobs[0]?.id).toBe('job-live')
  })

  it('passes auto tags through enqueue', async () => {
    const store = useDownloadStore()
    vi.mocked(downloadApi.create).mockResolvedValue({
      data: {
        id: 'job-tags',
        url: 'https://example.com/tagged',
        source_type: 'auto',
        status: 'queued',
        progress: 0,
        total_files: 0,
        downloaded_files: 0,
        total_bytes: 0,
        downloaded_bytes: 0,
        target_directory: '/media',
        auto_tags: ['artist:Foo'],
        retry_count: 0,
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      },
    })

    await store.enqueue({
      url: 'https://example.com/tagged',
      target_directory: '/media',
      auto_tags: ['artist:Foo'],
    })

    expect(downloadApi.create).toHaveBeenCalledWith({
      url: 'https://example.com/tagged',
      target_directory: '/media',
      auto_tags: ['artist:Foo'],
    })
  })

  it('passes auto tags through batch enqueue', async () => {
    const store = useDownloadStore()
    vi.mocked(downloadApi.batch).mockResolvedValue({ data: { created: ['job-1'] } })

    await store.enqueueBatch(['https://example.com/a'], '/media', ['series:Bar'])

    expect(downloadApi.batch).toHaveBeenCalledWith(['https://example.com/a'], '/media', ['series:Bar'])
  })

  it('passes default tags through schedule creation', async () => {
    const store = useDownloadStore()
    vi.mocked(downloadApi.createSchedule).mockResolvedValue({
      data: {
        id: 'sched-1',
        name: 'Nightly source',
        url_pattern: 'https://example.com/source',
        source_type: 'auto',
        cron_expression: '0 3 * * *',
        enabled: true,
        default_tags: ['artist:Foo'],
        target_directory: '/media',
        created_at: new Date().toISOString(),
      },
    })

    await store.createSchedule({
      name: 'Nightly source',
      url_pattern: 'https://example.com/source',
      source_type: 'auto',
      cron_expression: '0 3 * * *',
      enabled: true,
      default_tags: ['artist:Foo'],
      target_directory: '/media',
    })

    expect(downloadApi.createSchedule).toHaveBeenCalledWith({
      name: 'Nightly source',
      url_pattern: 'https://example.com/source',
      source_type: 'auto',
      cron_expression: '0 3 * * *',
      enabled: true,
      default_tags: ['artist:Foo'],
      target_directory: '/media',
    })
  })
})
