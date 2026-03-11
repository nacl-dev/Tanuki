<template>
  <div class="library-page">
    <section class="library-hero card">
      <div class="library-hero__copy">
        <span class="library-hero__eyebrow">Library</span>
        <h1>Browse, continue and organize your vault.</h1>
        <p>Global maintenance actions live here now instead of floating across every page.</p>
      </div>
      <div class="library-hero__actions">
        <button class="btn btn-ghost" :disabled="tagging" @click="runBatchAutoTag">
          {{ tagging ? 'Queueing…' : 'Auto-tag Untagged' }}
        </button>
        <button class="btn btn-primary" :disabled="scanning" @click="scanLibrary">
          {{ scanning ? 'Queueing…' : 'Scan Library' }}
        </button>
      </div>
    </section>

    <section v-if="recentTasks.length" class="library-tasks">
      <article
        v-for="task in recentTasks"
        :key="task.id"
        class="card task-card"
        :class="`task-card--${task.status}`"
      >
        <div class="task-card__head">
          <div>
            <span class="gallery-count">{{ formatTaskKind(task.kind) }}</span>
            <h3>{{ task.message || 'Background task' }}</h3>
          </div>
          <span class="task-status">{{ task.status }}</span>
        </div>

        <div class="task-progress">
          <div class="task-progress__bar">
            <span class="task-progress__fill" :style="{ width: `${task.percent || 0}%` }" />
          </div>
          <span class="task-progress__meta">
            {{ task.total ? `${task.completed} / ${task.total}` : task.status === 'running' ? 'Running' : 'Idle' }}
          </span>
        </div>

        <p v-if="task.error" class="task-error">{{ task.error }}</p>
      </article>
    </section>

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

      <div class="rating-filter" @mouseleave="hoveredRating = null">
        <button
          v-for="star in 5"
          :key="star"
          type="button"
          class="rating-star"
          :class="{ active: (hoveredRating ?? store.filters.min_rating ?? 0) >= star }"
          :aria-label="`Minimum ${star} star${star === 1 ? '' : 's'}`"
          :aria-pressed="store.filters.min_rating === star"
          @click="setMinRating(star)"
          @mouseenter="hoveredRating = star"
          title="Minimum rating"
        >★</button>
        <button
          v-if="store.filters.min_rating"
          type="button"
          class="clear-rating"
          aria-label="Clear minimum rating"
          @click="store.setFilter('min_rating', undefined)"
        >
          <AppIcon name="close" :size="11" />
        </button>
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

    <section v-if="collections.length" class="library-collections">
      <div class="library-collections__header">
        <span class="gallery-count">Collections</span>
      </div>

      <div class="library-collections__grid">
        <button
          v-for="collection in collections"
          :key="collection.id"
          :class="['collection-card', { active: expandedCollectionId === collection.id }]"
          @click="toggleCollection(collection.id)"
        >
          <div class="collection-card__preview" v-if="collection.items?.length">
            <div class="preview-stack">
              <div
                v-for="(item, index) in previewItems(collection)"
                :key="item.id"
                class="preview-tile"
                :class="{ 'preview-tile--lead': index === 0 }"
                :style="previewTileStyle(index, previewItems(collection).length)"
              >
                <img
                  class="preview-image"
                  :src="mediaAssetUrl(item.id, 'thumbnail', item.updated_at)"
                  :alt="item.title"
                  loading="lazy"
                />
              </div>
            </div>
          </div>
          <div class="collection-card__meta">
            <span class="collection-card__name">{{ collection.name }}</span>
            <small>{{ collection.item_count }}</small>
          </div>
        </button>
      </div>

      <div v-if="expandedCollectionId" class="library-collections__expanded">
        <div class="library-collections__expanded-head">
          <h3>{{ expandedCollection?.name ?? 'Collection' }}</h3>
          <button class="btn btn-ghost btn-sm" @click="expandedCollectionId = ''">Close</button>
        </div>
        <MediaGrid :items="expandedCollection?.items ?? []" :loading="expandedCollectionLoading" :density="gridDensity" />
      </div>
    </section>

    <section v-if="quickShelves.length" class="library-shelves">
      <article
        v-for="shelf in quickShelves"
        :key="shelf.key"
        class="card shelf-card"
      >
        <div class="shelf-card__head">
          <div>
            <span class="gallery-count">{{ shelf.kicker }}</span>
            <h3>{{ shelf.title }}</h3>
          </div>
        </div>
        <div class="shelf-card__meta">
          <span class="shelf-card__count">{{ shelf.visibleCount }}</span>
          <button
            v-if="shelf.expandable"
            type="button"
            class="btn btn-ghost btn-sm"
            @click="toggleContinueShelf"
          >
            {{ continueExpanded ? 'Show less' : `Show more (${shelf.totalCount})` }}
          </button>
        </div>
        <MediaGrid :items="shelf.items" :loading="false" :density="gridDensity" />
      </article>
    </section>

    <section class="gallery-section">
      <div class="gallery-header">
        <div class="gallery-controls">
          <span class="gallery-count">{{ store.total }} items</span>
          <div class="density-toggle" role="group" aria-label="Grid density">
            <button
              type="button"
              :class="['density-chip', { active: gridDensity === 'cozy' }]"
              :aria-pressed="gridDensity === 'cozy'"
              @click="setGridDensity('cozy')"
            >
              Cozy
            </button>
            <button
              type="button"
              :class="['density-chip', { active: gridDensity === 'compact' }]"
              :aria-pressed="gridDensity === 'compact'"
              @click="setGridDensity('compact')"
            >
              Compact
            </button>
          </div>
        </div>
      </div>
      <MediaGrid :items="store.items" :loading="store.loading" :density="gridDensity" />

      <div v-if="store.totalPages > 1" class="pagination">
        <button class="btn btn-ghost btn-sm" :disabled="store.currentPage <= 1" @click="store.prevPage()">← Previous</button>
        <span class="pagination-info">Page {{ store.currentPage }} of {{ store.totalPages }}</span>
        <button class="btn btn-ghost btn-sm" :disabled="store.currentPage >= store.totalPages" @click="store.nextPage()">Next →</button>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { autotagApi } from '@/api/autotagApi'
