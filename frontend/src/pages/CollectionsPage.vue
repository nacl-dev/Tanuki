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
        <div class="compact-form">
          <input v-model="draft.name" class="input" type="text" placeholder="Favorites, Series, Watchlist…" />
          <textarea
            v-model="draft.description"
            class="input textarea"
            rows="3"
            placeholder="Optional description"
          />
          <button class="btn btn-primary" :disabled="saving || !draft.name.trim()" @click="createCollection">
            {{ saving ? 'Saving…' : 'Create Collection' }}
          </button>
        </div>
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
            <span class="collection-link__name">{{ collection.name }}</span>
            <small>{{ collection.item_count }}</small>
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
            <span class="collection-count">{{ selected.items?.length ?? 0 }} items</span>
          </div>
          <div class="collection-detail__actions">
            <button class="btn btn-ghost btn-sm" @click="startEditing">Edit</button>
            <button class="btn btn-danger btn-sm" @click="removeCollection">Delete</button>
          </div>
        </div>

        <div v-if="editing" class="edit-box">
          <input v-model="editForm.name" class="input" type="text" />
          <textarea v-model="editForm.description" class="input textarea" rows="3" />
          <div class="edit-actions">
            <button class="btn btn-ghost btn-sm" @click="cancelEditing">Cancel</button>
            <button class="btn btn-secondary btn-sm" :disabled="saving" @click="saveCollection">Save</button>
          </div>
        </div>

        <MediaGrid :items="selected.items ?? []" :loading="false" />
      </div>
      <div v-else class="card empty">Select a collection to view its items.</div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import MediaGrid from '@/components/Gallery/MediaGrid.vue'
import { collectionApi, type Collection } from '@/api/collectionApi'

const collections = ref<Collection[]>([])
const selected = ref<Collection | null>(null)
const selectedId = ref('')
const loading = ref(false)
const saving = ref(false)
const editing = ref(false)
const draft = ref({ name: '', description: '' })
const editForm = ref({ name: '', description: '' })

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
  } finally {
    loading.value = false
  }
}

async function selectCollection(id: string) {
  selectedId.value = id
  const res = await collectionApi.get(id)
  selected.value = {
    ...res.data,
    items: res.data.items ?? [],
  }
  cancelEditing()
}

async function createCollection() {
  saving.value = true
  try {
    const res = await collectionApi.create({
      name: draft.value.name.trim(),
      description: draft.value.description.trim(),
    })
    draft.value = { name: '', description: '' }
    await loadCollections(false)
    await selectCollection(res.data.id)
  } finally {
    saving.value = false
  }
}

function startEditing() {
  if (!selected.value) return
  editForm.value = {
    name: selected.value.name,
    description: selected.value.description ?? '',
  }
  editing.value = true
}

function cancelEditing() {
  editing.value = false
}

async function saveCollection() {
  if (!selected.value) return
  saving.value = true
  try {
    const res = await collectionApi.update(selected.value.id, {
      name: editForm.value.name.trim(),
      description: editForm.value.description.trim(),
    })
    selected.value = res.data
    await loadCollections(false)
    editing.value = false
  } finally {
    saving.value = false
  }
}

async function removeCollection() {
  if (!selected.value) return
  await collectionApi.remove(selected.value.id)
  selected.value = null
  selectedId.value = ''
  await loadCollections(true)
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
  justify-content: space-between;
  align-items: center;
  gap: 10px;
  padding: 9px 10px;
  border-radius: 12px;
  border: 1px solid var(--border);
  background: var(--bg-surface);
  color: var(--text-primary);
  cursor: pointer;
  text-align: left;
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
