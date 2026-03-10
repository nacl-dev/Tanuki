<template>
  <div v-if="loading" class="loading">Loading…</div>

  <!-- Manga Reader (fullscreen overlay) -->
  <MangaReader
    v-else-if="media && showReader && pages"
    :mediaId="media.id"
    :totalPages="pages.total_pages"
    :pages="pages.pages"
    :initialPage="media.read_progress || 0"
    @close="showReader = false"
    @pagechange="onPageChange"
  />

  <div v-else-if="media" class="media-detail">
    <!-- Header -->
    <div class="detail-header">
      <button class="btn btn-ghost btn-sm" @click="router.back()">← Back</button>
      <h1 class="detail-title">{{ media.title }}</h1>
      <div class="header-nav">
        <button
          class="btn btn-ghost btn-sm"
          :disabled="!hasPrev"
          @click="goToPrev"
          title="Previous media"
        >‹ Prev</button>
        <button
          class="btn btn-ghost btn-sm"
          :disabled="!hasNext"
          @click="goToNext"
          title="Next media"
        >Next ›</button>
      </div>
      <button
        :class="['btn btn-ghost', { 'fav-active': media.favorite }]"
        @click="toggleFav"
      >♥ {{ media.favorite ? 'Unfavorite' : 'Favorite' }}</button>
    </div>

    <!-- Body -->
    <div class="detail-body">
      <!-- Preview -->
      <div class="detail-preview">
        <VideoPlayer
          v-if="media.type === 'video'"
          :src="`/api/media/${media.id}/file`"
          :poster="media.thumbnail_path ? `/api/media/${media.id}/thumbnail` : undefined"
          @timeupdate="onVideoTimeUpdate"
          @ended="onVideoEnded"
        />
        <img
          v-else-if="media.type === 'image' && !imgError"
          :src="`/api/media/${media.id}/file`"
          :alt="media.title"
          class="media-image"
          @error="onImgError"
        />
        <!-- Archive preview + read button -->
        <div v-else-if="isArchive && pages" class="archive-preview">
          <img
            v-if="!imgError"
            :src="`/api/media/${media.id}/thumbnail`"
            :alt="media.title"
            class="media-image archive-thumb"
            @error="onImgError"
          />
          <div v-else class="media-placeholder">{{ typeIcon(media.type) }}</div>
          <button class="btn btn-primary read-btn" @click="openReader">
            📖 Read
          </button>
        </div>
        <div v-else class="media-placeholder">{{ typeIcon(media.type) }}</div>
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
          <!-- Auto-Tag button -->
          <button
            class="btn btn-secondary btn-sm autotag-btn"
            :disabled="autoTagging"
            @click="runAutoTag"
          >
            {{ autoTagging ? '⏳ Tagging…' : '🏷️ Auto-Tag' }}
          </button>
        </div>

        <!-- Duplicate warning -->
        <div v-if="duplicates.length > 0" class="meta-section">
          <span class="meta-label">Duplicates</span>
          <RouterLink :to="`/duplicates`" class="dup-warning">
            ⚠️ {{ duplicates.length }} duplicate{{ duplicates.length !== 1 ? 's' : '' }} found
          </RouterLink>
        </div>

        <!-- Details -->
        <div class="meta-section">
          <span class="meta-label">Details</span>
          <table class="meta-table">
            <tr><td>Type</td><td>{{ media.type }}</td></tr>
            <tr><td>Language</td><td>{{ media.language || '—' }}</td></tr>
            <tr><td>Views</td><td>{{ media.view_count }}</td></tr>
            <tr><td>Size</td><td>{{ formatBytes(media.file_size) }}</td></tr>
            <tr v-if="isArchive && pages">
              <td>Pages</td>
              <td>{{ pages.total_pages }}</td>
            </tr>
            <tr v-if="isArchive && media.read_progress > 0">
              <td>Progress</td>
              <td>Page {{ media.read_progress + 1 }} / {{ media.read_total || pages?.total_pages }}</td>
            </tr>
            <tr v-if="media.auto_tag_status === 'completed'">
              <td>Auto-tag</td>
              <td>{{ media.auto_tag_source }} ({{ media.auto_tag_similarity?.toFixed(1) }}%)</td>
            </tr>
            <tr><td>SHA-256</td><td class="meta-checksum">{{ media.checksum || '—' }}</td></tr>
            <tr><td>Added</td><td>{{ new Date(media.created_at).toLocaleDateString() }}</td></tr>
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

  <!-- Auto-Tag Result Dialog -->
  <AutoTagDialog
    v-if="autoTagResult"
    :result="autoTagResult"
    @close="autoTagResult = null"
    @apply="onApplyTags"
  />
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { mediaApi, type Media, type PagesResponse } from '@/api/mediaApi'
import { autotagApi, type AutoTagResult, type SuggestedTag } from '@/api/autotagApi'
import { dedupApi, type DuplicateItem } from '@/api/dedupApi'
import { useMediaStore } from '@/stores/mediaStore'
import TagBadge from '@/components/Tags/TagBadge.vue'
import VideoPlayer from '@/components/Player/VideoPlayer.vue'
import MangaReader from '@/components/Reader/MangaReader.vue'
import AutoTagDialog from '@/components/AutoTag/AutoTagDialog.vue'

const route = useRoute()
const router = useRouter()
const mediaStore = useMediaStore()

const media = ref<Media | null>(null)
const loading = ref(true)
const imgError = ref(false)
const pages = ref<PagesResponse | null>(null)
const showReader = ref(false)

// Auto-tag state
const autoTagging = ref(false)
const autoTagResult = ref<AutoTagResult | null>(null)

