<template>
  <div v-if="loading" class="loading">Loading…</div>

  <!-- Manga Reader (fullscreen overlay) -->
  <MangaReader
    v-else-if="media && showReader && pages"
    :media-id="media.id"
    :total-pages="pages.total_pages"
    :pages="pages.pages"
    :initial-page="readerStartPage"
    @close="showReader = false"
    @pagechange="onPageChange"
  />

  <div v-else-if="media" class="media-detail">
    <!-- Header -->
    <div class="detail-header">
      <button type="button" class="btn btn-ghost btn-sm" @click="router.back()">Back</button>
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
        type="button"
        :class="['btn btn-ghost', { 'fav-active': media.favorite }]"
        :aria-pressed="media.favorite"
        :aria-label="media.favorite ? 'Remove from favorites' : 'Add to favorites'"
        @click="toggleFav"
      >
        <AppIcon name="heart" :size="15" :filled="media.favorite" />
        {{ media.favorite ? 'Unfavorite' : 'Favorite' }}
      </button>
      <button type="button" class="btn btn-danger btn-sm" @click="showDeleteDialog = true">Delete</button>
    </div>

    <!-- Body -->
    <div class="detail-body">
      <!-- Preview -->
      <div class="detail-preview">
        <VideoPlayer
          v-if="media.type === 'video'"
          :src="mediaAssetUrl(media.id, 'file')"
          :poster="thumbnailAssetUrl"
          :initial-time="media.read_progress || 0"
          @timeupdate="onVideoTimeUpdate"
          @ended="onVideoEnded"
        />
        <img
          v-else-if="media.type === 'image' && !imgError"
          :src="mediaAssetUrl(media.id, 'file')"
          :alt="media.title"
          class="media-image"
          @error="onImgError"
        />
        <!-- Archive preview + read button -->
        <div v-else-if="isArchive && pages" class="archive-preview">
          <img
            v-if="!imgError"
            :src="archivePreviewUrl"
            :alt="media.title"
            class="media-image archive-thumb"
            @error="onImgError"
          />
          <div v-else class="media-placeholder">
            <AppIcon :name="typeIcon(media.type)" :size="34" />
          </div>
          <div class="archive-actions">
            <button type="button" class="btn btn-primary read-btn" @click="openReader()">
              <AppIcon name="book" :size="15" />
              {{ media.read_progress > 0 ? 'Resume reading' : 'Read archive' }}
            </button>
          </div>
        </div>
        <div v-else class="media-placeholder">
          <AppIcon :name="typeIcon(media.type)" :size="34" />
        </div>
      </div>

      <!-- Meta -->
      <aside class="detail-meta">
        <!-- Rating -->
        <div class="meta-section">
          <span class="meta-label">Rating</span>
          <div class="rating-tools">
            <div class="stars" @mouseleave="hoveredMediaRating = null">
              <button
                v-for="i in 5" :key="i"
                type="button"
                :class="['star', { 'star--on': i <= (hoveredMediaRating ?? media.rating ?? 0) }]"
                :aria-label="`Set rating to ${i} star${i === 1 ? '' : 's'}`"
                :aria-pressed="media.rating === i"
                @click="setRating(i)"
                @mouseenter="hoveredMediaRating = i"
              >★</button>
            </div>
            <div class="rating-actions">
              <button
                v-if="isArchive && media.read_progress > 0"
                type="button"
                class="btn btn-secondary btn-sm read-btn-secondary"
                @click="openReader(0)"
              >
                Start Over
              </button>
              <button
                type="button"
                class="btn btn-secondary btn-sm autotag-btn"
                :disabled="autoTagging"
                @click="runAutoTag"
              >
                <AppIcon :name="autoTagging ? 'spark' : 'tag'" :size="14" />
                {{ autoTagging ? 'Tagging…' : 'Auto-tag' }}
              </button>
            </div>
          </div>
        </div>

        <!-- Tags -->
        <div class="meta-section">
          <span class="meta-label">Tags</span>
          <div class="tags-list">
            <TagBadge v-for="tag in media.tags" :key="tag.id" :tag="tag" />
          </div>
        </div>

        <div class="meta-section">
          <div class="edit-header">
            <span class="meta-label">Details</span>
            <div class="edit-actions">
              <button
                v-if="!editingMetadata"
                class="btn btn-ghost btn-sm"
                @click="startEditing"
              >
                Edit
              </button>
              <template v-else>
                <button class="btn btn-ghost btn-sm" @click="cancelEditing" :disabled="savingMetadata">
                  Cancel
                </button>
                <button class="btn btn-secondary btn-sm" @click="saveMetadata" :disabled="savingMetadata">
                  {{ savingMetadata ? 'Saving…' : 'Save' }}
                </button>
              </template>
            </div>
          </div>
          <div v-if="!editingMetadata" class="meta-summary">
            <div class="meta-summary-row">
              <span class="meta-summary-label">Title</span>
              <span class="meta-summary-value">{{ media.title || '—' }}</span>
            </div>
            <div class="meta-summary-row">
              <span class="meta-summary-label">Date</span>
              <span class="meta-summary-value">{{ new Date(media.created_at).toLocaleDateString() }}</span>
            </div>
            <div class="meta-summary-row">
              <span class="meta-summary-label">Language</span>
              <span class="meta-summary-value">{{ media.language || '—' }}</span>
            </div>
            <div class="meta-summary-row">
              <span class="meta-summary-label">Source</span>
              <span class="meta-summary-value meta-summary-link">
                <a v-if="media.source_url" :href="media.source_url" target="_blank" rel="noopener">Source</a>
                <template v-else>—</template>
              </span>
            </div>
            <div class="meta-summary-row meta-summary-row--stacked">
              <span class="meta-summary-label">Collections</span>
              <div class="meta-summary-tags">
                <span
                  v-for="collection in activeCollections"
                  :key="collection.id"
                  class="meta-collection-chip"
                >
                  {{ collection.name }}
                </span>
                <span v-if="loadingCollections" class="meta-summary-empty">Loading collections…</span>
                <span v-else-if="!activeCollections.length" class="meta-summary-empty">No collections</span>
              </div>
            </div>
            <div class="meta-summary-row">
              <span class="meta-summary-label">Type</span>
              <span class="meta-summary-value">{{ media.type }}</span>
            </div>
            <div class="meta-summary-row">
              <span class="meta-summary-label">Views</span>
              <span class="meta-summary-value">{{ media.view_count }}</span>
            </div>
            <div class="meta-summary-row">
              <span class="meta-summary-label">Size</span>
              <span class="meta-summary-value">{{ formatBytes(media.file_size) }}</span>
            </div>
            <div v-if="isArchive && pages" class="meta-summary-row">
              <span class="meta-summary-label">Pages</span>
              <span class="meta-summary-value">{{ pages.total_pages }}</span>
            </div>
            <div v-if="isArchive && media.read_progress > 0" class="meta-summary-row">
              <span class="meta-summary-label">Progress</span>
              <span class="meta-summary-value">Page {{ media.read_progress + 1 }} / {{ media.read_total || pages?.total_pages }}</span>
            </div>
            <div v-if="media.auto_tag_status === 'completed'" class="meta-summary-row">
              <span class="meta-summary-label">Auto-tag</span>
              <span class="meta-summary-value">{{ media.auto_tag_source }} ({{ media.auto_tag_similarity?.toFixed(1) }}%)</span>
            </div>
            <div class="meta-summary-row">
              <span class="meta-summary-label">SHA-256</span>
              <span class="meta-summary-value meta-checksum">{{ media.checksum || '—' }}</span>
            </div>
            <div class="meta-summary-row">
              <span class="meta-summary-label">Added</span>
              <span class="meta-summary-value">{{ new Date(media.created_at).toLocaleDateString() }}</span>
            </div>
          </div>
          <div v-else class="edit-form">
            <div class="thumbnail-manager">
              <span class="edit-field-label">Cover</span>
              <img
                v-if="media.thumbnail_path"
                :src="thumbnailAssetUrl"
                :alt="`${media.title} cover`"
                class="thumbnail-preview"
              />
              <div v-else class="thumbnail-empty">No thumbnail yet</div>

              <div class="thumbnail-actions">
                <label class="btn btn-ghost btn-sm thumbnail-upload">
                  <input
                    class="thumbnail-upload-input"
                    type="file"
                    accept="image/png,image/jpeg,image/webp,image/gif"
                    @change="onThumbnailFileSelected"
                  />
                  {{ savingThumbnail ? 'Uploading…' : 'Upload Image' }}
                </label>
                <input
                  v-model="thumbnailUrl"
                  class="edit-input"
                  type="url"
                  placeholder="https://example.com/cover.jpg"
                />
                <button
                  class="btn btn-secondary btn-sm"
                  :disabled="savingThumbnail || !thumbnailUrl.trim()"
                  @click="applyThumbnailUrl"
                >
                  {{ savingThumbnail ? 'Saving…' : 'Use URL' }}
                </button>
                <img
                  v-if="thumbnailUrlPreview"
                  :src="thumbnailUrlPreview"
                  alt="Thumbnail URL preview"
                  class="thumbnail-preview thumbnail-preview--remote"
                />
              </div>
            </div>
            <label class="edit-field">
              <span>Title</span>
              <input v-model="editForm.title" class="edit-input" type="text" />
            </label>
            <label class="edit-field">
              <span>Date</span>
              <input v-model="editForm.created_at" class="edit-input" type="date" />
            </label>
            <label class="edit-field">
              <span>Language</span>
              <input v-model="editForm.language" class="edit-input" type="text" placeholder="ja, en, de …" />
            </label>
            <label class="edit-field">
              <span>Source URL</span>
              <input v-model="editForm.source_url" class="edit-input" type="url" placeholder="https://…" />
            </label>
            <label class="edit-field">
              <span>Tags</span>
              <textarea
                v-model="editForm.tags"
                class="edit-input edit-textarea"
                rows="3"
                placeholder="comma, separated, tags"
              />
            </label>
            <div class="edit-field">
              <span>Collections</span>
              <div v-if="loadingCollections" class="meta-summary-empty">Loading collections…</div>
              <div v-else-if="!collections.length" class="meta-summary-empty">No collections yet.</div>
              <div v-else class="collection-memberships">
                <label
                  v-for="collection in collections"
                  :key="collection.id"
                  class="collection-checkbox"
                >
                  <input
                    type="checkbox"
                    :checked="activeCollectionIds.has(collection.id)"
                    :disabled="savingCollectionId === collection.id"
                    @change="toggleCollection(collection.id, ($event.target as HTMLInputElement).checked)"
                  />
                  <span>{{ collection.name }}</span>
                </label>
              </div>
            </div>
          </div>
        </div>

        <!-- Duplicate warning -->
        <div v-if="duplicates.length > 0" class="meta-section">
          <span class="meta-label">Duplicates</span>
          <RouterLink :to="{ name: 'settings', query: { section: 'duplicates' } }" class="dup-warning">
            {{ duplicates.length }} duplicate{{ duplicates.length !== 1 ? 's' : '' }} found
          </RouterLink>
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

  <ModalShell
    v-if="showDeleteDialog"
    title="Delete media"
    description="Choose whether to remove only the library entry or delete the local file as well."
    size="sm"
    @close="showDeleteDialog = false"
  >
    <template #actions>
      <button type="button" class="btn btn-ghost" @click="showDeleteDialog = false">Cancel</button>
      <button type="button" class="btn btn-secondary" :disabled="deletingMedia" @click="deleteMedia(false)">
        {{ deletingMedia ? 'Deleting…' : 'Database Only' }}
      </button>
      <button type="button" class="btn btn-danger" :disabled="deletingMedia" @click="deleteMedia(true)">
        {{ deletingMedia ? 'Deleting…' : 'Delete Local Too' }}
      </button>
    </template>
  </ModalShell>
