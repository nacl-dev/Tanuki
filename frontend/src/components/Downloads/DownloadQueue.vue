<template>
  <div class="download-queue">
    <div class="queue-header">
      <div class="queue-header__copy">
        <div class="queue-header__title">
          <h3>Download Queue</h3>
          <span :class="['queue-live', `queue-live--${liveStatus}`]">{{ liveLabel }}</span>
        </div>
        <p>{{ store.jobs.length }} job{{ store.jobs.length !== 1 ? 's' : '' }} tracked across active and finished link captures.</p>
      </div>
      <div class="queue-filters">
        <button
          v-for="f in filters"
          :key="f.value"
          :class="['btn btn-ghost btn-sm', { 'active': activeFilter === f.value }]"
          @click="setFilter(f.value)"
        >{{ f.label }}</button>
      </div>
    </div>

    <div v-if="store.jobs.length > 0" class="queue-summary">
      <span class="queue-summary__chip">
        <strong>{{ activeCount }}</strong>
        active
      </span>
      <span class="queue-summary__chip">
        <strong>{{ completedCount }}</strong>
        completed
      </span>
      <span class="queue-summary__chip">
        <strong>{{ failedCount }}</strong>
        failed
      </span>
    </div>

    <div v-if="store.loading" class="queue-empty">Loading…</div>
    <div v-else-if="filtered.length === 0" class="queue-empty queue-empty--framed">
      <AppIcon name="download" :size="18" />
      <span>No downloads found for this filter.</span>
    </div>

    <div v-else class="queue-list">
      <DownloadProgress
        v-for="job in paginatedJobs"
        :key="job.id"
        :job="job"
        @control="(action) => store.control(job.id, action)"
        @remove="store.remove(job.id)"
      />
    </div>

    <div v-if="filtered.length > pageSize" class="queue-pagination">
      <p class="queue-pagination__summary">
        Showing {{ pageStart }}-{{ pageEnd }} of {{ filtered.length }} downloads
      </p>
      <div class="queue-pagination__controls">
        <button
          class="btn btn-ghost btn-sm"
          :disabled="currentPage === 1"
          @click="currentPage -= 1"
        >Previous</button>
        <span class="queue-pagination__page">Page {{ currentPage }} / {{ totalPages }}</span>
        <button
          class="btn btn-ghost btn-sm"
          :disabled="currentPage === totalPages"
          @click="currentPage += 1"
        >Next</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { downloadApi, type DownloadJob } from '@/api/downloadApi'
import { useDownloadStore } from '@/stores/downloadStore'
import AppIcon from '@/components/Layout/AppIcon.vue'
import DownloadProgress from './DownloadProgress.vue'

const store = useDownloadStore()

const activeFilter = ref('all')
const currentPage = ref(1)
const pageSize = 10
const filters = [
  { value: 'all',         label: 'All'         },
  { value: 'queued',      label: 'Queued'      },
  { value: 'downloading', label: 'Active'      },
  { value: 'completed',   label: 'Completed'   },
  { value: 'failed',      label: 'Failed'      },
]

const sortedJobs = computed(() =>
  [...store.jobs].sort((a, b) => {
    const aTime = new Date(a.created_at).getTime()
    const bTime = new Date(b.created_at).getTime()
    return bTime - aTime
  })
)

const filtered = computed(() =>
  activeFilter.value === 'all'
    ? sortedJobs.value
    : sortedJobs.value.filter((j) => j.status === activeFilter.value)
)
const totalPages = computed(() => Math.max(1, Math.ceil(filtered.value.length / pageSize)))
const paginatedJobs = computed(() => {
  const start = (currentPage.value - 1) * pageSize
  return filtered.value.slice(start, start + pageSize)
})
const pageStart = computed(() => {
  if (filtered.value.length === 0) {
    return 0
  }
  return (currentPage.value - 1) * pageSize + 1
})
const pageEnd = computed(() => Math.min(currentPage.value * pageSize, filtered.value.length))
const activeCount = computed(() => store.jobs.filter((job) => ['queued', 'downloading', 'processing'].includes(job.status)).length)
const completedCount = computed(() => store.jobs.filter((job) => job.status === 'completed').length)
const failedCount = computed(() => store.jobs.filter((job) => job.status === 'failed').length)
const liveStatus = ref<'connecting' | 'live' | 'fallback'>('connecting')
const liveLabel = computed(() => {
  switch (liveStatus.value) {
    case 'live':
      return 'Live'
    case 'fallback':
      return 'Polling fallback'
    default:
      return 'Connecting'
  }
})