import { useMediaStore } from '@/stores/mediaStore'
import MediaGrid from '@/components/Gallery/MediaGrid.vue'
import { collectionApi, type Collection } from '@/api/collectionApi'
import { libraryApi } from '@/api/libraryApi'
import { mediaApi, mediaAssetUrl, type Media } from '@/api/mediaApi'
import { taskApi, type BackgroundTask } from '@/api/taskApi'
import AppIcon from '@/components/Layout/AppIcon.vue'
import { useNoticeStore } from '@/stores/noticeStore'

const store = useMediaStore()
const route = useRoute()
const { pushNotice } = useNoticeStore()
const hoveredRating = ref<number | null>(null)
const collections = ref<Collection[]>([])
const expandedCollectionId = ref('')
const expandedCollection = ref<Collection | null>(null)
const expandedCollectionLoading = ref(false)
const scanning = ref(false)
const tagging = ref(false)
const continueItems = ref<Media[]>([])
const recentItems = ref<Media[]>([])
const favoriteItems = ref<Media[]>([])
const tasks = ref<BackgroundTask[]>([])
const hadActiveTasks = ref(false)
const gridDensity = ref<'cozy' | 'compact'>('cozy')
const continueExpanded = ref(false)
let taskPollTimer: number | null = null

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

function previewItems(collection: Collection) {
  return (collection.items ?? []).slice(0, 5)
}

function previewTileStyle(index: number, count: number) {
  if (index === 0) {
    return {
      left: '0%',
      width: '42%',
      zIndex: count + 1,
    }
  }

  const trailing = Math.max(count - 1, 1)
  const slotWidth = 58 / Math.min(trailing, 4)
  return {
    left: `${42 + (index - 1) * slotWidth}%`,
    width: `${slotWidth}%`,
    zIndex: count - index,
  }
}

function toggleCollection(id: string) {
  if (expandedCollectionId.value === id) {
    expandedCollectionId.value = ''
    expandedCollection.value = null
    return
  }
  expandedCollectionId.value = id
  void loadExpandedCollection(id)
}
const quickShelves = computed(() => [
  {
    key: 'continue',
    kicker: 'Continue',
    title: 'Pick up where you left off',
    items: continueExpanded.value ? continueItems.value : continueItems.value.slice(0, 3),
    visibleCount: continueExpanded.value ? continueItems.value.length : Math.min(continueItems.value.length, 3),
    totalCount: continueItems.value.length,
    expandable: continueItems.value.length > 3,
  },
  {
    key: 'favorites',
    kicker: 'Favorites',
    title: 'Fast access to your saved picks',
    items: favoriteItems.value,
    visibleCount: favoriteItems.value.length,
    totalCount: favoriteItems.value.length,
    expandable: false,
  },
  {
    key: 'recent',
    kicker: 'Recent',
    title: 'Newest additions to the vault',
    items: recentItems.value.slice(0, 3),
    visibleCount: Math.min(recentItems.value.length, 3),
    totalCount: recentItems.value.length,
    expandable: false,
  },
].filter((section) => section.items.length > 0))
const recentTasks = computed(() =>
  tasks.value
    .filter((task) => task.status !== 'completed')
    .slice(0, 4),
)

