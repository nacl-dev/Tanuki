<template>
  <div class="library-page">
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
        <div class="library-collections__header-actions">
          <span v-if="pinnedCollectionIds.length" class="library-collections__summary">
            Pinned {{ pinnedCollectionIds.length }} / {{ maxPinnedCollections }}
          </span>
          <button
            v-if="collectionsExpanded || hiddenCollectionsCount > 0"
            type="button"
            class="btn btn-ghost btn-sm library-collections__toggle"
            @click="toggleCollectionsExpanded"
          >
            {{ collectionsExpanded ? 'Show less' : `More (${hiddenCollectionsCount})` }}
          </button>
        </div>
      </div>

      <div class="library-collections__grid">
        <article
          v-for="collection in visibleCollections"
          :key="collection.id"
          class="collection-card-shell"
        >
          <button
            type="button"
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
          <button
            type="button"
            :class="['collection-card__pin', { 'collection-card__pin--active': isCollectionPinned(collection.id) }]"
            :title="isCollectionPinned(collection.id) ? 'Unpin collection from Library' : 'Pin collection to Library'"
            :aria-pressed="isCollectionPinned(collection.id)"
            @click.stop="toggleCollectionPin(collection.id)"
          >
            <AppIcon name="pin" :size="13" :filled="isCollectionPinned(collection.id)" />
            <span>{{ isCollectionPinned(collection.id) ? 'Pinned' : 'Pin' }}</span>
          </button>
        </article>
      </div>

      <div v-if="expandedCollectionId" class="library-collections__expanded">
        <div class="library-collections__expanded-head">
          <h3>{{ expandedCollection?.name ?? 'Collection' }}</h3>
          <button class="btn btn-ghost btn-sm" @click="expandedCollectionId = ''">Close</button>
        </div>
        <MediaGrid
          :items="expandedCollection?.items ?? []"
          :loading="expandedCollectionLoading"
          :density="gridDensity"
          :show-tags="false"
        />
      </div>
    </section>

    <section v-if="quickShelves.length" class="library-shelves">
      <article
        v-for="shelf in quickShelves"
        :key="shelf.key"
        class="card shelf-card"
      >
        <div class="shelf-card__head">
          <div class="shelf-card__copy">
            <span class="gallery-count">{{ shelf.kicker }}</span>
            <h3>{{ shelf.title }}</h3>
          </div>
          <div class="shelf-card__overlay">
            <span class="shelf-card__count">{{ shelf.visibleCount }}</span>
            <button
              v-if="shelf.expandable"
              type="button"
              class="btn btn-ghost btn-sm shelf-card__more"
              @click="toggleShelf(shelf.key)"
            >
              {{ shelf.expanded ? 'Show less' : `Show more (${shelf.moreCount})` }}
            </button>
          </div>
        </div>
        <MediaGrid
          :items="shelf.items"
          :loading="false"
          :density="gridDensity"
          :show-tags="false"
          :compact-cards="true"
        />
      </article>
    </section>

    <section v-if="workGroups.length" class="library-workgroups">
      <article
        v-for="group in workGroups"
        :key="group.key"
        class="card shelf-card work-shelf"
      >
        <div class="shelf-card__head">
          <div class="shelf-card__copy">
            <span class="gallery-count">Work</span>
            <h3>{{ group.title }}</h3>
            <p class="work-shelf__copy">
              {{ group.items.length }} item{{ group.items.length === 1 ? '' : 's' }}
              <template v-if="group.items.some((item) => item.work_index > 0)"> · ordered</template>
            </p>
          </div>
          <div class="shelf-card__overlay">
            <span class="shelf-card__count">{{ group.items.length }}</span>
            <button
              type="button"
              class="btn btn-ghost btn-sm shelf-card__more"
              @click="toggleWorkGroup(group.key)"
            >
              {{ isWorkGroupExpanded(group.key) ? 'Hide items' : 'Open work' }}
            </button>
          </div>
        </div>

        <MediaGrid
          v-if="isWorkGroupExpanded(group.key)"
          :items="group.items"
          :loading="false"
          :density="gridDensity"
          :show-tags="false"
          :compact-cards="true"
        />

        <div v-else class="work-shelf__peek">
          <span
            v-for="item in previewWorkItems(group.items)"
            :key="item.id"
            class="work-shelf__chip"
          >
            {{ workItemLabel(item) }}
          </span>
          <span
            v-if="group.items.length > previewWorkItems(group.items).length"
            class="work-shelf__chip work-shelf__chip--muted"
          >
            +{{ group.items.length - previewWorkItems(group.items).length }} more
          </span>
        </div>
      </article>
    </section>

    <section class="gallery-section">
      <div class="gallery-header">
        <div class="gallery-controls">
          <span class="gallery-count">{{ galleryCountLabel }}</span>
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
      <div v-if="workGroups.length && !ungroupedItems.length && !store.loading" class="gallery-grouped-empty">
        All items on this page are grouped into works.
      </div>
      <MediaGrid
        v-else
        :items="galleryItems"
        :loading="store.loading"
        :density="gridDensity"
        :show-tags="false"
      />

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
import { useMediaStore } from '@/stores/mediaStore'
import MediaGrid from '@/components/Gallery/MediaGrid.vue'
import { collectionApi, type Collection } from '@/api/collectionApi'
import { mediaApi, mediaAssetUrl, type Media } from '@/api/mediaApi'
import { taskApi, type BackgroundTask } from '@/api/taskApi'
import AppIcon from '@/components/Layout/AppIcon.vue'
import { useAuthStore } from '@/stores/authStore'
import { useNoticeStore } from '@/stores/noticeStore'

