<template>
  <div v-if="loading" class="loading">Loading…</div>

  <div v-else-if="media" class="media-detail">
    <!-- Header -->
    <div class="detail-header">
      <button class="btn btn-ghost btn-sm" @click="router.back()">← Back</button>
      <h1 class="detail-title">{{ media.title }}</h1>
      <button
        :class="['btn btn-ghost', { 'fav-active': media.favorite }]"
        @click="toggleFav"
      >♥ {{ media.favorite ? 'Unfavorite' : 'Favorite' }}</button>
    </div>

    <!-- Body -->
    <div class="detail-body">
      <!-- Preview -->
      <div class="detail-preview">
        <video
          v-if="media.type === 'video'"
          :src="`/api/media/${media.id}/stream`"
          controls
          class="media-video"
        />
        <img
          v-else-if="media.thumbnail_url"
          :src="media.thumbnail_url"
          :alt="media.title"
          class="media-image"
        />
        <div v-else class="media-placeholder">{{ media.type }}</div>
      </div>

      <!-- Meta -->
      <aside class="detail-meta">
        <!-- Rating -->
        <div class="meta-section">
          <span class="meta-label">Rating</span>
          <div class="stars">
            <span
              v-for="i in 5" :key="i"
              :class="['star', { 'star--on': i <= (media.rating ?? 0) }]"
              @click="setRating(i)"
            >★</span>
          </div>
        </div>

        <!-- Tags -->
        <div class="meta-section">
          <span class="meta-label">Tags</span>
          <div class="tags-list">
            <TagBadge v-for="tag in media.tags" :key="tag.id" :tag="tag" />
          </div>
        </div>

        <!-- Details -->
        <div class="meta-section">
          <span class="meta-label">Details</span>
          <table class="meta-table">
            <tr><td>Type</td><td>{{ media.type }}</td></tr>
            <tr><td>Language</td><td>{{ media.language || '—' }}</td></tr>
            <tr><td>Views</td><td>{{ media.view_count }}</td></tr>
            <tr><td>Size</td><td>{{ formatBytes(media.file_size) }}</td></tr>
          </table>
        </div>

        <div v-if="media.source_url" class="meta-section">
          <a :href="media.source_url" target="_blank" rel="noopener" class="source-link">
            🔗 Source
          </a>
        </div>
      </aside>
    </div>
  </div>

  <div v-else class="not-found">Media not found.</div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { mediaApi, type Media } from '@/api/mediaApi'
import TagBadge from '@/components/Tags/TagBadge.vue'

const route = useRoute()
const router = useRouter()
const media = ref<Media | null>(null)
const loading = ref(true)

onMounted(async () => {
  try {
    const res = await mediaApi.get(route.params.id as string)
    media.value = res.data
  } finally {
    loading.value = false
  }
})

async function toggleFav() {
  if (!media.value) return
  const res = await mediaApi.update(media.value.id, { favorite: !media.value.favorite })
  media.value = res.data
}

async function setRating(r: number) {
  if (!media.value) return
  const res = await mediaApi.update(media.value.id, { rating: r })
  media.value = res.data
}

function formatBytes(b: number): string {
  if (b < 1024) return `${b} B`
  if (b < 1024 * 1024) return `${(b / 1024).toFixed(1)} KB`
  if (b < 1024 ** 3) return `${(b / 1024 / 1024).toFixed(1)} MB`
  return `${(b / 1024 ** 3).toFixed(2)} GB`
}
</script>

<style scoped>
.loading, .not-found { text-align: center; padding: 48px; color: var(--text-muted); }

.detail-header {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 24px;
}
.detail-title { flex: 1; font-size: 20px; font-weight: 700; }

.detail-body {
  display: flex;
  gap: 32px;
  align-items: flex-start;
}

.detail-preview {
  flex: 1;
  min-width: 0;
  background: var(--bg-surface);
  border-radius: var(--radius-lg);
  overflow: hidden;
}

.media-video { width: 100%; max-height: 70vh; }
.media-image { width: 100%; height: auto; }
.media-placeholder { padding: 80px; text-align: center; color: var(--text-muted); font-size: 32px; }

.detail-meta { width: 260px; flex-shrink: 0; display: flex; flex-direction: column; gap: 20px; }

.meta-section { display: flex; flex-direction: column; gap: 8px; }
.meta-label { font-size: 11px; text-transform: uppercase; color: var(--text-muted); letter-spacing: 0.05em; }

.stars { display: flex; gap: 4px; }
.star { font-size: 20px; cursor: pointer; color: var(--border); transition: color 0.1s; }
.star--on, .star:hover { color: var(--accent); }

.tags-list { display: flex; flex-wrap: wrap; gap: 6px; }

.meta-table { border-collapse: collapse; width: 100%; font-size: 13px; }
.meta-table td { padding: 4px 0; }
.meta-table td:first-child { color: var(--text-muted); width: 60px; }

.source-link { color: var(--accent); font-size: 13px; }
.fav-active { color: var(--danger); border-color: var(--danger); }
.btn-sm { padding: 6px 12px; font-size: 13px; }
</style>
