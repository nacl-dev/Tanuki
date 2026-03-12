import { defineStore } from 'pinia'
import { ref } from 'vue'
import { downloadApi, type DownloadJob, type DownloadSchedule, type CreateDownloadInput } from '@/api/downloadApi'

export const useDownloadStore = defineStore('download', () => {
  const jobs = ref<DownloadJob[]>([])
  const schedules = ref<DownloadSchedule[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetchJobs(status?: string, options?: { silent?: boolean }) {
    const silent = options?.silent === true
    if (!silent) {
      loading.value = true
    }
    error.value = null
    try {
      const res = await downloadApi.list(status)
      jobs.value = res.data ?? []
    } catch (e: any) {
      error.value = e.message
    } finally {
      if (!silent) {
        loading.value = false
      }
    }
  }

  async function fetchSchedules() {
    const res = await downloadApi.listSchedules()
    schedules.value = res.data ?? []
  }

  function replaceJobs(nextJobs: DownloadJob[]) {
    jobs.value = nextJobs
  }

  async function createSchedule(input: Omit<DownloadSchedule, 'id' | 'created_at' | 'last_run' | 'next_run'>) {
    const res = await downloadApi.createSchedule(input)
    schedules.value.unshift(res.data)
    return res.data
  }

  async function updateSchedule(id: string, body: Partial<DownloadSchedule>) {
    const res = await downloadApi.updateSchedule(id, body)
    const idx = schedules.value.findIndex((s) => s.id === id)
    if (idx !== -1) schedules.value[idx] = res.data
  }

  async function removeSchedule(id: string) {
    await downloadApi.removeSchedule(id)
    schedules.value = schedules.value.filter((s) => s.id !== id)
  }

  async function enqueue(input: CreateDownloadInput) {
    const res = await downloadApi.create(input)
    jobs.value.unshift(res.data)
    return res.data
  }

  async function enqueueBatch(urls: string[], targetDirectory?: string, autoTags?: string[]) {
    await downloadApi.batch(urls, targetDirectory, autoTags)
    await fetchJobs()
  }

  async function control(id: string, action: 'pause' | 'resume' | 'cancel' | 'retry') {
    const res = await downloadApi.update(id, action)
    const idx = jobs.value.findIndex((j) => j.id === id)
    if (idx !== -1) jobs.value[idx] = res.data
  }

  async function remove(id: string) {
    await downloadApi.remove(id)
    jobs.value = jobs.value.filter((j) => j.id !== id)
  }

  /** Active downloads (for progress polling) */
  const activeJobs = () =>
    jobs.value.filter((j) =>
      j.status === 'downloading' || j.status === 'queued' || j.status === 'processing',
    )

  return {
    jobs,
    schedules,
    loading,
    error,
    replaceJobs,
    fetchJobs,
    fetchSchedules,
    createSchedule,
    updateSchedule,
    removeSchedule,
    enqueue,
    enqueueBatch,
    control,
    remove,
    activeJobs,
  }
})
