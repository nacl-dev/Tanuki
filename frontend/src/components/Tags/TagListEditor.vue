<template>
  <div class="tag-list-editor">
    <div v-if="tags.length" class="tag-list-editor__chips">
      <button
        v-for="tag in tags"
        :key="tag"
        type="button"
        class="tag-list-editor__chip"
        :aria-label="`Remove tag ${tag}`"
        @click="removeTag(tag)"
      >
        <span class="tag-list-editor__chip-label">{{ tag }}</span>
        <AppIcon name="close" :size="11" />
      </button>
    </div>
    <div v-else class="tag-list-editor__empty">No tags added yet.</div>

    <div class="tag-list-editor__composer">
      <TagSuggestInput
        v-model="draft"
        :placeholder="placeholder"
        :disabled="disabled"
        @select="addSelectedTag"
      />
      <button
        type="button"
        class="btn btn-secondary btn-sm"
        :disabled="disabled || !canAddDraft"
        @click="addDraft"
      >
        Add tag
      </button>
    </div>

    <p class="tag-list-editor__hint">
      Use `namespace:value` like `artist:name`, `series:title` or `language:en`.
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { Tag } from '@/api/mediaApi'
import TagSuggestInput from '@/components/Tags/TagSuggestInput.vue'
import AppIcon from '@/components/Layout/AppIcon.vue'
import { tagExpression } from '@/utils/tags'

const props = withDefaults(defineProps<{
  modelValue: string[]
  placeholder?: string
  disabled?: boolean
}>(), {
  placeholder: 'artist:name',
  disabled: false,
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: string[]): void
}>()

const draft = ref('')
const tags = ref<string[]>(normalizeTags(props.modelValue))

watch(() => props.modelValue, (value) => {
  tags.value = normalizeTags(value)
})

const canAddDraft = computed(() => normalizeTag(draft.value) !== '')

function normalizeTag(value: string) {
  return value.trim()
}

function normalizeTags(values: string[]) {
  const seen = new Set<string>()
  const normalized: string[] = []
  for (const raw of values) {
    const value = normalizeTag(raw)
    if (!value) continue
    const key = value.toLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    normalized.push(value)
  }
  return normalized
}

function updateTags(next: string[]) {
  tags.value = normalizeTags(next)
  emit('update:modelValue', tags.value)
}

function addDraft() {
  const value = normalizeTag(draft.value)
  if (!value) return
  updateTags([...tags.value, value])
  draft.value = ''
}

function addSelectedTag(tag: Tag) {
  updateTags([...tags.value, tagExpression(tag)])
  draft.value = ''
}

function removeTag(tag: string) {
  updateTags(tags.value.filter((item) => item !== tag))
}
</script>

<style scoped>
.tag-list-editor {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.tag-list-editor__chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.tag-list-editor__chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 999px;
  border: 1px solid var(--border);
  background: var(--bg-hover);
  color: var(--text-primary);
  cursor: pointer;
  font-size: 12px;
}

.tag-list-editor__chip:hover {
  border-color: color-mix(in srgb, var(--accent) 40%, var(--border));
}

.tag-list-editor__chip:focus-visible {
  outline: 2px solid var(--focus-ring);
  outline-offset: 2px;
}

.tag-list-editor__chip-label {
  word-break: break-word;
}

.tag-list-editor__composer {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 8px;
  align-items: start;
}

.tag-list-editor__empty,
.tag-list-editor__hint {
  font-size: 12px;
  color: var(--text-muted);
}

@media (max-width: 640px) {
  .tag-list-editor__composer {
    grid-template-columns: 1fr;
  }

  .tag-list-editor__composer .btn {
    width: 100%;
  }
}
</style>
