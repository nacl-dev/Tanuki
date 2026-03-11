<template>
  <div class="download-queue">
    <div class="queue-header">
      <div class="queue-header__copy">
        <h3>Queue</h3>
        <p>{{ store.jobs.length }} job{{ store.jobs.length !== 1 ? 's' : '' }} tracked across active and finished downloads.</p>
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
        v-for="job in filtered"
        :key="job.id"
        :job="job"
        @control="(action) => store.control(job.id, action)"
        @remove="store.remove(job.id)"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useDownloadStore } from '@/stores/downloadStore'
import AppIcon from '@/components/Layout/AppIcon.vue'
import DownloadProgress from './DownloadProgress.vue'

const store = useDownloadStore()

const activeFilter = ref('all')
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
const activeCount = computed(() => store.jobs.filter((job) => ['queued', 'downloading', 'processing'].includes(job.status)).length)
const completedCount = computed(() => store.jobs.filter((job) => job.status === 'completed').length)
const failedCount = computed(() => store.jobs.filter((job) => job.status === 'failed').length)
const shouldPoll = computed(() => store.activeJobs().length > 0)

function setFilter(v: string) {
  activeFilter.value = v
}

onMounted(() => { store.fetchJobs() })

let interval: ReturnType<typeof setInterval> | null = null

function startPolling() {
  if (interval) return
  interval = setInterval(() => store.fetchJobs(undefined, { silent: true }), 3000)
}

function stopPolling() {
  if (!interval) return
  clearInterval(interval)
  interval = null
}

watch(shouldPoll, (active) => {
  if (active) {
    startPolling()
  } else {
    stopPolling()
  }
}, { immediate: true })

onUnmounted(() => stopPolling())
</script>

<style scoped>
.download-queue { display: flex; flex-direction: column; gap: 12px; }
.queue-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.queue-header__copy { display: flex; flex-direction: column; gap: 4px; }
.queue-header h3 { font-size: 15px; font-weight: 600; }
.queue-header p { font-size: 12px; color: var(--text-muted); }
.queue-filters { display: flex; gap: 6px; flex-wrap: wrap; }
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
}
</style>
