<template>
  <div class="downloads-page">
    <section class="downloads-hero card">
      <div>
        <span class="downloads-hero__eyebrow">Downloads</span>
        <h2 class="page-title">Capture new items without leaving the queue behind.</h2>
        <p class="page-copy">Form and schedules stay together, while the active queue gets the wider reading area.</p>
      </div>
    </section>

    <div class="downloads-layout">
      <div class="downloads-sidebar">
        <DownloadForm />
        <ScheduleManager />
      </div>

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

.downloads-hero {
  background:
    radial-gradient(circle at top right, rgba(59, 130, 246, 0.12), transparent 28%),
    linear-gradient(180deg, rgba(255,255,255,0.02), rgba(255,255,255,0)),
    var(--bg-card);
}

.downloads-hero__eyebrow {
  display: inline-flex;
  margin-bottom: 8px;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--info);
}

.page-title {
  font-size: clamp(22px, 3vw, 30px);
  font-weight: 700;
  line-height: 1.15;
}

.page-copy {
  margin-top: 8px;
  max-width: 680px;
  color: var(--text-muted);
}

.downloads-layout {
  display: grid;
  grid-template-columns: minmax(280px, 340px) minmax(0, 1fr);
  gap: 24px;
  align-items: flex-start;
}

.downloads-sidebar {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.downloads-main { flex: 1; }

@media (max-width: 980px) {
  .downloads-layout {
    grid-template-columns: 1fr;
  }
}
</style>