</template>

<script setup lang="ts">
import { ref, computed, watch, onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { mediaApi, mediaAssetUrl, mediaPageUrl, type Media, type PagesResponse } from '@/api/mediaApi'
import { autotagApi, type AutoTagResult, type SuggestedTag } from '@/api/autotagApi'
import { collectionApi, type Collection } from '@/api/collectionApi'
import { dedupApi, type DuplicateItem } from '@/api/dedupApi'
import { useMediaStore } from '@/stores/mediaStore'
import { useNoticeStore } from '@/stores/noticeStore'
import AppIcon from '@/components/Layout/AppIcon.vue'
import ModalShell from '@/components/Layout/ModalShell.vue'
import TagBadge from '@/components/Tags/TagBadge.vue'
import VideoPlayer from '@/components/Player/VideoPlayer.vue'
import MangaReader from '@/components/Reader/MangaReader.vue'
import AutoTagDialog from '@/components/AutoTag/AutoTagDialog.vue'

const route = useRoute()
const router = useRouter()
const mediaStore = useMediaStore()
const { pushNotice } = useNoticeStore()

const media = ref<Media | null>(null)
const loading = ref(true)
const imgError = ref(false)
const pages = ref<PagesResponse | null>(null)
const showReader = ref(false)
const readerStartPage = ref(0)

// Auto-tag state
const autoTagging = ref(false)
const autoTagResult = ref<AutoTagResult | null>(null)

// Duplicate detection state
const duplicates = ref<DuplicateItem[]>([])
const savingMetadata = ref(false)
const editingMetadata = ref(false)
const deletingMedia = ref(false)
const savingThumbnail = ref(false)
const collections = ref<Collection[]>([])
const loadingCollections = ref(false)
const savingCollectionId = ref('')
const showDeleteDialog = ref(false)
const thumbnailUrl = ref('')
const hoveredMediaRating = ref<number | null>(null)
const editForm = ref({
  title: '',
  created_at: '',
  language: '',
  source_url: '',
  tags: '',
})
let activeLoadToken = 0

// Debounce timer for progress saves
let progressTimer: ReturnType<typeof setTimeout> | null = null
let videoProgressTimer: ReturnType<typeof setTimeout> | null = null

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
const thumbnailAssetUrl = computed(() => {
  if (!media.value?.thumbnail_path) return undefined
  return mediaAssetUrl(media.value.id, 'thumbnail', media.value.updated_at)
})
const archivePreviewUrl = computed(() => {
  if (!media.value || !isArchive.value) return undefined
  const firstPage = pages.value?.pages?.[0]
  if (firstPage) {
    return mediaPageUrl(media.value.id, firstPage.index)
  }
  return thumbnailAssetUrl.value
})
const thumbnailUrlPreview = computed(() => {
  const url = thumbnailUrl.value.trim()
  if (!url) return ''
  return url
})
const activeCollectionIds = computed(() => new Set(collections.value.filter((item) => item.item_count > 0).map((item) => item.id)))
const activeCollections = computed(() => collections.value.filter((item) => item.item_count > 0))

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

watch(() => route.params.id, (id) => {
  if (typeof id === 'string' && id) {
    void loadMediaDetail(id)
  }
}, { immediate: true })

onBeforeUnmount(() => {
  void flushPendingProgress()
})

async function loadMediaDetail(id: string) {
  const loadToken = ++activeLoadToken
  loading.value = true
  await flushPendingProgress()
  resetDetailState()
  try {
    const res = await mediaApi.get(id)
    if (loadToken !== activeLoadToken) return
    media.value = res.data
    syncEditForm(res.data)

    // Load pages for archive types
    if (isArchiveType(res.data.type)) {
      try {
        const pRes = await mediaApi.getPages(res.data.id)
        if (loadToken !== activeLoadToken) return
        pages.value = pRes.data
      } catch {
        // Non-fatal: archive may not be accessible
      }
    }

    // Load duplicates (non-blocking)
    void loadDuplicates(res.data.id, loadToken)
    void loadCollections(res.data.id, loadToken)
  } catch {
    if (loadToken === activeLoadToken) {
      media.value = null
    }
  } finally {
    if (loadToken === activeLoadToken) {
      loading.value = false
    }
  }
}

async function loadCollections(id: string, loadToken = activeLoadToken) {
  loadingCollections.value = true
  try {
    const res = await collectionApi.listForMedia(id)
    if (loadToken !== activeLoadToken) return
    collections.value = res.data ?? []
  } finally {
    if (loadToken === activeLoadToken) {
      loadingCollections.value = false
    }
  }
}

async function loadDuplicates(id: string, loadToken = activeLoadToken) {
  try {
    const res = await dedupApi.getDuplicatesForMedia(id)
    if (loadToken !== activeLoadToken) return
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
    pushNotice({ type: 'error', message: msg })
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
    syncEditForm(res.data)
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : 'Failed to apply tags'
    pushNotice({ type: 'error', message: msg })
  } finally {
    autoTagResult.value = null
  }
}

async function toggleFav() {
  if (!media.value) return
  const res = await mediaApi.update(media.value.id, { favorite: !media.value.favorite })
  media.value = res.data
  syncEditForm(res.data)
}

async function setRating(r: number) {
  if (!media.value) return
  const res = await mediaApi.update(media.value.id, { rating: r })
  media.value = res.data
  syncEditForm(res.data)
}

async function saveMetadata() {
  if (!media.value || savingMetadata.value) return
  savingMetadata.value = true
  try {
    const res = await mediaApi.update(media.value.id, {
      title: editForm.value.title.trim(),
      created_at: editForm.value.created_at || undefined,
      language: editForm.value.language.trim() || undefined,
      source_url: editForm.value.source_url.trim() || undefined,
      tag_names: editForm.value.tags.split(',').map((tag) => tag.trim()).filter(Boolean),
    })
    media.value = res.data
    syncEditForm(res.data)
    editingMetadata.value = false
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : 'Failed to save metadata'
    pushNotice({ type: 'error', message: msg })
  } finally {
    savingMetadata.value = false
  }
}

async function onThumbnailFileSelected(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!media.value || !file || savingThumbnail.value) return

  savingThumbnail.value = true
  try {
    const res = await mediaApi.uploadThumbnail(media.value.id, file)
    media.value = res.data
    syncEditForm(res.data)
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : 'Failed to upload thumbnail'
    pushNotice({ type: 'error', message: msg })
  } finally {
    savingThumbnail.value = false
    input.value = ''
  }
}

