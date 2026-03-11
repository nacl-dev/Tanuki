<template>
  <div class="collections-page">
    <aside class="collections-sidebar">
      <div class="card collections-form">
        <div class="section-header">
          <div>
            <h3>Create Collection</h3>
            <p class="section-copy">Kurze Listen statt riesiger Seitenleisten.</p>
          </div>
        </div>
        <form class="compact-form" @submit.prevent="createCollection">
          <input v-model="draft.name" class="input" type="text" placeholder="Favorites, Series, Watchlist…" />
          <textarea
            v-model="draft.description"
            class="input textarea"
            rows="3"
            placeholder="Optional description"
          />
          <div class="smart-box">
            <div class="smart-box__header">Auto Collection Rules</div>
            <div class="smart-grid">
              <select v-model="draft.auto_type" class="input">
                <option value="">Any type</option>
                <option value="video">Video</option>
                <option value="image">Image</option>
                <option value="manga">Manga</option>
                <option value="comic">Comic</option>
                <option value="doujinshi">Doujin</option>
              </select>
              <input v-model="draft.auto_title" class="input" type="text" placeholder="Title contains, e.g. Venus Blood" />
              <input v-model="draft.auto_tag" class="input" type="text" placeholder="Tag, e.g. tentacles" />
              <select v-model="draft.auto_favorite_mode" class="input">
                <option value="">Any favorite state</option>
                <option value="true">Favorites only</option>
              </select>
              <select v-model="draft.auto_min_rating" class="input">
                <option value="">Any rating</option>
                <option value="1">1★+</option>
                <option value="2">2★+</option>
                <option value="3">3★+</option>
                <option value="4">4★+</option>
                <option value="5">5★</option>
              </select>
            </div>
          </div>
          <button class="btn btn-primary" type="submit" :disabled="!canCreateCollection">
            {{ saving ? 'Saving…' : 'Create Collection' }}
          </button>
        </form>
      </div>

      <div class="card collections-list">
        <div class="section-header">
          <div>
            <h3>Collections</h3>
            <p class="section-copy">{{ collections.length }} total</p>
          </div>
        </div>
        <div class="collections-pill-list">
          <button
            v-for="collection in collections"
            :key="collection.id"
            :class="['collection-link', { active: selectedId === collection.id }]"
            @click="selectCollection(collection.id)"
          >
            <div class="collection-link__preview" v-if="collection.items?.length">
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
            <div class="collection-link__meta">
              <span class="collection-link__name">{{ collection.name }}</span>
              <small>{{ collection.item_count }}</small>
            </div>
          </button>
        </div>
        <div v-if="!collections.length" class="empty">No collections yet.</div>
      </div>
    </aside>

    <section class="collections-main">
      <div v-if="loading" class="card empty">Loading…</div>
      <div v-else-if="selected" class="card collection-detail">
        <div class="collection-detail__header">
          <div class="collection-detail__title">
            <h2>{{ selected.name }}</h2>
            <p v-if="selected.description">{{ selected.description }}</p>
            <div v-if="autoSummary(selected).length" class="collection-rules">
              <span v-for="rule in autoSummary(selected)" :key="rule" class="collection-rule">{{ rule }}</span>
            </div>
            <span class="collection-count">{{ selected.items?.length ?? 0 }} items</span>
          </div>
          <div class="collection-detail__actions">
            <button class="btn btn-ghost btn-sm" @click="startEditing">Edit</button>
            <button class="btn btn-danger btn-sm" @click="removeCollection">Delete</button>
          </div>
        </div>

        <form v-if="editing" class="edit-box" @submit.prevent="saveCollection">
          <input v-model="editForm.name" class="input" type="text" />
          <textarea v-model="editForm.description" class="input textarea" rows="3" />
          <div class="smart-box">
            <div class="smart-box__header">Auto Collection Rules</div>
            <div class="smart-grid">
              <select v-model="editForm.auto_type" class="input">
                <option value="">Any type</option>
                <option value="video">Video</option>
                <option value="image">Image</option>
                <option value="manga">Manga</option>
                <option value="comic">Comic</option>
                <option value="doujinshi">Doujin</option>
              </select>
              <input v-model="editForm.auto_title" class="input" type="text" placeholder="Title contains, e.g. Venus Blood" />
              <input v-model="editForm.auto_tag" class="input" type="text" placeholder="Tag, e.g. tentacles" />
              <select v-model="editForm.auto_favorite_mode" class="input">
                <option value="">Any favorite state</option>
                <option value="true">Favorites only</option>
              </select>
              <select v-model="editForm.auto_min_rating" class="input">
                <option value="">Any rating</option>
                <option value="1">1★+</option>
                <option value="2">2★+</option>
                <option value="3">3★+</option>
                <option value="4">4★+</option>
                <option value="5">5★</option>
              </select>
            </div>
          </div>
          <div class="edit-actions">
            <button class="btn btn-ghost btn-sm" type="button" @click="cancelEditing">Cancel</button>
            <button class="btn btn-secondary btn-sm" type="submit" :disabled="!canSaveCollection">Save</button>
          </div>
        </form>

        <MediaGrid :items="selected.items ?? []" :loading="false" />
      </div>
      <div v-else class="card empty">Select a collection to view its items.</div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import MediaGrid from '@/components/Gallery/MediaGrid.vue'