const authStore = useAuthStore()
const store = useMediaStore()
const route = useRoute()
const { pushNotice } = useNoticeStore()
const hoveredRating = ref<number | null>(null)
const collections = ref<Collection[]>([])
const expandedCollectionId = ref('')
const expandedCollection = ref<Collection | null>(null)
const expandedCollectionLoading = ref(false)
const continueItems = ref<Media[]>([])
const recentItems = ref<Media[]>([])
const favoriteItems = ref<Media[]>([])
const tasks = ref<BackgroundTask[]>([])
const hadActiveTasks = ref(false)
const gridDensity = ref<'cozy' | 'compact'>('cozy')
const pinnedCollectionIds = ref<string[]>([])
const collectionsExpanded = ref(false)
const continueExpanded = ref(false)
const recentExpanded = ref(false)
const expandedWorkGroupKeys = ref<string[]>([])
let taskFallbackTimer: number | null = null
let taskStream: EventSource | null = null
const maxExpandedShelfItems = 6
const maxPinnedCollections = 2

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

const legacyPinnedCollectionsStorageKey = computed(() =>
  `tanuki_library_pinned_collections:${authStore.user?.id ?? authStore.user?.username ?? 'default'}`,
)

const orderedCollections = computed(() => {
  const collectionMap = new Map(collections.value.map((collection) => [collection.id, collection]))
  const pinned = pinnedCollectionIds.value
    .map((id) => collectionMap.get(id))
    .filter((collection): collection is Collection => Boolean(collection))
  const pinnedIds = new Set(pinned.map((collection) => collection.id))
  const others = collections.value.filter((collection) => !pinnedIds.has(collection.id))
  return [...pinned, ...others]
})

const featuredCollections = computed(() => orderedCollections.value.slice(0, maxPinnedCollections))
const visibleCollections = computed(() => collectionsExpanded.value ? orderedCollections.value : featuredCollections.value)
const hiddenCollectionsCount = computed(() => Math.max(orderedCollections.value.length - featuredCollections.value.length, 0))

