<template>
  <ModalShell
    title="Auto-Tag Results"
    :description="result.source === 'none' ? 'No reliable matches were found for this item.' : 'Review the suggested tags before applying them.'"
    size="md"
    @close="$emit('close')"
  >
    <div v-if="result.source !== 'none'" class="source-info">
      <span class="badge" :class="`badge--${result.source}`">
        {{ result.source === 'saucenao' ? 'SauceNAO' : 'IQDB' }}
      </span>
      <span class="similarity">{{ result.similarity.toFixed(1) }}% similarity</span>
      <a v-if="result.source_url" :href="result.source_url" target="_blank" rel="noopener" class="source-link">
        <AppIcon name="link" :size="14" />
        Source
      </a>
    </div>

    <div v-if="result.source === 'none' || !result.suggested_tags?.length" class="no-results">
      <p>No matching tags found. Try adjusting the similarity threshold.</p>
    </div>

    <div v-else class="tags-list">
      <label
        v-for="tag in result.suggested_tags"
        :key="tag.name"
        class="tag-row"
      >
        <input
          v-model="selected"
          type="checkbox"
          :value="tag"
          class="tag-checkbox"
        />
        <div class="tag-info">
          <span class="tag-name">{{ tag.name }}</span>
          <span class="tag-category">{{ tag.category }}</span>
        </div>
        <div class="confidence-bar-wrap">
          <div class="confidence-bar-track">
            <div
              class="confidence-bar"
              :style="{ width: tag.confidence + '%' }"
              :title="`${tag.confidence.toFixed(1)}% confidence`"
            />
          </div>
          <span class="confidence-label">{{ tag.confidence.toFixed(0) }}%</span>
        </div>
      </label>
    </div>

    <div class="manual-tags">
      <h4 class="manual-tags__title">Manual Tags</h4>
      <TagListEditor
        v-model="manualTags"
        placeholder="artist:name"
      />
    </div>

    <template #actions>
      <button class="btn btn-ghost" @click="$emit('close')">Cancel</button>
      <button
        v-if="result.suggested_tags?.length"
        class="btn btn-secondary"
        @click="selectAll"
      >
        Select All
      </button>
      <button
        class="btn btn-primary"
        :disabled="selected.length === 0"
        @click="apply"
      >
        <AppIcon name="check" :size="14" />
        Apply {{ selected.length > 0 ? `(${selected.length})` : '' }}
      </button>
    </template>
  </ModalShell>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import type { AutoTagResult, SuggestedTag } from '@/api/autotagApi'
import AppIcon from '@/components/Layout/AppIcon.vue'
import ModalShell from '@/components/Layout/ModalShell.vue'
import TagListEditor from '@/components/Tags/TagListEditor.vue'
import { parseTagExpression } from '@/utils/tags'

const props = defineProps<{ result: AutoTagResult }>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'apply', tags: SuggestedTag[]): void
}>()

const selected = ref<SuggestedTag[]>([...(props.result.suggested_tags ?? [])])
const manualTags = ref<string[]>([])

const allTags = computed<SuggestedTag[]>(() => {
  const combined: SuggestedTag[] = [...selected.value]
  const seen = new Set(combined.map((tag) => `${tag.category}:${tag.name}`.toLowerCase()))

  for (const raw of manualTags.value) {
    const parsed = parseTagExpression(raw)
    if (!parsed.name) continue
    const key = `${parsed.category}:${parsed.name}`.toLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    combined.push({
      name: parsed.name,
      category: parsed.category,
      confidence: 100,
    })
  }

  return combined
})

function selectAll() {
  selected.value = [...(props.result.suggested_tags ?? [])]
}

function apply() {
  emit('apply', allTags.value)
}
</script>

<style scoped>
.source-info {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  font-size: 13px;
}

.badge {
  padding: 3px 9px;
  border-radius: 99px;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
}

.badge--saucenao { background: rgba(59, 130, 246, 0.18); color: #93c5fd; }
.badge--iqdb     { background: rgba(16, 185, 129, 0.18); color: #6ee7b7; }
.badge--none     { background: var(--border); color: var(--text-muted); }

.similarity { color: var(--text-muted); }

.source-link {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--accent);
}

.no-results {
  color: var(--text-muted);
  font-size: 14px;
  text-align: center;
  padding: 8px 0;
}

.no-results p {
  margin: 0;
}

.tags-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.manual-tags {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding-top: 6px;
  border-top: 1px solid var(--border);
}

.manual-tags__title {
  margin: 0;
  font-size: 12px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.tag-row {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border-radius: 14px;
  cursor: pointer;
  border: 1px solid var(--border);
  background: color-mix(in srgb, var(--bg-surface) 92%, transparent);
  transition: background 0.15s, border-color 0.15s;
}

.tag-row:hover {
  border-color: color-mix(in srgb, var(--accent) 35%, var(--border));
  background: color-mix(in srgb, var(--accent-dimmed) 38%, var(--bg-surface));
}

.tag-checkbox {
  margin: 0;
}

.tag-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.tag-name {
  font-size: 14px;
  font-weight: 500;
}

.tag-category {
  font-size: 11px;
  color: var(--text-muted);
  text-transform: capitalize;
}

.confidence-bar-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.confidence-bar-track {
  width: 88px;
  height: 6px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.08);
  overflow: hidden;
}

.confidence-bar {
  height: 100%;
  min-width: 6px;
  background: var(--accent);
  border-radius: inherit;
  transition: width 0.2s;
}

.confidence-label {
  width: 34px;
  text-align: right;
  font-size: 11px;
  color: var(--text-muted);
}

@media (max-width: 640px) {
  .tag-row {
    grid-template-columns: auto minmax(0, 1fr);
  }

  .confidence-bar-wrap {
    grid-column: 2;
  }
}
</style>
