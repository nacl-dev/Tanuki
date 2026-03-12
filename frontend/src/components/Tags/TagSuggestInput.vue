<template>
  <div ref="root" class="tag-suggest">
    <input
      v-model="inputValue"
      :class="['input', 'tag-suggest__input']"
      type="text"
      role="combobox"
      aria-autocomplete="list"
      :aria-expanded="showSuggestions && suggestions.length > 0"
      :aria-controls="listId"
      :aria-activedescendant="activeIndex >= 0 ? optionId(activeIndex) : undefined"
      :placeholder="placeholder"
      :disabled="disabled"
      autocomplete="off"
      @focus="onFocus"
      @keydown.enter.prevent="onEnter"
      @keydown.down.prevent="moveSelection(1)"
      @keydown.up.prevent="moveSelection(-1)"
      @keydown.esc="showSuggestions = false"
    />

    <div
      v-if="showSuggestions && suggestions.length"
      :id="listId"
      class="tag-suggest__menu"
      role="listbox"
    >
      <button
        v-for="(suggestion, index) in suggestions"
        :id="optionId(index)"
        :key="suggestion.id"
        type="button"
        role="option"
        :aria-selected="index === activeIndex"
        :class="['tag-suggest__option', { 'tag-suggest__option--active': index === activeIndex }]"
        @mousedown.prevent="applySuggestion(suggestion)"
      >
        <span class="tag-suggest__option-type">{{ suggestion.category }}</span>
        <span class="tag-suggest__option-label">{{ tagExpression(suggestion) }}</span>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import type { Tag } from '@/api/mediaApi'
import { useTagStore } from '@/stores/tagStore'
import { tagExpression } from '@/utils/tags'

const props = withDefaults(defineProps<{
  modelValue: string
  placeholder?: string
  disabled?: boolean
}>(), {
  placeholder: 'artist:name',
  disabled: false,
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
  (e: 'select', value: Tag): void
}>()

const store = useTagStore()
const root = ref<HTMLElement | null>(null)
const inputValue = ref(props.modelValue)
const suggestions = ref<Tag[]>([])
const activeIndex = ref(-1)
const showSuggestions = ref(false)
const listId = `tag-suggest-${Math.random().toString(36).slice(2, 8)}`
let suggestionRequestId = 0
let suppressSuggest = false

watch(() => props.modelValue, (value) => {
  if (value !== inputValue.value) {
    inputValue.value = value
  }
})

const debouncedSuggest = useDebounceFn(async (value: string) => {
  const requestId = ++suggestionRequestId
  const term = value.trim()
  if (!term) {
    suggestions.value = []
    activeIndex.value = -1
    showSuggestions.value = false
    return
  }

  const items = await store.search(term)
  if (requestId !== suggestionRequestId) {
    return
  }

  suggestions.value = items.slice(0, 8)
  activeIndex.value = suggestions.value.length ? 0 : -1
  showSuggestions.value = suggestions.value.length > 0
}, 180)

watch(inputValue, (value) => {
  emit('update:modelValue', value)
  if (suppressSuggest) {
    suppressSuggest = false
    return
  }
  void debouncedSuggest(value)
})

function onFocus() {
  if (suggestions.value.length > 0) {
    showSuggestions.value = true
    return
  }
  if (inputValue.value.trim()) {
    void debouncedSuggest(inputValue.value)
  }
}

function moveSelection(direction: number) {
  if (!suggestions.value.length) return
  showSuggestions.value = true
  activeIndex.value = (activeIndex.value + direction + suggestions.value.length) % suggestions.value.length
}

function onEnter() {
  if (showSuggestions.value && activeIndex.value >= 0 && suggestions.value[activeIndex.value]) {
    applySuggestion(suggestions.value[activeIndex.value])
    return
  }
  showSuggestions.value = false
}

function applySuggestion(tag: Tag) {
  suppressSuggest = true
  inputValue.value = tagExpression(tag)
  emit('select', tag)
  suggestions.value = []
  activeIndex.value = -1
  showSuggestions.value = false
}

function optionId(index: number) {
  return `${listId}-option-${index}`
}

function onDocumentClick(event: MouseEvent) {
  if (!root.value) return
  if (!root.value.contains(event.target as Node)) {
    showSuggestions.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', onDocumentClick)
})

onUnmounted(() => {
  document.removeEventListener('click', onDocumentClick)
})
</script>

<style scoped>
.tag-suggest {
  position: relative;
}

.tag-suggest__input {
  width: 100%;
  background: var(--bg-hover);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  color: var(--text-primary);
  padding: 8px 12px;
  font-size: 14px;
}

.tag-suggest__menu {
  position: absolute;
  top: calc(100% + 8px);
  left: 0;
  right: 0;
  z-index: 24;
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 8px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  box-shadow: 0 14px 30px rgba(0, 0, 0, 0.28);
}

.tag-suggest__option {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 8px 10px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--text-primary);
  text-align: left;
  cursor: pointer;
}

.tag-suggest__option:hover,
.tag-suggest__option--active {
  background: var(--bg-hover);
}

.tag-suggest__option:focus-visible {
  outline: 2px solid var(--focus-ring);
  outline-offset: 2px;
}

.tag-suggest__option-type {
  width: 72px;
  flex-shrink: 0;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
}

.tag-suggest__option-label {
  min-width: 0;
  font-size: 13px;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