async function applyThumbnailUrl() {
  if (!media.value || savingThumbnail.value || !thumbnailUrl.value.trim()) return

  savingThumbnail.value = true
  try {
    const res = await mediaApi.fetchThumbnail(media.value.id, { url: thumbnailUrl.value.trim() })
    media.value = res.data
    syncEditForm(res.data)
    thumbnailUrl.value = ''
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : 'Failed to fetch thumbnail'
    pushNotice({ type: 'error', message: msg })
  } finally {
    savingThumbnail.value = false
  }
}

async function toggleCollection(collectionId: string, checked: boolean) {
  if (!media.value) return
  savingCollectionId.value = collectionId
  try {
    const res = checked
      ? await collectionApi.addMedia(collectionId, media.value.id)
      : await collectionApi.removeMedia(collectionId, media.value.id)

    collections.value = collections.value.map((item) =>
      item.id === collectionId
        ? { ...item, item_count: res.data.items?.some((mediaItem) => mediaItem.id === media.value?.id) ? 1 : 0 }
        : item,
    )
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : 'Failed to update collection'
    pushNotice({ type: 'error', message: msg })
  } finally {
    savingCollectionId.value = ''
  }
}

function startEditing() {
  if (!media.value) return
  syncEditForm(media.value)
  thumbnailUrl.value = ''
  editingMetadata.value = true
}

