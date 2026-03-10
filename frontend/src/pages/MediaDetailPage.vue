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
      <button class="btn btn-danger btn-sm" @click="showDeleteDialog = true">Delete</button>
    </div>

    <!-- Body -->
    <div class="detail-body">
      <!-- Preview -->
      <div class="detail-preview">
        <VideoPlayer
          v-if="media.type === 'video'"
          :src="mediaAssetUrl(media.id, 'file')"
          :poster="thumbnailAssetUrl"
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
            :src="thumbnailAssetUrl"
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
          <div class="rating-tools">
            <div class="stars">
              <span
                v-for="i in 5" :key="i"
                :class="['star', { 'star--on': i <= (media.rating ?? 0) }]"
                @click="setRating(i)"
              >★</span>
            </div>
            <button
              class="btn btn-secondary btn-sm autotag-btn"
              :disabled="autoTagging"
              @click="runAutoTag"
            >
              {{ autoTagging ? '⏳ Tagging…' : '🏷️ Auto-Tag' }}
            </button>
          </div>
        </div>

        <!-- Tags -->
        <div class="meta-section">
          <span class="meta-label">Tags</span>
          <div class="tags-list">
            <TagBadge v-for="tag in media.tags" :key="tag.id" :tag="tag" />
          </div>
        </div>

        <div class="meta-section edit-section">
          <div class="edit-header">
            <span class="meta-label">Collections</span>
          </div>
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

        <div class="meta-section edit-section">
          <div class="edit-header">
            <span class="meta-label">Metadata</span>
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
                <a v-if="media.source_url" :href="media.source_url" target="_blank" rel="noopener">{{ media.source_url }}</a>
                <template v-else>—</template>
              </span>
            </div>
            <div class="meta-summary-row meta-summary-row--stacked">
              <span class="meta-summary-label">Tags</span>
              <div class="meta-summary-tags">
                <TagBadge v-for="tag in media.tags" :key="`edit-${tag.id}`" :tag="tag" />
                <span v-if="!media.tags?.length" class="meta-summary-empty">No tags</span>
              </div>
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
          </div>
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

  <div v-if="showDeleteDialog" class="dialog-overlay" @click.self="showDeleteDialog = false">
    <div class="dialog-card">
      <h3>Delete Media</h3>
      <p>Wähle, ob nur der Library-Eintrag entfernt werden soll oder die Datei zusätzlich lokal gelöscht wird.</p>
      <div class="dialog-actions">
        <button class="btn btn-ghost" @click="showDeleteDialog = false">Cancel</button>
        <button class="btn btn-secondary" :disabled="deletingMedia" @click="deleteMedia(false)">
          {{ deletingMedia ? 'Deleting…' : 'Database Only' }}
        </button>
        <button class="btn btn-danger" :disabled="deletingMedia" @click="deleteMedia(true)">
          {{ deletingMedia ? 'Deleting…' : 'Delete Local Too' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { mediaApi, mediaAssetUrl, type Media, type PagesResponse } from '@/api/mediaApi'
import { autotagApi, type AutoTagResult, type SuggestedTag } from '@/api/autotagApi'
import { collectionApi, type Collection } from '@/api/collectionApi'
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
const savingMetadata = ref(false)
const editingMetadata = ref(false)
const deletingMedia = ref(false)
const savingThumbnail = ref(false)
const collections = ref<Collection[]>([])
const loadingCollections = ref(false)
const savingCollectionId = ref('')
const showDeleteDialog = ref(false)
const thumbnailUrl = ref('')
const editForm = ref({
  title: '',
  created_at: '',
  language: '',
  source_url: '',
  tags: '',
})

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
const thumbnailAssetUrl = computed(() => {
  if (!media.value?.thumbnail_path) return undefined
  return mediaAssetUrl(media.value.id, 'thumbnail', media.value.updated_at)
})
const thumbnailUrlPreview = computed(() => {
  const url = thumbnailUrl.value.trim()
  if (!url) return ''
  return url
})
const activeCollectionIds = computed(() => new Set(collections.value.filter((item) => item.item_count > 0).map((item) => item.id)))

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
    syncEditForm(res.data)

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
    loadCollections(res.data.id)
  } finally {
    loading.value = false
  }
})

async function loadCollections(id: string) {
  loadingCollections.value = true
  try {
    const res = await collectionApi.listForMedia(id)
    collections.value = res.data ?? []
  } finally {
    loadingCollections.value = false
  }
}

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
    syncEditForm(res.data)
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
    alert(msg)
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
    alert(msg)
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
    alert(msg)
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
    alert(msg)
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
    alert(msg)
  } finally {
    deletingMedia.value = false
  }
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

function syncEditForm(item: Media) {
  editForm.value = {
    title: item.title ?? '',
    created_at: item.created_at ? item.created_at.slice(0, 10) : '',
    language: item.language ?? '',
    source_url: item.source_url ?? '',
    tags: (item.tags ?? []).map((tag) => tag.name).join(', '),
  }
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
.btn-danger {
  background: rgba(239, 68, 68, 0.12);
  border: 1px solid rgba(239, 68, 68, 0.35);
  color: #f87171;
}

.autotag-btn { align-self: flex-start; }

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

.dialog-overlay {
  position: fixed;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.6);
  z-index: 1100;
}

.dialog-card {
  width: min(460px, calc(100vw - 32px));
  padding: 20px;
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background: var(--bg-card);
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.dialog-card h3 {
  margin: 0;
  font-size: 18px;
}

.dialog-card p {
  margin: 0;
  font-size: 13px;
  color: var(--text-muted);
  line-height: 1.5;
}

.dialog-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  flex-wrap: wrap;
}
</style>