async function loadCollections() {
  const res = await collectionApi.list()
  collections.value = res.data ?? []
}

async function loadExpandedCollection(id: string) {
  expandedCollectionLoading.value = true
  try {
    const res = await collectionApi.get(id)
    if (expandedCollectionId.value !== id) return
    expandedCollection.value = {
      ...res.data,
      items: res.data.items ?? [],
    }
  } catch (error) {
    pushNotice({
      type: 'error',
      message: error instanceof Error ? error.message : 'Failed to load collection items',
    })
    if (expandedCollectionId.value === id) {
      expandedCollectionId.value = ''
      expandedCollection.value = null
    }
  } finally {
    if (expandedCollectionId.value === id) {
      expandedCollectionLoading.value = false
    }
  }
}

async function loadQuickShelves() {
  const [continueRes, favoriteRes, recentRes] = await Promise.all([
    mediaApi.list({ limit: 12, in_progress: true, sort: 'newest' }),
    mediaApi.list({ limit: 6, favorite: true, sort: 'rating' }),
    mediaApi.list({ limit: 3, sort: 'newest' }),
  ])
  continueItems.value = continueRes.data ?? []
  favoriteItems.value = favoriteRes.data ?? []
  recentItems.value = recentRes.data ?? []
}

function toggleContinueShelf() {
  continueExpanded.value = !continueExpanded.value
}

async function refreshLibrarySurfaces() {
  await Promise.all([
    store.fetchList(),
    loadCollections(),
    loadQuickShelves(),
  ])
}

function formatTaskKind(kind: string) {
  switch (kind) {
    case 'library.scan':
      return 'Library Scan'
    case 'library.organize':
      return 'Organize'
    case 'media.autotag_batch':
      return 'Auto-tag'
    default:
      return kind
  }
}

function stopTaskPolling() {
  if (taskPollTimer !== null) {
    window.clearInterval(taskPollTimer)
    taskPollTimer = null
  }
}

function setGridDensity(next: 'cozy' | 'compact') {
  gridDensity.value = next
  window.localStorage.setItem('tanuki_grid_density', next)
}

function ensureTaskPolling() {
  if (taskPollTimer !== null) return
  taskPollTimer = window.setInterval(() => {
    void loadTasks()
  }, 4000)
}

async function loadTasks() {
  let nextTasks: BackgroundTask[] = []
  try {
    nextTasks = await taskApi.list(8)
  } catch {
    stopTaskPolling()
    return
  }
  tasks.value = nextTasks

  const hasActiveTasks = nextTasks.some((task) => task.status === 'queued' || task.status === 'running')
  if (hasActiveTasks) {
    hadActiveTasks.value = true
    ensureTaskPolling()
    return
  }

  stopTaskPolling()
  if (hadActiveTasks.value) {
    hadActiveTasks.value = false
    await refreshLibrarySurfaces()
  }
}

async function scanLibrary() {
  if (scanning.value) return
  scanning.value = true
  try {
    const response = await libraryApi.scan()
    pushNotice({
      type: 'success',
      message: `Library scan queued (${response.data.task_id.slice(0, 8)})`,
    })
    await loadTasks()
  } catch (error) {
    pushNotice({
      type: 'error',
      message: error instanceof Error ? error.message : 'Failed to queue library scan',
    })
  } finally {
    scanning.value = false
  }
}

async function runBatchAutoTag() {
  if (tagging.value) return
  tagging.value = true
  try {
    const response = await autotagApi.autotagBatch('all_untagged')
    pushNotice({
      type: 'success',
      message: `Auto-tag batch queued (${response.data.queued} items)`,
    })
    await loadTasks()
  } catch (error) {
    pushNotice({
      type: 'error',
      message: error instanceof Error ? error.message : 'Failed to queue auto-tag batch',
    })
  } finally {
    tagging.value = false
  }
}

onMounted(() => {
  const savedDensity = window.localStorage.getItem('tanuki_grid_density')
  if (savedDensity === 'cozy' || savedDensity === 'compact') {
    gridDensity.value = savedDensity
  }
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
  void loadCollections()
  void loadQuickShelves()
  void loadTasks()
})

