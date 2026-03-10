<template>
  <div class="download-queue">
    <div class="queue-header">
      <h3>Queue ({{ store.jobs.length }})</h3>
      <div class="queue-filters">
        <button
          v-for="f in filters"
          :key="f.value"
          :class="['btn btn-ghost btn-sm', { 'active': activeFilter === f.value }]"
          @click="setFilter(f.value)"
        >{{ f.label }}</button>
      </div>
    </div>

    <div v-if="store.loading" class="queue-empty">Loading…</div>
    <div v-else-if="filtered.length === 0" class="queue-empty">No downloads found.</div>

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
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useDownloadStore } from '@/stores/downloadStore'
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

const filtered = computed(() =>
  activeFilter.value === 'all'
    ? store.jobs
    : store.jobs.filter((j) => j.status === activeFilter.value)
)

function setFilter(v: string) {
  activeFilter.value = v
}

onMounted(() => { store.fetchJobs() })

// Poll active downloads every 3 seconds
let interval: ReturnType<typeof setInterval>
onMounted(() => { interval = setInterval(() => store.fetchJobs(), 3000) })
onUnmounted(() => clearInterval(interval))
</script>

<style scoped>
.download-queue { display: flex; flex-direction: column; gap: 12px; }
.queue-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.queue-header h3 { font-size: 15px; font-weight: 600; }
.queue-filters { display: flex; gap: 6px; }
.queue-list { display: flex; flex-direction: column; gap: 8px; }
.queue-empty { color: var(--text-muted); text-align: center; padding: 32px; }
.btn-sm { padding: 4px 10px; font-size: 12px; }
.active { background: var(--accent-dimmed); color: var(--accent); border-color: var(--accent); }
</style>
