<template>
  <div class="dl-item">
    <div class="dl-item__header">
      <div class="dl-item__source">
        <div class="dl-item__source-icon">
          <AppIcon name="download" :size="16" />
        </div>
        <div class="dl-item__source-copy">
          <span class="dl-item__url" :title="sourceTitle || job.url">{{ sourceTitle || shortUrl }}</span>
          <span class="dl-item__host" :title="job.url">{{ job.url }}</span>
        </div>
      </div>
      <StatusBadge :status="job.status" />
    </div>

    <div v-if="isActive" class="dl-item__progress">
      <div class="progress-bar">
        <div class="progress-bar__fill" :style="{ width: job.progress + '%' }" />
      </div>
      <span class="progress-pct">{{ job.progress.toFixed(1) }}%</span>
    </div>

    <div class="dl-item__meta">
      <span v-if="workSummary" class="dl-item__meta-chip">
        {{ workSummary }}
      </span>
      <span v-if="job.total_bytes > 0" class="dl-item__meta-chip">
        {{ formatBytes(job.downloaded_bytes) }} / {{ formatBytes(job.total_bytes) }}
      </span>
      <span v-else-if="job.downloaded_bytes > 0" class="dl-item__meta-chip">
        {{ formatBytes(job.downloaded_bytes) }}
      </span>
      <span v-if="job.total_files > 0" class="dl-item__meta-chip">
        {{ job.downloaded_files }} / {{ job.total_files }} files
      </span>
      <span v-else-if="job.downloaded_files > 0" class="dl-item__meta-chip">
        {{ job.downloaded_files }} files
      </span>
      <span v-if="job.target_directory" class="dl-item__meta-chip">
        {{ job.target_directory }}
      </span>
    </div>

    <div v-if="job.error_message" class="dl-item__error">{{ job.error_message }}</div>

    <div class="dl-item__actions">
      <button v-if="job.status === 'downloading'" type="button" class="btn btn-ghost btn-sm" aria-label="Pause download" @click="emit('control', 'pause')">
        <AppIcon name="pause" :size="13" />
        Pause
      </button>
      <button v-if="job.status === 'paused'" type="button" class="btn btn-ghost btn-sm" aria-label="Resume download" @click="emit('control', 'resume')">
        <AppIcon name="play" :size="13" />
        Resume
      </button>
      <button v-if="job.status === 'failed'" type="button" class="btn btn-ghost btn-sm" aria-label="Retry download" @click="emit('control', 'retry')">
        <AppIcon name="refresh" :size="13" />
        Retry
      </button>
      <button v-if="canCancel" type="button" class="btn btn-ghost btn-sm" aria-label="Cancel download" @click="emit('control', 'cancel')">
        <AppIcon name="close" :size="13" />
        Cancel
      </button>
      <button type="button" class="btn btn-ghost btn-sm" aria-label="Remove download from list" @click="emit('remove')">
        <AppIcon name="trash" :size="13" />
        Remove
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { DownloadJob } from '@/api/downloadApi'
import AppIcon from '@/components/Layout/AppIcon.vue'
import StatusBadge from './StatusBadge.vue'

const props = defineProps<{ job: DownloadJob }>()
const emit = defineEmits<{
  (e: 'control', action: 'pause' | 'resume' | 'cancel' | 'retry'): void
  (e: 'remove'): void
}>()

const shortUrl = computed(() => {
  try { return new URL(props.job.url).hostname + '…' } catch { return props.job.url }
})
const sourceTitle = computed(() => props.job.source_metadata?.title?.trim() || '')
const workSummary = computed(() => {
  const workTitle = props.job.source_metadata?.work_title?.trim()
  if (!workTitle) {
    return ''
  }
  const workIndex = props.job.source_metadata?.work_index ?? 0
  return workIndex > 0
    ? `${workTitle} #${String(workIndex).padStart(2, '0')}`
    : workTitle
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
  gap: 10px;
  padding: 14px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  box-shadow: inset 0 1px 0 rgba(255,255,255,0.03);
}

.dl-item__header { display: flex; justify-content: space-between; align-items: center; gap: 12px; }

.dl-item__source {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.dl-item__source-icon {
  width: 34px;
  height: 34px;
  border-radius: 12px;
  background: rgba(245, 158, 11, 0.12);
  color: var(--accent);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.dl-item__source-copy {
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 0;
}

.dl-item__url {
  font-size: 13px;
  color: var(--text-primary);
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.dl-item__host {
  font-size: 11px;
  color: var(--text-muted);
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

.dl-item__meta-chip {
  display: inline-flex;
  align-items: center;
  min-height: 28px;
  padding: 0 10px;
  background: var(--bg-surface);
  border: 1px solid var(--border);
  border-radius: 999px;
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

.dl-item__actions { display: flex; gap: 6px; flex-wrap: wrap; }
.btn-sm { padding: 4px 8px; font-size: 12px; }

@media (max-width: 620px) {
  .dl-item__header {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