import { collectionApi, type Collection } from '@/api/collectionApi'
import { mediaAssetUrl } from '@/api/mediaApi'
import { useNoticeStore } from '@/stores/noticeStore'

const collections = ref<Collection[]>([])
const selected = ref<Collection | null>(null)
const selectedId = ref('')
const loading = ref(false)
const saving = ref(false)
const editing = ref(false)
const draft = ref({ name: '', description: '', auto_type: '', auto_title: '', auto_tag: '', auto_favorite_mode: '', auto_min_rating: '' })
const editForm = ref({ name: '', description: '', auto_type: '', auto_title: '', auto_tag: '', auto_favorite_mode: '', auto_min_rating: '' })
const { pushNotice } = useNoticeStore()

const canCreateCollection = computed(() => !saving.value && draft.value.name.trim().length > 0)
const canSaveCollection = computed(() => !saving.value && editForm.value.name.trim().length > 0)

async function loadCollections(selectFirst = true) {
  loading.value = true
  try {
    const res = await collectionApi.list()
    const items = res.data ?? []
    collections.value = items
    if (selectFirst && items.length && !selectedId.value) {
      await selectCollection(items[0].id)
    } else if (selectedId.value) {
      const stillExists = items.find((item) => item.id === selectedId.value)
      if (stillExists) {
        await selectCollection(stillExists.id)
      } else {
        selected.value = null
        selectedId.value = ''
      }
    }
  } catch (error) {
    const msg = error instanceof Error ? error.message : 'Failed to load collections'
    pushNotice({ type: 'error', message: msg })
  } finally {
    loading.value = false
  }
}

async function selectCollection(id: string) {
  try {
    selectedId.value = id
    const res = await collectionApi.get(id)
    selected.value = {
      ...res.data,
      items: res.data.items ?? [],
    }
    cancelEditing()
  } catch (error) {
    selected.value = null
    selectedId.value = ''
    const msg = error instanceof Error ? error.message : 'Failed to load collection'
    pushNotice({ type: 'error', message: msg })
  }
}

async function createCollection() {
  if (!canCreateCollection.value) {
    pushNotice({ type: 'error', message: 'Please enter a collection name.' })
    return
  }
  saving.value = true
  try {
    const res = await collectionApi.create({
      name: draft.value.name.trim(),
      description: draft.value.description.trim(),
      auto_type: draft.value.auto_type || undefined,
      auto_title: draft.value.auto_title.trim() || undefined,
      auto_tag: draft.value.auto_tag.trim() || undefined,
      auto_favorite: parseFavoriteMode(draft.value.auto_favorite_mode),
      auto_min_rating: parseMinRating(draft.value.auto_min_rating),
    })
    draft.value = { name: '', description: '', auto_type: '', auto_title: '', auto_tag: '', auto_favorite_mode: '', auto_min_rating: '' }
    await loadCollections(false)
    await selectCollection(res.data.id)
    pushNotice({ type: 'success', message: `Collection "${res.data.name}" created.` })
  } catch (error) {
    const msg = error instanceof Error ? error.message : 'Failed to create collection'
    pushNotice({ type: 'error', message: msg })
  } finally {
    saving.value = false
  }
}

function startEditing() {
  if (!selected.value) return
  editForm.value = {
    name: selected.value.name,
    description: selected.value.description ?? '',
    auto_type: selected.value.auto_type ?? '',
    auto_title: selected.value.auto_title ?? '',
    auto_tag: selected.value.auto_tag ?? '',
    auto_favorite_mode: selected.value.auto_favorite ? 'true' : '',
    auto_min_rating: selected.value.auto_min_rating ? String(selected.value.auto_min_rating) : '',
  }
  editing.value = true
}

function cancelEditing() {
  editing.value = false
}

async function saveCollection() {
  if (!selected.value) return
  if (!canSaveCollection.value) {
    pushNotice({ type: 'error', message: 'Please enter a collection name.' })
    return
  }
  saving.value = true
  try {
    const res = await collectionApi.update(selected.value.id, {
      name: editForm.value.name.trim(),
      description: editForm.value.description.trim(),
      auto_type: editForm.value.auto_type || '',
      auto_title: editForm.value.auto_title.trim(),
      auto_tag: editForm.value.auto_tag.trim(),
      auto_favorite: parseFavoriteMode(editForm.value.auto_favorite_mode),
      auto_min_rating: parseMinRating(editForm.value.auto_min_rating),
    })
    selected.value = res.data
    await loadCollections(false)
    editing.value = false
    pushNotice({ type: 'success', message: `Collection "${res.data.name}" updated.` })
  } catch (error) {
    const msg = error instanceof Error ? error.message : 'Failed to update collection'
    pushNotice({ type: 'error', message: msg })
  } finally {
    saving.value = false
  }
}