const shelfPreviewLimit = computed(() => 2)
const expandedShelfCount = (items: Media[]) => Math.min(items.length, maxExpandedShelfItems)
const moreShelfCount = (items: Media[]) => Math.max(expandedShelfCount(items) - Math.min(items.length, shelfPreviewLimit.value), 0)

type WorkGroup = {
  key: string
  title: string
  items: Media[]
}

const workGroups = computed<WorkGroup[]>(() => {
  const grouped = new Map<string, WorkGroup>()
  for (const item of store.items) {
    const title = item.work_title?.trim()
    if (!title) continue
    const key = title.toLocaleLowerCase()
    const existing = grouped.get(key)
    if (existing) {
      existing.items.push(item)
      continue
    }
    grouped.set(key, {
      key,
      title,
      items: [item],
    })
  }

  return Array.from(grouped.values())
    .map((group) => ({
      ...group,
      items: [...group.items].sort(compareWorkItems),
    }))
    .sort((a, b) => a.title.localeCompare(b.title, undefined, { sensitivity: 'base' }))
})

const ungroupedItems = computed(() =>
  store.items.filter((item) => !item.work_title?.trim()),
)

const galleryItems = computed(() => (workGroups.value.length ? ungroupedItems.value : store.items))
const galleryCountLabel = computed(() => {
  if (!workGroups.value.length) {
    return `${store.total} items`
  }
  if (!ungroupedItems.value.length) {
    return 'All items grouped into works'
  }
  return `${ungroupedItems.value.length} ungrouped items`
})

const quickShelves = computed(() => [
  {
    key: 'continue',
    kicker: 'Continue',
    title: 'Pick up where you left off',
    items: continueExpanded.value
      ? continueItems.value.slice(0, maxExpandedShelfItems)
      : continueItems.value.slice(0, shelfPreviewLimit.value),
    visibleCount: continueExpanded.value
      ? expandedShelfCount(continueItems.value)
      : Math.min(continueItems.value.length, shelfPreviewLimit.value),
    totalCount: continueItems.value.length,
    expandable: continueItems.value.length > shelfPreviewLimit.value,
    expanded: continueExpanded.value,
    moreCount: moreShelfCount(continueItems.value),
  },
  {
    key: 'favorites',
    kicker: 'Favorites',
    title: 'Fast access to your saved picks',
    items: favoriteItems.value,
    visibleCount: favoriteItems.value.length,
    totalCount: favoriteItems.value.length,
    expandable: false,
    expanded: false,
    moreCount: 0,
  },
  {
    key: 'recent',
    kicker: 'Recent',
    title: 'Newest additions to the vault',
    items: recentExpanded.value
      ? recentItems.value.slice(0, maxExpandedShelfItems)
      : recentItems.value.slice(0, shelfPreviewLimit.value),
    visibleCount: recentExpanded.value
      ? expandedShelfCount(recentItems.value)
      : Math.min(recentItems.value.length, shelfPreviewLimit.value),
    totalCount: recentItems.value.length,
    expandable: recentItems.value.length > shelfPreviewLimit.value,
    expanded: recentExpanded.value,
    moreCount: moreShelfCount(recentItems.value),
  },
].filter((section) => section.items.length > 0))
const recentTasks = computed(() =>
  tasks.value
    .filter((task) => task.status !== 'completed')
    .slice(0, 4),
)

function sanitizePinnedCollectionIds(ids: string[]) {
  return ids
    .map((id) => typeof id === 'string' ? id.trim() : '')
    .filter((id, index, all) => id !== '' && all.indexOf(id) === index)
    .slice(0, maxPinnedCollections)
}

function normalizePinnedCollectionIds(ids: string[]) {
  const validCollectionIds = new Set(collections.value.map((collection) => collection.id))
  return sanitizePinnedCollectionIds(ids).filter((id) => validCollectionIds.has(id))
}

function readLegacyPinnedCollectionIds() {
  if (typeof window === 'undefined') return []
  try {
    const raw = window.localStorage.getItem(legacyPinnedCollectionsStorageKey.value)
    if (!raw) return []
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed)
      ? parsed.filter((value): value is string => typeof value === 'string')
      : []
  } catch {
    return []
  }
}