function cancelEditing() {
  if (media.value) {
    syncEditForm(media.value)
  }
  thumbnailUrl.value = ''
  editingMetadata.value = false
}

async function deleteMedia(deleteFile: boolean) {
  if (!media.value || deletingMedia.value) return
  deletingMedia.value = true
  try {
    await mediaApi.remove(media.value.id, deleteFile)
    showDeleteDialog.value = false
    router.push({ name: 'library' })
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : 'Failed to delete media'
    pushNotice({ type: 'error', message: msg })
  } finally {
    deletingMedia.value = false
  }
}

function onImgError() {
  imgError.value = true
}

function openReader(startPage?: number) {
  if (!pages.value || !media.value) return
  const nextStartPage = startPage ?? media.value.read_progress ?? 0
  readerStartPage.value = Math.max(0, Math.min(pages.value.total_pages - 1, nextStartPage))
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

function onVideoTimeUpdate(time: number) {
  if (!media.value) return
  media.value = { ...media.value, read_progress: Math.floor(time), read_total: 0 }
  if (videoProgressTimer) clearTimeout(videoProgressTimer)
  videoProgressTimer = setTimeout(async () => {
    // read_total is not used for videos (0 = not applicable)
    await mediaStore.saveProgress(media.value!.id, Math.floor(time), 0)
  }, 5000)
}

function onVideoEnded() {
  if (!media.value) return
  if (videoProgressTimer) clearTimeout(videoProgressTimer)
  media.value = { ...media.value, read_progress: 0, read_total: 0 }
  mediaStore.saveProgress(media.value.id, 0, 0)
}

function typeIcon(type: string): 'video' | 'image' | 'book' {
  if (type === 'video') return 'video'
  if (type === 'image') return 'image'
  return 'book'
}

function formatBytes(b: number): string {
  if (b < 1024) return `${b} B`
  if (b < 1024 * 1024) return `${(b / 1024).toFixed(1)} KB`
  if (b < 1024 ** 3) return `${(b / 1024 / 1024).toFixed(1)} MB`
  return `${(b / 1024 ** 3).toFixed(2)} GB`
}

function syncEditForm(item: Media) {
  editForm.value = {
    title: item.title ?? '',
    created_at: item.created_at ? item.created_at.slice(0, 10) : '',
    language: item.language ?? '',
    source_url: item.source_url ?? '',
    tags: (item.tags ?? []).map((tag) => tag.name).join(', '),
  }
}

function resetDetailState() {
  media.value = null
  imgError.value = false
  pages.value = null
  showReader.value = false
  readerStartPage.value = 0
  duplicates.value = []
  collections.value = []
  loadingCollections.value = false
  savingCollectionId.value = ''
  showDeleteDialog.value = false
  autoTagResult.value = null
  thumbnailUrl.value = ''
  hoveredMediaRating.value = null
  editingMetadata.value = false
}

async function flushPendingProgress() {
  const currentMedia = media.value
  if (!currentMedia) {
    clearProgressTimers()
    return
  }

  const hadPendingArchiveSave = progressTimer !== null
  const hadPendingVideoSave = videoProgressTimer !== null
  clearProgressTimers()

  if (hadPendingVideoSave) {
    await mediaStore.saveProgress(currentMedia.id, Math.floor(currentMedia.read_progress || 0), 0)
    return
  }

  if (hadPendingArchiveSave && pages.value) {
    await mediaStore.saveProgress(currentMedia.id, currentMedia.read_progress || 0, pages.value.total_pages)
  }
}

function clearProgressTimers() {
  if (progressTimer) {
    clearTimeout(progressTimer)
    progressTimer = null
  }
  if (videoProgressTimer) {
    clearTimeout(videoProgressTimer)
    videoProgressTimer = null
  }
}

function isArchiveType(type: Media['type']) {
  return type === 'manga' || type === 'comic' || type === 'doujinshi'
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
  background: transparent;
  border-radius: 0;
  overflow: visible;
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: min(78vh, 960px);
  max-height: min(78vh, 960px);
}

.detail-preview :deep(.vp-container) {
  width: min(100%, calc(min(78vh, 960px) * 16 / 9));
  max-width: 100%;
  max-height: 100%;
}

.media-image { width: 100%; height: auto; display: block; }
.media-placeholder { padding: 80px; text-align: center; color: var(--text-muted); font-size: 32px; }

.archive-preview {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
  padding: 24px;
  box-sizing: border-box;
}

.archive-thumb {
  width: auto;
  max-width: 100%;
  height: 100%;
  max-height: calc(min(78vh, 960px) - 48px);
  object-fit: contain;
  display: block;
  border-radius: 10px;
  box-shadow: 0 18px 40px rgba(0, 0, 0, 0.35);
}

.archive-actions {
  position: absolute;
  bottom: 20px;
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  justify-content: center;
}

.read-btn {
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
.edit-section {
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--bg-card);
}
.thumbnail-manager {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.thumbnail-preview {
  width: 100%;
  border-radius: var(--radius);
  border: 1px solid var(--border);
  background: var(--bg-surface);
  object-fit: cover;
  aspect-ratio: 16 / 9;
}
.thumbnail-preview--remote {
  aspect-ratio: 16 / 9;
}
.thumbnail-empty {
  border: 1px dashed var(--border);
  border-radius: var(--radius);
  padding: 24px 12px;
  text-align: center;
  color: var(--text-muted);
  background: var(--bg-surface);
}
.thumbnail-actions {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.thumbnail-upload {
  display: inline-flex;
  align-self: flex-start;
  cursor: pointer;
}
.thumbnail-upload-input {
  display: none;
}
.edit-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.edit-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.edit-form {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.edit-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 12px;
  color: var(--text-muted);
}
.edit-field-label {
  font-size: 12px;
  color: var(--text-muted);
}
.edit-input {
  width: 100%;
  background: var(--bg-surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  color: var(--text-primary);
  font-size: 13px;
  padding: 8px 10px;
}
.edit-textarea {
  resize: vertical;
  min-height: 72px;
}
.meta-summary {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.meta-summary-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: flex-start;
}
.meta-summary-row--stacked {
  flex-direction: column;
}
.meta-summary-label {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--text-muted);
  min-width: 64px;
}
.meta-summary-value {
  font-size: 13px;
  color: var(--text-primary);
  text-align: right;
  word-break: break-word;
}
.meta-summary-link a {
  color: var(--accent);
  text-decoration: none;
}
.meta-summary-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.meta-collection-chip {
  display: inline-flex;
  align-items: center;
  padding: 4px 10px;
  border-radius: 999px;
  border: 1px solid var(--border);
  background: rgba(255, 255, 255, 0.04);
  font-size: 12px;
  color: var(--text-primary);
}
.meta-summary-empty {
  font-size: 12px;
  color: var(--text-muted);
}
.collection-memberships {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.collection-checkbox {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--text-primary);
}

.rating-tools { display: flex; flex-direction: column; gap: 10px; }
.rating-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  align-items: center;
}
.stars { display: flex; gap: 4px; }
.star {
  appearance: none;
  border: none;
  background: transparent;
  padding: 0;
  line-height: 1;
  font-size: 20px;
  cursor: pointer;
  color: var(--border);
  transition: color 0.1s;
}
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
.btn-danger {
  background: rgba(239, 68, 68, 0.12);
  border: 1px solid rgba(239, 68, 68, 0.35);
  color: #f87171;
}

.autotag-btn,
.read-btn-secondary {
  align-self: flex-start;
}

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