async function removeCollection() {
  if (!selected.value) return
  const name = selected.value.name
  try {
    await collectionApi.remove(selected.value.id)
    selected.value = null
    selectedId.value = ''
    await loadCollections(true)
    pushNotice({ type: 'success', message: `Collection "${name}" deleted.` })
  } catch (error) {
    const msg = error instanceof Error ? error.message : 'Failed to delete collection'
    pushNotice({ type: 'error', message: msg })
  }
}

function previewItems(collection: Collection) {
  return (collection.items ?? []).slice(0, 5)
}

function parseFavoriteMode(value: string): boolean | null | undefined {
  if (value === 'true') return true
  if (value === '') return null
  return undefined
}

function parseMinRating(value: string): number | null | undefined {
  if (!value) return null
  const parsed = Number.parseInt(value, 10)
  return Number.isFinite(parsed) ? parsed : undefined
}

function autoSummary(collection: Collection) {
  const rules: string[] = []
  if (collection.auto_type) rules.push(collection.auto_type)
  if (collection.auto_title) rules.push(`title:${collection.auto_title}`)
  if (collection.auto_tag) rules.push(`#${collection.auto_tag}`)
  if (collection.auto_favorite) rules.push('favorites')
  if (collection.auto_min_rating) rules.push(`${collection.auto_min_rating}★+`)
  return rules
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

onMounted(() => {
  loadCollections()
})
</script>

<style scoped>
.collections-page {
  display: grid;
  grid-template-columns: minmax(260px, 320px) minmax(0, 1fr);
  gap: 24px;
  align-items: flex-start;
}

.collections-sidebar {
  display: flex;
  flex-direction: column;
  gap: 16px;
  position: sticky;
  top: 16px;
}

.collections-form,
.collections-list,
.collection-detail,
.empty {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.section-header {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: flex-start;
}

.section-copy {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--text-muted);
}

.compact-form {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.smart-box {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 10px;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--bg-surface);
}

.smart-box__header {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
}

.smart-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.collections-pill-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: min(42vh, 420px);
  overflow: auto;
  padding-right: 2px;
}

.collection-link {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 10px;
  border-radius: 12px;
  border: 1px solid var(--border);
  background: var(--bg-surface);
  color: var(--text-primary);
  cursor: pointer;
  text-align: left;
}

.collection-link__preview {
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

.collection-link__meta {
  width: 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 10px;
}

.collection-link.active {
  border-color: var(--accent);
  background: var(--accent-dimmed);
}

.collection-link__name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.collection-link small {
  color: var(--text-muted);
  flex-shrink: 0;
  padding: 2px 8px;
  border-radius: 999px;
  background: var(--bg-card);
}

.collections-main {
  flex: 1;
  min-width: 0;
}

.collection-detail__header {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: flex-start;
}

.collection-detail__title {
  min-width: 0;
}

.collection-detail__header h2,
.collections-form h3,
.collections-list h3 {
  margin: 0;
}

.collection-detail__header p {
  margin: 6px 0 0;
  color: var(--text-muted);
  font-size: 13px;
}

.collection-count {
  display: inline-flex;
  margin-top: 10px;
  font-size: 12px;
  color: var(--text-muted);
  padding: 4px 8px;
  border-radius: 999px;
  border: 1px solid var(--border);
  background: var(--bg-surface);
}

.collection-rules {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 10px;
}

.collection-rule {
  display: inline-flex;
  padding: 4px 8px;
  border-radius: 999px;
  background: var(--bg-surface);
  border: 1px solid var(--border);
  color: var(--text-muted);
  font-size: 12px;
}

.collection-detail__actions,
.edit-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.edit-box {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
  background: var(--bg-surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
}

.input {
  width: 100%;
  background: var(--bg-hover);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  color: var(--text-primary);
  padding: 8px 12px;
  font-size: 14px;
}

.textarea {
  resize: vertical;
  min-height: 90px;
  font-family: inherit;
}

.btn-danger {
  background: rgba(239, 68, 68, 0.12);
  border: 1px solid rgba(239, 68, 68, 0.35);
  color: #f87171;
}

.empty {
  color: var(--text-muted);
}

@media (max-width: 980px) {
  .collections-page {
    grid-template-columns: 1fr;
  }

  .collections-sidebar {
    position: static;
  }

  .collections-pill-list {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
    max-height: none;
    overflow: visible;
  }
}

@media (max-width: 640px) {
  .collections-page {
    gap: 16px;
  }

  .smart-grid {
    grid-template-columns: 1fr;
  }

  .collection-detail__header {
    flex-direction: column;
  }

  .collection-detail__actions {
    width: 100%;
    justify-content: stretch;
  }

  .collection-detail__actions .btn {
    flex: 1;
  }
}
</style>