function clearLegacyPinnedCollectionIds() {
  if (typeof window === 'undefined') return
  window.localStorage.removeItem(legacyPinnedCollectionsStorageKey.value)
}

function hydratePinnedCollectionIdsFromAccount() {
  pinnedCollectionIds.value = sanitizePinnedCollectionIds(authStore.user?.library_pinned_collection_ids ?? [])
}

async function persistPinnedCollectionIds(nextIds: string[], options?: { clearLegacy?: boolean; silent?: boolean }) {
  if (!authStore.user) return

  const previousIds = [...pinnedCollectionIds.value]
  pinnedCollectionIds.value = nextIds

  try {
    await authStore.updateLibraryPins(nextIds)
    if (options?.clearLegacy) {
      clearLegacyPinnedCollectionIds()
    }
  } catch (error) {
    pinnedCollectionIds.value = previousIds
    if (!options?.silent) {
      pushNotice({
        type: 'error',
        message: error instanceof Error ? error.message : 'Failed to save pinned collections',
      })
    }
    throw error
  }
}

async function syncPinnedCollectionIds() {
  const next = normalizePinnedCollectionIds(pinnedCollectionIds.value)
  if (JSON.stringify(next) !== JSON.stringify(pinnedCollectionIds.value)) {
    await persistPinnedCollectionIds(next, { silent: true })
  }
}

async function migrateLegacyPinnedCollectionIds() {
  if (authStore.user?.library_pinned_collection_ids?.length) {
    clearLegacyPinnedCollectionIds()
    return
  }

  const next = normalizePinnedCollectionIds(readLegacyPinnedCollectionIds())
  if (!next.length) return

  await persistPinnedCollectionIds(next, { clearLegacy: true, silent: true })
  hydratePinnedCollectionIdsFromAccount()
}

function isCollectionPinned(id: string) {
  return pinnedCollectionIds.value.includes(id)
}

async function toggleCollectionPin(id: string) {
  const nextIds = isCollectionPinned(id)
    ? pinnedCollectionIds.value.filter((pinnedId) => pinnedId !== id)
    : [...pinnedCollectionIds.value, id]

  if (!isCollectionPinned(id) && pinnedCollectionIds.value.length >= maxPinnedCollections) {
    pushNotice({
      type: 'info',
      message: 'You can pin up to two collections on the Library page.',
    })
    return
  }

  try {
    await persistPinnedCollectionIds(sanitizePinnedCollectionIds(nextIds))
  } catch {
    hydratePinnedCollectionIdsFromAccount()
  }
}

function toggleCollectionsExpanded() {
  collectionsExpanded.value = !collectionsExpanded.value

  if (!collectionsExpanded.value) {
    const visibleIds = new Set(featuredCollections.value.map((collection) => collection.id))
    if (expandedCollectionId.value && !visibleIds.has(expandedCollectionId.value)) {
      expandedCollectionId.value = ''
      expandedCollection.value = null
    }
  }
}

async function loadCollections() {
  const res = await collectionApi.list()
  collections.value = res.data ?? []
  hydratePinnedCollectionIdsFromAccount()
  await syncPinnedCollectionIds()
  await migrateLegacyPinnedCollectionIds()
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
    mediaApi.list({ limit: 12, sort: 'newest' }),
  ])
  continueItems.value = continueRes.data ?? []
  favoriteItems.value = favoriteRes.data ?? []
  recentItems.value = recentRes.data ?? []
}

function toggleShelf(key: string) {
  if (key === 'continue') {
    continueExpanded.value = !continueExpanded.value
    return
  }

  if (key === 'recent') {
    recentExpanded.value = !recentExpanded.value
  }
}

