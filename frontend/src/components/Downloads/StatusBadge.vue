<template>
  <span :class="['status-badge', `status-badge--${status}`]">
    <AppIcon
      class="status-badge__icon"
      :class="{ 'status-badge__icon--spin': status === 'downloading' }"
      :name="icon"
      :size="12"
    />
    {{ label }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { DownloadJob } from '@/api/downloadApi'
import AppIcon from '@/components/Layout/AppIcon.vue'

const props = defineProps<{ status: DownloadJob['status'] }>()

const icon = computed(() => ({
  queued: 'download',
  downloading: 'refresh',
  processing: 'spark',
  completed: 'check',
  failed: 'close',
  paused: 'pause',
}[props.status] ?? 'download'))

const label = computed(() => ({
  queued:      'Queued',
  downloading: 'Downloading',
  processing:  'Processing',
  completed:   'Done',
  failed:      'Failed',
  paused:      'Paused',
}[props.status] ?? props.status))
</script>

<style scoped>
.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 999px;
  white-space: nowrap;
}
.status-badge__icon--spin {
  animation: status-spin 1.1s linear infinite;
}
.status-badge--queued      { background: rgba(96,112,128,0.2);  color: #8098b0; }
.status-badge--downloading { background: rgba(59,130,246,0.2);  color: #60a5fa; }
.status-badge--processing  { background: rgba(168,85,247,0.2);  color: #c084fc; }
.status-badge--completed   { background: rgba(34,197,94,0.2);   color: #4ade80; }
.status-badge--failed      { background: rgba(239,68,68,0.2);   color: #f87171; }
.status-badge--paused      { background: rgba(245,158,11,0.2);  color: #fbbf24; }

@keyframes status-spin {
  to { transform: rotate(360deg); }
}
</style>