onBeforeUnmount(() => {
  stopTaskPolling()
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
  gap: 14px;
}

.library-tasks {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 14px;
}

.library-hero {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 20px;
  background:
    radial-gradient(circle at top right, rgba(245, 158, 11, 0.16), transparent 30%),
    linear-gradient(135deg, rgba(255,255,255,0.02), rgba(255,255,255,0)),
    var(--bg-card);
}

.task-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.task-card--running,
.task-card--queued {
  border-color: color-mix(in srgb, var(--accent) 35%, var(--border));
}

.task-card--failed {
  border-color: rgba(248, 113, 113, 0.35);
}

.task-card__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.task-card__head h3 {
  margin-top: 4px;
  font-size: 15px;
}

.task-status {
  display: inline-flex;
  align-items: center;
  padding: 4px 8px;
  border-radius: 999px;
  background: var(--bg-surface);
  color: var(--text-secondary);
  font-size: 11px;
  text-transform: capitalize;
}

.task-progress {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.task-progress__bar {
  width: 100%;
  height: 8px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.06);
  overflow: hidden;
}

.task-progress__fill {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, var(--accent), color-mix(in srgb, var(--accent) 55%, white));
  transition: width 0.2s ease;
}

.task-progress__meta {
  font-size: 12px;
  color: var(--text-muted);
}

.task-error {
  margin: 0;
  color: #fca5a5;
  font-size: 12px;
}

.library-hero__copy {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.library-hero__eyebrow {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--accent);
}

.library-hero__copy h1 {
  font-size: clamp(22px, 3vw, 30px);
  line-height: 1.1;
}

.library-hero__copy p {
  max-width: 560px;
  color: var(--text-muted);
}

.library-hero__actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  justify-content: flex-end;
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
  appearance: none;
  border: none;
  background: transparent;
  cursor: pointer;
  font-size: 18px;
  color: var(--text-muted);
  transition: color 0.1s;
  padding: 0;
  line-height: 1;
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

.library-collections {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.library-shelves {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 16px;
}

.shelf-card {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.shelf-card__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.shelf-card__meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.shelf-card__head h3 {
  margin-top: 4px;
  font-size: 16px;
}

.shelf-card__count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 28px;
  padding: 4px 8px;
  border-radius: 999px;
  background: var(--bg-surface);
  color: var(--text-secondary);
  font-size: 12px;
}

.library-collections__header {
  display: flex;
  align-items: center;
}

.library-collections__grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 14px;
}

.collection-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 10px;
  border-radius: 12px;
  border: 1px solid var(--border);
  background: var(--bg-card);
  color: var(--text-primary);
  cursor: pointer;
  text-align: left;
  transition: border-color 0.15s, transform 0.15s, background 0.15s;
}

.collection-card:hover {
  border-color: var(--accent);
  transform: translateY(-1px);
}

.collection-card.active {
  border-color: var(--accent);
  background: var(--accent-dimmed);
}

.collection-card__preview {
  width: 100%;
}

.preview-stack {
  position: relative;
  width: 100%;
  height: 82px;
}

.preview-tile {
  position: absolute;
  top: 0;
  bottom: 0;
  overflow: hidden;
  border-radius: 10px;
  border: 1px solid color-mix(in srgb, var(--border) 75%, transparent);
  background: var(--bg-card);
  box-shadow: 0 10px 24px rgba(0, 0, 0, 0.18);
}

.preview-tile--lead {
  min-width: 92px;
}

.preview-image {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.collection-card__meta {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 10px;
}

.collection-card__name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.collection-card small {
  color: var(--text-muted);
  flex-shrink: 0;
  padding: 2px 8px;
  border-radius: 999px;
  background: var(--bg-surface);
}

.library-collections__expanded {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 16px;
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background: color-mix(in srgb, var(--bg-card) 96%, transparent);
}

.library-collections__expanded-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.library-collections__expanded-head h3 {
  font-size: 16px;
}

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
  flex-wrap: wrap;
}

.gallery-count { font-size: 13px; color: var(--text-muted); }

.density-toggle {
  display: inline-flex;
  gap: 8px;
}

.density-chip {
  appearance: none;
  border: 1px solid var(--border);
  background: var(--bg-surface);
  color: var(--text-secondary);
  padding: 6px 10px;
  border-radius: 999px;
  font-size: 12px;
  cursor: pointer;
}

.density-chip.active {
  border-color: var(--accent);
  background: var(--accent-dimmed);
  color: var(--accent);
}

.pagination {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 16px;
  padding-top: 16px;
}

.pagination-info { font-size: 13px; color: var(--text-muted); }

@media (max-width: 960px) {
  .library-hero {
    flex-direction: column;
    align-items: flex-start;
  }

  .library-hero__actions {
    width: 100%;
    justify-content: stretch;
  }

  .library-hero__actions .btn {
    flex: 1;
    justify-content: center;
  }

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