function toggleWorkGroup(key: string) {
  if (expandedWorkGroupKeys.value.includes(key)) {
    expandedWorkGroupKeys.value = expandedWorkGroupKeys.value.filter((item) => item !== key)
    return
  }
  expandedWorkGroupKeys.value = [...expandedWorkGroupKeys.value, key]
}

function isWorkGroupExpanded(key: string) {
  return expandedWorkGroupKeys.value.includes(key)
}

function previewWorkItems(items: Media[]) {
  return items.slice(0, 3)
}

function workItemLabel(item: Media) {
  if (item.work_index > 0) {
    return `#${item.work_index} ${item.title}`
  }
  return item.title
}

function compareWorkItems(a: Media, b: Media) {
  const aIndex = a.work_index > 0 ? a.work_index : Number.MAX_SAFE_INTEGER
  const bIndex = b.work_index > 0 ? b.work_index : Number.MAX_SAFE_INTEGER
  if (aIndex !== bIndex) {
    return aIndex - bIndex
  }
  return a.title.localeCompare(b.title, undefined, { sensitivity: 'base' })
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

function setGridDensity(next: 'cozy' | 'compact') {
  gridDensity.value = next
  window.localStorage.setItem('tanuki_grid_density', next)
}

function ensureTaskFallbackPolling() {
  if (taskFallbackTimer !== null) return
  taskFallbackTimer = window.setInterval(() => {
    void loadTasks()
  }, 4000)
}

function stopTaskFallbackPolling() {
  if (taskFallbackTimer !== null) {
    window.clearInterval(taskFallbackTimer)
    taskFallbackTimer = null
  }
}

function startTaskStream() {
  if (typeof window === 'undefined' || typeof EventSource === 'undefined') {
    ensureTaskFallbackPolling()
    return
  }

  taskStream = new EventSource(taskApi.streamUrl(8))
  taskStream.onopen = () => {
    stopTaskFallbackPolling()
  }
  taskStream.onmessage = (event) => {
    try {
      void applyTasks(JSON.parse(event.data) as BackgroundTask[])
    } catch {
      // Ignore malformed frames and wait for the next snapshot.
    }
  }
  taskStream.onerror = () => {
    ensureTaskFallbackPolling()
  }
}

function stopTaskStream() {
  if (taskStream === null) return
  taskStream.close()
  taskStream = null
}

async function applyTasks(nextTasks: BackgroundTask[]) {
  tasks.value = nextTasks

  const hasActiveTasks = nextTasks.some((task) => task.status === 'queued' || task.status === 'running')
  if (hasActiveTasks) {
    hadActiveTasks.value = true
    return
  }

  if (hadActiveTasks.value) {
    hadActiveTasks.value = false
    await refreshLibrarySurfaces()
  }
}

async function loadTasks() {
  try {
    await applyTasks(await taskApi.list(8))
  } catch {
    return
  }
}

watch(
  () => authStore.user?.library_pinned_collection_ids,
  () => {
    hydratePinnedCollectionIdsFromAccount()
  },
  { immediate: true },
)

watch(workGroups, (groups) => {
  const valid = new Set(groups.map((group) => group.key))
  expandedWorkGroupKeys.value = expandedWorkGroupKeys.value.filter((key) => valid.has(key))
})

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
  void loadTasks().finally(() => {
    startTaskStream()
  })
})

