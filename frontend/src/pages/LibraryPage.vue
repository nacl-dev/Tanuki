<template>
  <div class="library-page">
    <aside class="filter-sidebar">
      <div class="import-panel">
        <h4>Library Intake</h4>
        <p class="import-help">
          Lege unorganisierte Dateien in den Root-Ordner <code>/inbox</code> und sortiere sie hier automatisch in die Bibliothek ein.
        </p>
        <div class="import-structure">
          <strong>Target layout</strong>
          <code>/Video/2D (Hentai)</code>
          <code>/Video/3D (Real)/Studio</code>
          <code>/Image/CG Sets</code>
          <code>/Image/GIFs</code>
          <code>/Image/Random</code>
          <code>/Comics/Manga</code>
          <code>/Comics/Doujins</code>
        </div>
        <input
          v-model="organizePath"
          class="import-input"
          type="text"
          placeholder="z. B. inbox/new oder inbox/studio-a"
        />
        <div class="import-actions">
          <button
            class="btn btn-secondary btn-sm"
            :disabled="organizing"
            @click="previewLibrary('move')"
          >
            {{ organizing ? 'Analyzing…' : 'Preview Move' }}
          </button>
          <button
            class="btn btn-primary btn-sm"
            :disabled="organizing"
            @click="organizeLibrary('move')"
          >
            {{ organizing ? 'Organizing…' : 'Organize + Move' }}
          </button>
          <button
            class="btn btn-ghost btn-sm"
            :disabled="organizing"
            @click="organizeLibrary('copy')"
          >
            Copy Instead
          </button>
        </div>
        <div v-if="organizePreview" class="preview-panel">
          <div class="preview-summary">
            <strong>{{ organizePreview.moved }}</strong> erkannt,
            <strong>{{ organizePreview.skipped }}</strong> übersprungen
          </div>
          <div class="preview-list">
            <div
              v-for="item in organizePreview.items?.slice(0, 8)"
              :key="`${item.source_path}-${item.target_path}`"
              class="preview-item"
            >
              <div class="preview-source">{{ item.source_path }}</div>
              <div v-if="item.skipped" class="preview-skip">Skip: {{ item.reason }}</div>
              <div v-else class="preview-target">{{ item.media_type }} → {{ item.target_path }}</div>
            </div>
          </div>
          <div v-if="(organizePreview.items?.length ?? 0) > 8" class="preview-more">
            … und {{ (organizePreview.items?.length ?? 0) - 8 }} weitere
          </div>
        </div>
      </div>

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
      <div class="rating-filter-row">
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
        <button
          class="btn btn-secondary btn-sm autotag-all-btn"
          :disabled="batchTagging"
          @click="autoTagAll"
          title="Auto-tag all untagged items"
        >
          {{ batchTagging ? 'Queuing…' : 'Auto-Tag Untagged' }}
        </button>
      </div>
    </aside>

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

      <div v-if="store.totalPages > 1" class="pagination">
        <button class="btn btn-ghost btn-sm" :disabled="store.currentPage <= 1" @click="store.prevPage()">← Previous</button>
        <span class="pagination-info">Page {{ store.currentPage }} of {{ store.totalPages }}</span>
        <button class="btn btn-ghost btn-sm" :disabled="store.currentPage >= store.totalPages" @click="store.nextPage()">Next →</button>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useMediaStore } from '@/stores/mediaStore'
import { autotagApi } from '@/api/autotagApi'
import { libraryApi, type OrganizeLibraryResult } from '@/api/libraryApi'
import MediaGrid from '@/components/Gallery/MediaGrid.vue'
import SearchBar from '@/components/Search/SearchBar.vue'

const store = useMediaStore()
const route = useRoute()

const batchTagging = ref(false)
const organizing = ref(false)
const organizePath = ref('inbox')
const organizePreview = ref<OrganizeLibraryResult | null>(null)

async function organizeLibrary(mode: 'move' | 'copy') {
  const sourcePath = organizePath.value.trim()
  if (!sourcePath) {
    alert('Bitte einen Unterordner innerhalb von /inbox angeben.')
    return
  }

  organizing.value = true
  try {
    const res = await libraryApi.organize(sourcePath, mode, false)
    organizePreview.value = null
    await store.fetchList()
    alert(`Organisiert: ${res.data.moved} Dateien, übersprungen: ${res.data.skipped}.`)
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : 'Library organize failed'
    alert(msg)
  } finally {
    organizing.value = false
  }
}

async function previewLibrary(mode: 'move' | 'copy') {
  const sourcePath = organizePath.value.trim()
  if (!sourcePath) {
    alert('Bitte einen Unterordner innerhalb von /inbox angeben.')
    return
  }

  organizing.value = true
  try {
    const res = await libraryApi.organize(sourcePath, mode, true)
    organizePreview.value = res.data
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : 'Library preview failed'
    alert(msg)
  } finally {
    organizing.value = false
  }
}

async function autoTagAll() {
  if (batchTagging.value) return
  batchTagging.value = true
  try {
    const res = await autotagApi.autotagBatch('all_untagged')
    await store.fetchList()
    alert(`Queued ${res.data.queued} items for auto-tagging.`)
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : 'Batch auto-tag failed'
    alert(msg)
  } finally {
    batchTagging.value = false
  }
}

const types = [
  { value: '', label: 'All' },
  { value: 'video', label: '🎬 Videos' },
  { value: 'image', label: '🖼️ Images' },
  { value: 'manga', label: '📖 Manga' },
  { value: 'comic', label: '📕 Comics' },
  { value: 'doujinshi', label: '📗 Doujin' },
]

const sortOptions = [
  { value: 'newest', label: '🕒 Newest' },
  { value: 'oldest', label: '🕰️ Oldest' },
  { value: 'title', label: '🔤 Title' },
  { value: 'rating', label: '⭐ Rating' },
  { value: 'size', label: '📦 Size' },
  { value: 'views', label: '👁️ Views' },
]

function onSearch(q: string) {
  store.setFilter('q', q)
}

function setMinRating(star: number) {
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
  width: 220px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.import-panel {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--bg-card);
}

.import-help {
  font-size: 12px;
  line-height: 1.4;
  color: var(--text-muted);
  margin: 0;
}

.import-help code {
  font-size: 11px;
}

.import-structure {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 8px;
  border-radius: var(--radius);
  background: var(--bg-surface);
  font-size: 11px;
  color: var(--text-muted);
}

.import-structure strong {
  color: var(--text-primary);
  font-size: 11px;
}

.import-structure code {
  font-size: 11px;
}

.import-input {
  width: 100%;
  background: var(--bg-surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  color: var(--text-primary);
  font-size: 13px;
  padding: 8px 10px;
}

.import-actions {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.preview-panel {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding-top: 4px;
  border-top: 1px solid var(--border);
}

.preview-summary,
.preview-more {
  font-size: 12px;
  color: var(--text-muted);
}

.preview-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.preview-item {
  padding: 8px;
  border-radius: var(--radius);
  background: var(--bg-surface);
}

.preview-source,
.preview-target,
.preview-skip {
  font-size: 11px;
  line-height: 1.4;
  word-break: break-word;
}

.preview-source {
  color: var(--text-primary);
}

.preview-target {
  color: var(--text-muted);
}

.preview-skip {
  color: #f59e0b;
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

.rating-filter-row {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

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
  gap: 8px;
}

.gallery-count { font-size: 13px; color: var(--text-muted); }
.autotag-all-btn { width: 100%; justify-content: center; }

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