// Duplicate detection state
const duplicates = ref<DuplicateItem[]>([])

// Debounce timer for progress saves
let progressTimer: ReturnType<typeof setTimeout> | null = null

const isArchive = computed(() =>
  media.value?.type === 'manga' ||
  media.value?.type === 'comic' ||
  media.value?.type === 'doujinshi',
)

// Navigation relative to current store items list
const currentIndex = computed(() => {
  if (!media.value) return -1
  return mediaStore.items.findIndex((m) => m.id === media.value!.id)
})

const hasPrev = computed(() => currentIndex.value > 0)
const hasNext = computed(() => currentIndex.value >= 0 && currentIndex.value < mediaStore.items.length - 1)

function goToPrev() {
  if (!hasPrev.value) return
  const prev = mediaStore.items[currentIndex.value - 1]
  router.push({ name: 'media-detail', params: { id: prev.id } })
}

function goToNext() {
  if (!hasNext.value) return
  const next = mediaStore.items[currentIndex.value + 1]
  router.push({ name: 'media-detail', params: { id: next.id } })
}

onMounted(async () => {
  try {
    const res = await mediaApi.get(route.params.id as string)
    media.value = res.data

    // Load pages for archive types
    if (
      res.data.type === 'manga' ||
      res.data.type === 'comic' ||
      res.data.type === 'doujinshi'
    ) {
      try {
        const pRes = await mediaApi.getPages(res.data.id)
        pages.value = pRes.data
      } catch {
        // Non-fatal: archive may not be accessible
      }
    }

    // Load duplicates (non-blocking)
    loadDuplicates(res.data.id)
  } finally {
    loading.value = false
  }
})

async function loadDuplicates(id: string) {
  try {
    const res = await dedupApi.getDuplicatesForMedia(id)
    duplicates.value = res.data ?? []
  } catch {
    // Non-fatal: pHash may not be computed yet
  }
}

async function runAutoTag() {
  if (!media.value || autoTagging.value) return
  autoTagging.value = true
  try {
    const res = await autotagApi.autotagSingle(media.value.id)
    autoTagResult.value = res.data
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : 'Auto-tag failed'
    alert(msg)
  } finally {
    autoTagging.value = false
  }
}

async function onApplyTags(tags: SuggestedTag[]) {
  if (!media.value || !autoTagResult.value) return
  try {
    await autotagApi.autotagSingle(media.value.id, false, tags)
    // Reload media to get updated tags
    const res = await mediaApi.get(media.value.id)
    media.value = res.data
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : 'Failed to apply tags'
    alert(msg)
  } finally {
    autoTagResult.value = null
  }
}

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

function onImgError() {
  imgError.value = true
}

function openReader() {
  if (!pages.value) return
  showReader.value = true
}

function onPageChange(page: number) {
  if (!media.value || !pages.value) return
  // Debounce progress save
  if (progressTimer) clearTimeout(progressTimer)
  progressTimer = setTimeout(async () => {
    await mediaStore.saveProgress(media.value!.id, page, pages.value!.total_pages)
    if (media.value) {
      media.value = { ...media.value, read_progress: page, read_total: pages.value!.total_pages }
    }
  }, 1000)
}

// Video progress: save on timeupdate (debounced) and ended
let videoProgressTimer: ReturnType<typeof setTimeout> | null = null

function onVideoTimeUpdate(time: number) {
  if (!media.value) return
  if (videoProgressTimer) clearTimeout(videoProgressTimer)
  videoProgressTimer = setTimeout(async () => {
    // read_total is not used for videos (0 = not applicable)
    await mediaStore.saveProgress(media.value!.id, Math.floor(time), 0)
  }, 5000)
}

function onVideoEnded() {
  if (!media.value) return
  mediaStore.saveProgress(media.value.id, 0, 0)
}

function typeIcon(type: string): string {
  const icons: Record<string, string> = {
    video: '🎬', image: '🖼️', manga: '📖', comic: '📕', doujinshi: '📗',
  }
  return icons[type] ?? '📄'
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
  flex-wrap: wrap;
}
.detail-title { flex: 1; font-size: 20px; font-weight: 700; min-width: 0; }

.header-nav {
  display: flex;
  gap: 8px;
}

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
  position: relative;
}

.media-image { width: 100%; height: auto; display: block; }
.media-placeholder { padding: 80px; text-align: center; color: var(--text-muted); font-size: 32px; }

.archive-preview {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.archive-thumb { opacity: 0.7; }

.read-btn {
  position: absolute;
  bottom: 20px;
  left: 50%;
  transform: translateX(-50%);
  padding: 10px 24px;
  font-size: 15px;
  font-weight: 600;
  background: #f59e0b;
  color: #111;
  border: none;
  border-radius: var(--radius-md, 6px);
  cursor: pointer;
  box-shadow: 0 2px 12px rgba(0,0,0,0.5);
  transition: background 0.15s;
}

.read-btn:hover { background: #fbbf24; }

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
.meta-checksum { font-family: monospace; font-size: 11px; word-break: break-all; }

.source-link { color: var(--accent); font-size: 13px; }
.fav-active { color: var(--danger); border-color: var(--danger); }
.btn-sm { padding: 6px 12px; font-size: 13px; }

.btn:disabled { opacity: 0.4; cursor: not-allowed; }

.autotag-btn { align-self: flex-start; margin-top: 4px; }

.dup-warning {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: #f59e0b;
  font-weight: 500;
  text-decoration: none;
  padding: 4px 8px;
  border: 1px solid #f59e0b;
  border-radius: var(--radius);
  transition: background 0.15s;
}

.dup-warning:hover { background: rgba(245, 158, 11, 0.1); }
</style>