onBeforeUnmount(() => {
  stopTaskFallbackPolling()
  stopTaskStream()
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


.filter-bar {
  display: flex;
  align-items: center;
  gap: 20px;
  flex-wrap: wrap;
  position: sticky;
  top: 0;
  z-index: 12;
  isolation: isolate;
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

.library-workgroups {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.library-shelves {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 14px;
}

.shelf-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
}

.shelf-card__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.shelf-card__copy {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.shelf-card__overlay {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.shelf-card__head h3 {
  margin-top: 2px;
  font-size: 15px;
  line-height: 1.2;
}

.shelf-card__count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 0;
  padding: 5px 10px;
  border-radius: 999px;
  background: var(--bg-surface);
  border: 1px solid var(--border);
  color: var(--text-secondary);
  font-size: 11px;
  font-weight: 500;
  line-height: 1;
}

.work-shelf__copy {
  margin: 0;
  font-size: 12px;
  color: var(--text-muted);
}

.work-shelf__peek {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.work-shelf__chip {
  display: inline-flex;
  align-items: center;
  padding: 4px 8px;
  border-radius: 999px;
  border: 1px solid var(--border);
  background: var(--bg-surface);
  color: var(--text-secondary);
  font-size: 12px;
}

.work-shelf__chip--muted {
  color: var(--text-muted);
}

.shelf-card__more {
  min-height: 28px;
  padding: 5px 10px;
  border-radius: 999px;
  font-size: 11px;
}

.library-collections__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.library-collections__header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.library-collections__summary {
  display: inline-flex;
  align-items: center;
  min-height: 28px;
  padding: 0 10px;
  border-radius: 999px;
  background: var(--bg-surface);
  border: 1px solid var(--border);
  color: var(--text-secondary);
  font-size: 12px;
}

.library-collections__toggle {
  min-height: 28px;
  padding: 4px 10px;
  border-radius: 999px;
  font-size: 12px;
}

.library-collections__grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 14px;
}

.collection-card-shell {
  position: relative;
}

.collection-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  width: 100%;
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

.collection-card__pin {
  position: absolute;
  top: 10px;
  right: 10px;
  z-index: 2;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 30px;
  padding: 0 10px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 999px;
  background: rgba(0, 0, 0, 0.66);
  color: #fff;
  cursor: pointer;
  font-size: 11px;
}

.collection-card__pin--active {
  border-color: rgba(245, 158, 11, 0.3);
  background: rgba(245, 158, 11, 0.16);
  color: #f4c06a;
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

.gallery-grouped-empty {
  padding: 24px;
  border: 1px dashed var(--border);
  border-radius: var(--radius);
  color: var(--text-muted);
  text-align: center;
  background: var(--bg-surface);
}

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
  .filter-bar {
    gap: 16px;
  }
}

@media (max-width: 720px) {
  .filter-bar {
    position: static;
    padding: 10px;
    align-items: flex-start;
  }

  .filter-options {
    width: 100%;
  }

  .filter-options--inline {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .filter-chip {
    width: auto;
  }

  .rating-filter {
    width: 100%;
    flex-wrap: wrap;
    justify-content: flex-start;
    gap: 8px;
  }

  .sort-select-wrap {
    width: auto;
    margin-left: auto;
  }

  .sort-select {
    width: auto;
    min-width: 112px;
    padding: 7px 30px 7px 14px;
  }

  .rating-star {
    font-size: 16px;
  }

  .clear-rating {
    margin-left: 0;
  }

  .gallery-controls {
    align-items: stretch;
  }

  .density-toggle {
    width: 100%;
  }

  .density-chip {
    flex: 1;
    text-align: center;
  }

  .pagination {
    flex-direction: column;
    gap: 10px;
  }

  .pagination .btn {
    width: 100%;
    justify-content: center;
  }
}

@media (max-width: 480px) {
  .library-collections__expanded-head {
    width: 100%;
    flex-direction: column;
    align-items: stretch;
  }

  .library-collections__grid,
  .library-shelves,
  .library-tasks {
    grid-template-columns: 1fr;
  }

  .library-collections__header {
    flex-direction: row;
    align-items: center;
    justify-content: space-between;
  }

  .library-collections__header-actions {
    margin-left: auto;
    justify-content: flex-end;
  }

  .collection-card__pin span {
    display: none;
  }

  .shelf-card__overlay {
    width: 100%;
    justify-content: space-between;
  }

  .rating-filter {
    gap: 6px;
  }

  .sort-select {
    min-width: 100px;
    padding-inline: 12px 28px;
  }
}
</style>
