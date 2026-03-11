<template>
  <div v-if="loading" class="grid-loading">Loading…</div>
  <div v-else-if="items.length === 0" class="grid-empty">No media found.</div>
  <div v-else :class="['media-grid', `media-grid--${density}`]">
    <MediaCard v-for="item in items" :key="item.id" :media="item" />
  </div>
</template>

<script setup lang="ts">
import type { Media } from '@/api/mediaApi'
import MediaCard from './MediaCard.vue'

defineProps<{
  items: Media[]
  loading?: boolean
  density?: 'cozy' | 'compact'
}>()
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
</style>
