<template>
  <div class="library-page">
    <!-- Filters sidebar -->
    <aside class="filter-sidebar">
      <h4>Type</h4>
      <label v-for="t in types" :key="t.value" class="filter-option">
        <input
          type="radio"
          name="type"
          :value="t.value"
          :checked="store.filters.type === t.value"
          @change="store.setFilter('type', t.value)"
        />
        {{ t.label }}
      </label>

      <h4>Show</h4>
      <label class="filter-option">
        <input
          type="checkbox"
          :checked="store.filters.favorite"
          @change="store.setFilter('favorite', ($event.target as HTMLInputElement).checked || undefined)"
        />
        Favorites only
      </label>

      <h4>Min Rating</h4>
      <div class="rating-filter">
        <span
          v-for="star in 5"
          :key="star"
          class="rating-star"
          :class="{ active: (store.filters.min_rating ?? 0) >= star }"
          @click="setMinRating(star)"
          title="Minimum rating"
        >★</span>
        <button
          v-if="store.filters.min_rating"
          class="clear-rating"
          @click="store.setFilter('min_rating', undefined)"
        >✕</button>
      </div>
    </aside>

    <!-- Gallery -->
    <section class="gallery-section">
      <div class="gallery-header">
        <SearchBar @search="onSearch" />
        <div class="gallery-controls">
          <span class="gallery-count">{{ store.total }} items</span>
          <select
            class="sort-select"
            :value="store.filters.sort"
            @change="store.setFilter('sort', ($event.target as HTMLSelectElement).value)"
          >
            <option v-for="s in sortOptions" :key="s.value" :value="s.value">{{ s.label }}</option>
          </select>
        </div>
      </div>
      <MediaGrid :items="store.items" :loading="store.loading" />

      <!-- Pagination controls -->
      <div v-if="store.totalPages > 1" class="pagination">
        <button
          class="btn btn-ghost btn-sm"
          :disabled="store.currentPage <= 1"
          @click="store.prevPage()"
        >← Previous</button>
        <span class="pagination-info">Page {{ store.currentPage }} of {{ store.totalPages }}</span>
        <button
          class="btn btn-ghost btn-sm"
          :disabled="store.currentPage >= store.totalPages"
          @click="store.nextPage()"
        >Next →</button>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useMediaStore } from '@/stores/mediaStore'
import MediaGrid from '@/components/Gallery/MediaGrid.vue'
import SearchBar from '@/components/Search/SearchBar.vue'

const store = useMediaStore()
const route = useRoute()

const types = [
  { value: '',          label: 'All'        },
  { value: 'video',     label: '🎬 Videos'  },
  { value: 'image',     label: '🖼️ Images'  },
  { value: 'manga',     label: '📖 Manga'   },
  { value: 'comic',     label: '📕 Comics'  },
  { value: 'doujinshi', label: '📗 Doujin'  },
]

const sortOptions = [
  { value: 'newest', label: '🕒 Newest'   },
  { value: 'oldest', label: '🕰️ Oldest'   },
  { value: 'title',  label: '🔤 Title'    },
  { value: 'rating', label: '⭐ Rating'   },
  { value: 'size',   label: '📦 Size'     },
  { value: 'views',  label: '👁️ Views'    },
]

function onSearch(q: string) {
  store.setFilter('q', q)
}

function setMinRating(star: number) {
  // Clicking the same star again clears the filter.
  if (store.filters.min_rating === star) {
    store.setFilter('min_rating', undefined)
  } else {
    store.setFilter('min_rating', star)
  }
}

onMounted(() => {
  const tagParam = route.query.tag
  if (tagParam && typeof tagParam === 'string' && tagParam.trim() !== '') {
    store.setFilter('tag', tagParam.trim())
  } else {
    store.fetchList()
  }
})
</script>

<style scoped>
.library-page {
  display: flex;
  gap: 24px;
  align-items: flex-start;
}

.filter-sidebar {
  width: 180px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.filter-sidebar h4 {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
  margin-top: 12px;
}

.filter-option {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  font-size: 13px;
  color: var(--text-secondary);
}

.filter-option:hover { color: var(--text-primary); }

.rating-filter {
  display: flex;
  align-items: center;
  gap: 2px;
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
  gap: 8px;
}

.gallery-controls {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.gallery-count { font-size: 13px; color: var(--text-muted); }

.sort-select {
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  color: var(--text-primary);
  font-size: 13px;
  padding: 4px 8px;
  cursor: pointer;
}

.pagination {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 16px;
  padding-top: 16px;
}

.pagination-info { font-size: 13px; color: var(--text-muted); }
</style>
