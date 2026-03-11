<template>
  <div class="duplicate-group">
    <div class="group-header">
      <div class="group-header__copy">
        <span class="group-label">Group #{{ group.group_id }}</span>
        <span class="group-count">{{ group.count }} similar items</span>
      </div>
      <span class="group-note">Keep one item and merge the rest away.</span>
    </div>

    <div class="group-items">
      <div class="dup-item dup-item--reference">
        <div class="dup-thumbnail">
          <img
            v-if="group.reference.thumbnail_path"
            :src="mediaAssetUrl(group.reference.id, 'thumbnail')"
            :alt="group.reference.title"
            class="thumb-img"
          />
          <div v-else class="thumb-placeholder">
            <AppIcon :name="typeIcon(group.reference.type)" :size="24" />
          </div>
        </div>
        <div class="dup-info">
          <p class="dup-title">{{ group.reference.title }}</p>
          <p class="dup-meta">{{ group.reference.type }} · {{ formatBytes(group.reference.file_size) }}</p>
          <span class="badge badge--keep">Reference</span>
        </div>
        <div class="dup-actions">
          <button
            class="btn btn-primary btn-sm"
            :disabled="keepId === group.reference.id"
            @click="setKeep(group.reference.id)"
          >
            <AppIcon :name="keepId === group.reference.id ? 'check' : 'spark'" :size="13" />
            {{ keepId === group.reference.id ? 'Selected' : 'Keep this' }}
          </button>
        </div>
      </div>

      <div
        v-for="match in group.matches"
        :key="match.id"
        class="dup-item"
        :class="{ 'dup-item--delete': keepId !== match.id && keepId !== '' }"
      >
        <div class="dup-thumbnail">
          <img
            v-if="match.thumbnail_path"
            :src="mediaAssetUrl(match.id, 'thumbnail')"
            :alt="match.title"
            class="thumb-img"
          />
          <div v-else class="thumb-placeholder">
            <AppIcon :name="typeIcon(match.type)" :size="24" />
          </div>
        </div>
        <div class="dup-info">
          <p class="dup-title">{{ match.title }}</p>
          <p class="dup-meta">{{ match.type }} · {{ formatBytes(match.file_size) }}</p>
          <span class="similarity-badge">{{ match.similarity.toFixed(1) }}% similar</span>
        </div>
        <div class="dup-actions">
          <button
            class="btn btn-primary btn-sm"
            :disabled="keepId === match.id"
            @click="setKeep(match.id)"
          >
            <AppIcon :name="keepId === match.id ? 'check' : 'spark'" :size="13" />
            {{ keepId === match.id ? 'Selected' : 'Keep this' }}
          </button>
        </div>
      </div>
    </div>

    <div class="group-resolve">
      <label class="merge-label">
        <input type="checkbox" v-model="mergeTags" />
        Merge tags from deleted items
      </label>
      <button
        class="btn btn-danger"
        :disabled="keepId === '' || resolving"
        @click="resolve"
      >
        <AppIcon name="trash" :size="14" />
        {{ resolving ? 'Resolving…' : 'Resolve duplicates' }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { mediaAssetUrl } from '@/api/mediaApi'
import type { DuplicateGroup } from '@/api/dedupApi'
import AppIcon from '@/components/Layout/AppIcon.vue'

const props = defineProps<{ group: DuplicateGroup }>()

const emit = defineEmits<{
  (e: 'resolved', keepId: string, deleteIds: string[], mergeTags: boolean): void
}>()

const keepId = ref(props.group.reference.id)
const mergeTags = ref(true)
const resolving = ref(false)

function setKeep(id: string) {
  keepId.value = id
}

async function resolve() {
  if (keepId.value === '' || resolving.value) return
  resolving.value = true
  const allIds = [props.group.reference.id, ...props.group.matches.map((m) => m.id)]
  const deleteIds = allIds.filter((id) => id !== keepId.value)
  emit('resolved', keepId.value, deleteIds, mergeTags.value)
  resolving.value = false
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
</script>

<style scoped>
.duplicate-group {
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.group-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.group-header__copy {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.group-label { font-size: 13px; font-weight: 600; color: var(--accent); }
.group-count  { font-size: 12px; color: var(--text-muted); }
.group-note { font-size: 12px; color: var(--text-muted); }

.group-items { display: flex; flex-direction: column; gap: 8px; }

.dup-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  border-radius: var(--radius);
  border: 1px solid var(--border);
  transition: opacity 0.2s, border-color 0.2s, background 0.2s;
}

.dup-item--reference { border-color: var(--accent); background: rgba(245, 158, 11, 0.06); }
.dup-item--delete    { opacity: 0.5; }

.dup-thumbnail {
  width: 72px;
  height: 72px;
  flex-shrink: 0;
  border-radius: var(--radius);
  overflow: hidden;
  background: var(--bg-surface);
  display: flex;
  align-items: center;
  justify-content: center;
}

.thumb-img { width: 100%; height: 100%; object-fit: cover; }
.thumb-placeholder {
  color: var(--text-muted);
}

.dup-info { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 4px; }
.dup-title { font-size: 13px; font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.dup-meta  { font-size: 11px; color: var(--text-muted); }

.badge {
  display: inline-block;
  padding: 1px 6px;
  border-radius: 99px;
  font-size: 10px;
  font-weight: 600;
  text-transform: uppercase;
}

.badge--keep { background: var(--accent-dimmed); color: var(--accent); }

.similarity-badge {
  display: inline-block;
  font-size: 11px;
  color: var(--accent);
  font-weight: 500;
}

.dup-actions { flex-shrink: 0; }

.group-resolve {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-top: 8px;
  border-top: 1px solid var(--border);
}

.merge-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--text-secondary);
  cursor: pointer;
}

.btn-danger {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: #ef4444;
  color: #fff;
  border: none;
  border-radius: var(--radius);
  padding: 7px 14px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.15s;
}

.btn-danger:hover:not(:disabled) { background: #dc2626; }
.btn-danger:disabled { opacity: 0.4; cursor: not-allowed; }

@media (max-width: 760px) {
  .group-header,
  .dup-item,
  .group-resolve {
    flex-direction: column;
    align-items: flex-start;
  }

  .dup-actions {
    width: 100%;
  }
}
</style>
