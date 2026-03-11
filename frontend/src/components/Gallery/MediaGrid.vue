<template>
  <div v-if="loading" class="grid-loading">Loading…</div>
  <div v-else-if="items.length === 0" class="grid-empty">No media found.</div>
  <div v-else :class="['media-grid', `media-grid--${density}`]">
    <MediaCard
      v-for="item in items"
      :key="item.id"
      :media="item"
      :show-tags="showTags"
      :compact="compactCards"
    />
  </div>
</template>

<script setup lang="ts">
import type { Media } from '@/api/mediaApi'
import MediaCard from './MediaCard.vue'

withDefaults(defineProps<{
  items: Media[]
  loading?: boolean
  density?: 'cozy' | 'compact'
  showTags?: boolean
  compactCards?: boolean
}>(), {
  showTags: true,
  compactCards: false,
})
</script>

<style scoped>
.media-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 16px;
}

.media-grid--compact {
  grid-template-columns: repeat(auto-fill, minmax(148px, 1fr));
  gap: 12px;
}

.grid-loading,
.grid-empty {
  text-align: center;
  padding: 48px;
  color: var(--text-muted);
}

@media (max-width: 640px) {
  .media-grid {
    grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
    gap: 12px;
  }

  .media-grid--compact {
    grid-template-columns: repeat(auto-fill, minmax(126px, 1fr));
    gap: 10px;
  }

  .grid-loading,
  .grid-empty {
    padding: 32px 16px;
  }
}
</style>