function setFilter(v: string) {
  activeFilter.value = v
  currentPage.value = 1
}

watch(filtered, (jobs) => {
  const maxPage = Math.max(1, Math.ceil(jobs.length / pageSize))
  if (currentPage.value > maxPage) {
    currentPage.value = maxPage
  }
})

let eventSource: EventSource | null = null
let fallbackInterval: ReturnType<typeof setInterval> | null = null

function startFallbackPolling() {
  if (fallbackInterval) return
  fallbackInterval = setInterval(() => store.fetchJobs(undefined, { silent: true }), 4000)
}

function stopFallbackPolling() {
  if (!fallbackInterval) return
  clearInterval(fallbackInterval)
  fallbackInterval = null
}

function startLiveStream() {
  if (typeof window === 'undefined' || typeof EventSource === 'undefined') {
    liveStatus.value = 'fallback'
    startFallbackPolling()
    return
  }

  eventSource = new EventSource(downloadApi.streamUrl())
  eventSource.onopen = () => {
    liveStatus.value = 'live'
    stopFallbackPolling()
  }
  eventSource.onmessage = (event) => {
    try {
      store.replaceJobs(JSON.parse(event.data) as DownloadJob[])
    } catch {
      // Ignore malformed frames and wait for the next snapshot.
    }
  }
  eventSource.onerror = () => {
    liveStatus.value = 'fallback'
    startFallbackPolling()
  }
}

function stopLiveStream() {
  if (!eventSource) return
  eventSource.close()
  eventSource = null
}

onMounted(async () => {
  await store.fetchJobs()
  startLiveStream()
})

onUnmounted(() => {
  stopFallbackPolling()
  stopLiveStream()
})
</script>

<style scoped>
.download-queue { display: flex; flex-direction: column; gap: 12px; }
.queue-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.queue-header__copy { display: flex; flex-direction: column; gap: 4px; }
.queue-header__title { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.queue-header h3 { font-size: 15px; font-weight: 600; }
.queue-header p { font-size: 12px; color: var(--text-muted); }
.queue-filters { display: flex; gap: 6px; flex-wrap: wrap; }
.queue-live {
  display: inline-flex;
  align-items: center;
  min-height: 22px;
  padding: 0 8px;
  border-radius: 999px;
  border: 1px solid var(--border);
  background: var(--bg-surface);
  color: var(--text-secondary);
  font-size: 11px;
  letter-spacing: 0.03em;
  text-transform: uppercase;
}
.queue-live--live {
  border-color: rgba(74, 222, 128, 0.35);
  color: #4ade80;
}
.queue-live--fallback {
  border-color: rgba(245, 158, 11, 0.35);
  color: #f0b35b;
}
.queue-summary { display: flex; gap: 8px; flex-wrap: wrap; }
.queue-summary__chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 30px;
  padding: 0 10px;
  border-radius: 999px;
  background: var(--bg-surface);
  border: 1px solid var(--border);
  color: var(--text-secondary);
  font-size: 12px;
}
.queue-summary__chip strong {
  color: var(--text-primary);
  font-size: 13px;
}
.queue-list { display: flex; flex-direction: column; gap: 8px; }
.queue-pagination {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  padding-top: 4px;
}
.queue-pagination__summary {
  font-size: 12px;
  color: var(--text-muted);
}
.queue-pagination__controls {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.queue-pagination__page {
  min-width: 80px;
  text-align: center;
  font-size: 12px;
  color: var(--text-secondary);
}
.queue-empty { color: var(--text-muted); text-align: center; padding: 32px; }
.queue-empty--framed {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  background: var(--bg-card);
  border: 1px dashed var(--border);
  border-radius: var(--radius-lg);
}
.btn-sm { padding: 4px 10px; font-size: 12px; }
.active { background: var(--accent-dimmed); color: var(--accent); border-color: var(--accent); }

@media (max-width: 720px) {
  .queue-header {
    flex-direction: column;
    align-items: flex-start;
  }

  .queue-pagination {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
