import { ref } from 'vue'

export type NoticeType = 'info' | 'success' | 'error'

export interface NoticeItem {
  id: number
  type: NoticeType
  message: string
}

const notices = ref<NoticeItem[]>([])
let noticeId = 0

export function useNoticeStore() {
  function pushNotice(input: { type?: NoticeType; message: string; durationMs?: number }) {
    const notice: NoticeItem = {
      id: ++noticeId,
      type: input.type ?? 'info',
      message: input.message,
    }
    notices.value = [...notices.value, notice]

    const durationMs = input.durationMs ?? 3200
    if (durationMs > 0) {
      window.setTimeout(() => removeNotice(notice.id), durationMs)
    }

    return notice.id
  }

  function removeNotice(id: number) {
    notices.value = notices.value.filter((item) => item.id !== id)
  }

  return {
    notices,
    pushNotice,
    removeNotice,
  }
}
