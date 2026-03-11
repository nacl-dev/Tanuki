import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useNoticeStore } from './noticeStore'

describe('noticeStore', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    const { notices } = useNoticeStore()
    notices.value = []
  })

  it('adds and auto-removes timed notices', () => {
    const { notices, pushNotice } = useNoticeStore()

    pushNotice({ type: 'error', message: 'Something failed', durationMs: 1000 })

    expect(notices.value).toHaveLength(1)
    expect(notices.value[0]?.message).toBe('Something failed')

    vi.advanceTimersByTime(1000)

    expect(notices.value).toHaveLength(0)
  })
})
