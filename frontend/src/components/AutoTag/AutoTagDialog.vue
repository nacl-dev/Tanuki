<template>
  <div class="dialog-overlay" @click.self="$emit('close')">
    <div class="dialog">
      <div class="dialog-header">
        <h3>🏷️ Auto-Tag Results</h3>
        <button class="btn btn-ghost btn-sm" @click="$emit('close')">✕</button>
      </div>

      <!-- Source info -->
      <div v-if="result.source !== 'none'" class="source-info">
        <span class="badge" :class="`badge--${result.source}`">
          {{ result.source === 'saucenao' ? 'SauceNAO' : 'IQDB' }}
        </span>
        <span class="similarity">{{ result.similarity.toFixed(1) }}% similarity</span>
        <a v-if="result.source_url" :href="result.source_url" target="_blank" rel="noopener" class="source-link">
          🔗 Source
        </a>
      </div>

      <!-- No results -->
      <div v-if="result.source === 'none' || !result.suggested_tags?.length" class="no-results">
        <p>No matching tags found. Try adjusting the similarity threshold.</p>
      </div>

      <!-- Suggested tags -->
      <div v-else class="tags-list">
        <label
          v-for="tag in result.suggested_tags"
          :key="tag.name"
          class="tag-row"
        >
          <input
            type="checkbox"
            :value="tag"
            v-model="selected"
            class="tag-checkbox"
          />
          <div class="tag-info">
            <span class="tag-name">{{ tag.name }}</span>
            <span class="tag-category">{{ tag.category }}</span>
          </div>
          <div class="confidence-bar-wrap">
            <div
              class="confidence-bar"
              :style="{ width: tag.confidence + '%' }"
              :title="`${tag.confidence.toFixed(1)}% confidence`"
            ></div>
            <span class="confidence-label">{{ tag.confidence.toFixed(0) }}%</span>
          </div>
        </label>
      </div>

      <div class="dialog-actions">
        <button class="btn btn-ghost" @click="$emit('close')">Cancel</button>
        <button
          v-if="result.suggested_tags?.length"
          class="btn btn-secondary"
          @click="selectAll"
        >Select All</button>
        <button
          class="btn btn-primary"
          :disabled="selected.length === 0"
          @click="apply"
        >
          ✅ Apply {{ selected.length > 0 ? `(${selected.length})` : '' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import type { AutoTagResult, SuggestedTag } from '@/api/autotagApi'

const props = defineProps<{ result: AutoTagResult }>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'apply', tags: SuggestedTag[]): void
}>()

const selected = ref<SuggestedTag[]>([...(props.result.suggested_tags ?? [])])

function selectAll() {
  selected.value = [...(props.result.suggested_tags ?? [])]
}

function apply() {
  emit('apply', selected.value)
}
</script>

<style scoped>
.dialog-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.dialog {
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  padding: 24px;
  width: 480px;
  max-width: 95vw;
  max-height: 80vh;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.dialog-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.dialog-header h3 { font-size: 16px; font-weight: 700; }

.source-info {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 13px;
}

.badge {
  padding: 2px 8px;
  border-radius: 99px;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
}

.badge--saucenao { background: #7c3aed; color: #fff; }
.badge--iqdb     { background: #0ea5e9; color: #fff; }
.badge--none     { background: var(--border); color: var(--text-muted); }

.similarity { color: var(--text-muted); }
.source-link { color: var(--accent); }

.no-results { color: var(--text-muted); font-size: 14px; text-align: center; padding: 16px 0; }

.tags-list { display: flex; flex-direction: column; gap: 8px; }

.tag-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px;
  border-radius: var(--radius);
  cursor: pointer;
  border: 1px solid var(--border);
  transition: background 0.1s;
}

.tag-row:hover { background: var(--bg-hover); }

.tag-checkbox { flex-shrink: 0; cursor: pointer; }

.tag-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.tag-name { font-size: 14px; font-weight: 500; }
.tag-category { font-size: 11px; color: var(--text-muted); text-transform: capitalize; }

.confidence-bar-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.confidence-bar {
  height: 4px;
  min-width: 4px;
  max-width: 80px;
  background: var(--accent);
  border-radius: 2px;
  transition: width 0.2s;
}

.confidence-label { font-size: 11px; color: var(--text-muted); width: 30px; text-align: right; }

.dialog-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 8px;
}
</style>
