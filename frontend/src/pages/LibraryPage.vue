<template>
  <div class="library-page">
    <aside class="filter-bar">
      <div class="filter-options filter-options--inline">
        <button
          v-for="t in types"
          :key="t.value"
          :class="['filter-chip', { active: store.filters.type === t.value }]"
          @click="store.setFilter('type', t.value)"
        >
          {{ t.label }}
        </button>
        <button
          :class="['filter-chip', { active: !!store.filters.favorite }]"
          @click="store.setFilter('favorite', store.filters.favorite ? undefined : true)"
        >
          Favorites only
        </button>
      </div>

      <div class="rating-filter">
        <span
          v-for="star in 5"
          :key="star"
          class="rating-star"
          :class="{ active: (hoveredRating ?? store.filters.min_rating ?? 0) >= star }"
          @click="setMinRating(star)"
          @mouseenter="hoveredRating = star"
          @mouseleave="hoveredRating = null"
          title="Minimum rating"
        >★</span>
        <button
          v-if="store.filters.min_rating"
          class="clear-rating"
          @click="store.setFilter('min_rating', undefined)"
        >✕</button>
        <div class="sort-select-wrap">
          <select
            class="sort-select"
            :value="store.filters.sort"
            @change="store.setFilter('sort', ($event.target as HTMLSelectElement).value)"
          >
            <option v-for="s in sortOptions" :key="s.value" :value="s.value">{{ s.label }}</option>
          </select>
        </div>
      </div>
    </aside>

    <section class="gallery-section">
      <div class="gallery-header">
        <div class="gallery-controls">
          <span class="gallery-count">{{ store.total }} items</span>
        </div>
      </div>
      <MediaGrid :items="store.items" :loading="store.loading" />

      <div v-if="store.totalPages > 1" class="pagination">
        <button class="btn btn-ghost btn-sm" :disabled="store.currentPage <= 1" @click="store.prevPage()">← Previous</button>
        <span class="pagination-info">Page {{ store.currentPage }} of {{ store.totalPages }}</span>
        <button class="btn btn-ghost btn-sm" :disabled="store.currentPage >= store.totalPages" @click="store.nextPage()">Next →</button>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useMediaStore } from '@/stores/mediaStore'
import MediaGrid from '@/components/Gallery/MediaGrid.vue'

const store = useMediaStore()
const route = useRoute()
const hoveredRating = ref<number | null>(null)

const types = [
  { value: '', label: 'All' },
  { value: 'video', label: 'Videos' },
  { value: 'image', label: 'Images' },
  { value: 'manga', label: 'Manga' },
  { value: 'comic', label: 'Comics' },
  { value: 'doujinshi', label: 'Doujin' },
]
const sortOptions = [
  { value: 'newest', label: 'Newest' },
  { value: 'oldest', label: 'Oldest' },
  { value: 'title', label: 'Title' },
  { value: 'rating', label: 'Rating' },
  { value: 'size', label: 'Size' },
  { value: 'views', label: 'Views' },
]

function setMinRating(star: number) {
  if (store.filters.min_rating === star) {
    store.setFilter('min_rating', undefined)
  } else {
    store.setFilter('min_rating', star)
  }
}

onMounted(() => {
  const tagsParam = route.query.tags
  const tagParam = route.query.tag
  if (typeof tagsParam === 'string' && tagsParam.trim() !== '') {
    store.filters.q = ''
    store.filters.tag = ''
    store.setFilter('tags', tagsParam.trim())
  } else if (tagParam && typeof tagParam === 'string' && tagParam.trim() !== '') {
    store.filters.q = ''
    store.filters.tags = ''
    store.setFilter('tag', tagParam.trim())
  } else {
    store.filters.tag = ''
    store.filters.tags = ''
    store.fetchList()
  }
})

watch(
  () => [route.query.tag, route.query.tags],
  ([tagParam, tagsParam]) => {
    if (typeof tagsParam === 'string' && tagsParam.trim() !== '') {
      store.filters.q = ''
      store.filters.tag = ''
      store.setFilter('tags', tagsParam.trim())
      return
    }
    if (typeof tagParam === 'string' && tagParam.trim() !== '') {
      store.filters.q = ''
      store.setFilter('tag', tagParam.trim())
      return
    }
    if (store.filters.tag || store.filters.tags) {
      store.filters.tags = ''
      store.setFilter('tag', '')
    }
  },
)
</script>

<style scoped>
.library-page {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.filter-bar {
  display: flex;
  align-items: center;
  gap: 20px;
  flex-wrap: wrap;
  position: sticky;
  top: 0;
  z-index: 5;
  margin-top: 0;
  padding: 8px 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-lg, 12px);
  background: color-mix(in srgb, var(--bg-card) 92%, transparent);
  backdrop-filter: blur(10px);
}

.filter-options {
  display: flex;
  min-width: 0;
}

.filter-options--inline {
  flex-wrap: wrap;
  gap: 8px 12px;
  flex: 1;
}

.filter-chip {
  appearance: none;
  border: 1px solid var(--border);
  background: var(--bg-surface);
  color: var(--text-secondary);
  padding: 8px 12px;
  border-radius: 999px;
  font-size: 13px;
  line-height: 1;
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s, color 0.15s;
}

.filter-chip:hover {
  border-color: var(--accent);
  color: var(--text-primary);
}

.filter-chip.active {
  border-color: var(--accent);
  background: var(--accent-dimmed);
  color: var(--accent);
}

.sort-select {
  appearance: none;
  border: 1px solid var(--border);
  background: var(--bg-surface);
  color: var(--text-primary);
  padding: 8px 38px 8px 18px;
  border-radius: 999px;
  font-size: 13px;
  line-height: 1;
  cursor: pointer;
  min-width: 124px;
  text-align: center;
  text-align-last: center;
}

.sort-select:hover {
  border-color: var(--accent);
}

.rating-filter {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

.sort-select-wrap {
  display: flex;
  align-items: center;
}

.rating-star {
  cursor: pointer;
  font-size: 18px;
  color: var(--text-muted);
  transition: color 0.1s;
}

.rating-star.active { color: var(--accent, #f59e0b); }
.rating-star:hover { color: var(--accent, #f59e0b); }

.clear-rating {
  background: transparent;
  border: none;
  cursor: pointer;
  color: var(--text-muted);
  font-size: 11px;
  margin-left: 4px;
}
.clear-rating:hover { color: var(--text-primary); }

.gallery-section { flex: 1; display: flex; flex-direction: column; gap: 16px; }

.gallery-header {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin: 0;
}

.gallery-controls {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.gallery-count { font-size: 13px; color: var(--text-muted); }

.pagination {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 16px;
  padding-top: 16px;
}

.pagination-info { font-size: 13px; color: var(--text-muted); }

@media (max-width: 960px) {
  .filter-bar {
    gap: 16px;
  }
}

@media (max-width: 720px) {
  .filter-bar {
    position: static;
    padding: 10px;
  }

  .filter-options--inline {
    flex-direction: column;
    gap: 8px;
  }
}
</style>
