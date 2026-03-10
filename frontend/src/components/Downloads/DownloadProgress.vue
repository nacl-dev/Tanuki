<template>
  <div class="dl-item">
    <div class="dl-item__header">
      <span class="dl-item__url" :title="job.url">{{ shortUrl }}</span>
      <StatusBadge :status="job.status" />
    </div>

    <div v-if="isActive" class="dl-item__progress">
      <div class="progress-bar">
        <div class="progress-bar__fill" :style="{ width: job.progress + '%' }" />
      </div>
      <span class="progress-pct">{{ job.progress.toFixed(1) }}%</span>
    </div>

    <div class="dl-item__meta">
      <span v-if="job.total_bytes > 0">
        {{ formatBytes(job.downloaded_bytes) }} / {{ formatBytes(job.total_bytes) }}
      </span>
      <span v-else-if="job.downloaded_bytes > 0">
        {{ formatBytes(job.downloaded_bytes) }}
      </span>
      <span v-if="job.total_files > 0">
        {{ job.downloaded_files }} / {{ job.total_files }} files
      </span>
      <span v-else-if="job.downloaded_files > 0">
        {{ job.downloaded_files }} files
      </span>
      <span v-if="job.target_directory">{{ job.target_directory }}</span>
    </div>

    <div v-if="job.error_message" class="dl-item__error">{{ job.error_message }}</div>

    <div class="dl-item__actions">
      <button v-if="job.status === 'downloading'" class="btn btn-ghost btn-sm" @click="emit('control', 'pause')">⏸</button>
      <button v-if="job.status === 'paused'"      class="btn btn-ghost btn-sm" @click="emit('control', 'resume')">▶️</button>
      <button v-if="job.status === 'failed'"      class="btn btn-ghost btn-sm" @click="emit('control', 'retry')">🔄 Retry</button>
      <button v-if="canCancel"                    class="btn btn-ghost btn-sm" @click="emit('control', 'cancel')">✕ Cancel</button>
      <button class="btn btn-ghost btn-sm" @click="emit('remove')">🗑</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { DownloadJob } from '@/api/downloadApi'
import StatusBadge from './StatusBadge.vue'

const props = defineProps<{ job: DownloadJob }>()
const emit = defineEmits<{
  (e: 'control', action: 'pause' | 'resume' | 'cancel' | 'retry'): void
  (e: 'remove'): void
}>()

const shortUrl = computed(() => {
  try { return new URL(props.job.url).hostname + '…' } catch { return props.job.url }
})

const isActive = computed(() => ['queued', 'downloading', 'processing'].includes(props.job.status))
const canCancel = computed(() => ['queued', 'downloading', 'paused'].includes(props.job.status))

function formatBytes(value: number) {
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`
  if (value < 1024 ** 3) return `${(value / 1024 / 1024).toFixed(1)} MB`
  return `${(value / 1024 ** 3).toFixed(2)} GB`
}
</script>

<style scoped>
.dl-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px 14px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: var(--radius);
}

.dl-item__header { display: flex; justify-content: space-between; align-items: center; gap: 12px; }

.dl-item__url {
  flex: 1;
  font-size: 13px;
  color: var(--text-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.dl-item__error { font-size: 12px; color: var(--danger); }
.dl-item__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  font-size: 12px;
  color: var(--text-muted);
}

.dl-item__progress { display: flex; align-items: center; gap: 10px; }

.progress-bar {
  flex: 1;
  height: 6px;
  background: var(--bg-hover);
  border-radius: 3px;
  overflow: hidden;
}

.progress-bar__fill {
  height: 100%;
  background: var(--accent);
  border-radius: 3px;
  transition: width 0.4s;
}

.progress-pct { font-size: 12px; color: var(--text-secondary); min-width: 44px; text-align: right; }

.dl-item__actions { display: flex; gap: 6px; }
.btn-sm { padding: 4px 8px; font-size: 12px; }
</style>
