<template>
  <div class="downloads-page">
    <h2 class="page-title">Download Manager</h2>

    <div class="downloads-layout">
      <!-- Left: form + schedules -->
      <div class="downloads-sidebar">
        <DownloadForm />
        <ScheduleManager />
      </div>

      <!-- Right: queue -->
      <div class="downloads-main">
        <DownloadQueue />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import DownloadForm from '@/components/Downloads/DownloadForm.vue'
import DownloadQueue from '@/components/Downloads/DownloadQueue.vue'
import ScheduleManager from '@/components/Downloads/ScheduleManager.vue'
</script>

<style scoped>
.downloads-page { display: flex; flex-direction: column; gap: 24px; }
.page-title { font-size: 22px; font-weight: 700; }

.downloads-layout {
  display: flex;
  gap: 24px;
  align-items: flex-start;
}

.downloads-sidebar {
  width: 320px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.downloads-main { flex: 1; }
</style>
