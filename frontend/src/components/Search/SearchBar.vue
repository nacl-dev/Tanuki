<template>
  <div ref="root" class="search-shell">
    <div class="search-bar">
      <span class="search-icon" aria-hidden="true">
        <AppIcon name="search" :size="16" />
      </span>
      <button
        v-for="tag in activeTags"
        :key="tag"
        class="active-tag-chip"
        type="button"
        :aria-label="`Remove tag ${tag}`"
        @click="removeTag(tag)"
      >
        #{{ tag }}
        <span class="active-tag-chip__close" aria-hidden="true">
          <AppIcon name="close" :size="10" />
        </span>
      </button>
      <input
        v-model="query"
        id="global-search"
        name="global-search"
        type="text"
        role="combobox"
        aria-autocomplete="list"
        :aria-expanded="showSuggestions && suggestions.length > 0"
        aria-controls="global-search-suggestions"
        :aria-activedescendant="activeIndex >= 0 ? suggestionId(activeIndex) : undefined"
        placeholder="Search by title or tag…"
        class="search-input"
        autocomplete="off"
        @focus="showSuggestions = suggestions.length > 0"
        @input="onInput"
        @keydown.enter.prevent="onEnter"
        @keydown.down.prevent="moveSelection(1)"
        @keydown.up.prevent="moveSelection(-1)"
        @keydown.esc="showSuggestions = false"
      />
      <button v-if="query || activeTags.length" type="button" class="clear-btn" aria-label="Clear search" @click="clearAll">
        <AppIcon name="close" :size="12" />
      </button>
    </div>

    <div
      v-if="showSuggestions && suggestions.length"
      id="global-search-suggestions"
      class="search-suggestions"
      role="listbox"
    >
      <button
        v-for="(suggestion, index) in suggestions"
        :key="`${suggestion.type}-${suggestion.value}`"
        type="button"
        :id="suggestionId(index)"
        role="option"
        :aria-selected="index === activeIndex"
        :class="['suggestion-item', { 'suggestion-item--active': index === activeIndex }]"
        @mousedown.prevent="applySuggestion(suggestion)"
      >
        <span class="suggestion-type">{{ suggestion.type === 'tag' ? 'Tag' : 'Title' }}</span>
        <span class="suggestion-label">{{ suggestion.label }}</span>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import { useRouter } from 'vue-router'
import AppIcon from '@/components/Layout/AppIcon.vue'
import { useMediaStore } from '@/stores/mediaStore'
import { tagApi } from '@/api/tagApi'
import { mediaApi, type SearchSuggestion } from '@/api/mediaApi'

const mediaStore = useMediaStore()
const router = useRouter()

const root = ref<HTMLElement | null>(null)
const query = ref(mediaStore.filters.q ?? '')
const suggestions = ref<SearchSuggestion[]>([])
const activeIndex = ref(-1)
const showSuggestions = ref(false)

const activeTags = computed(() => {
  const values = [
    ...(mediaStore.filters.tags ? mediaStore.filters.tags.split(',') : []),
    mediaStore.filters.tag ?? '',
  ]
  return values.map((item) => item.trim()).filter(Boolean).filter((item, index, arr) => arr.indexOf(item) === index)
})

watch(() => mediaStore.filters.q, (value) => {
  query.value = value ?? ''
})

watch(() => [mediaStore.filters.tag, mediaStore.filters.tags], () => {
  if (!activeTags.value.length && !query.value.trim()) {
    showSuggestions.value = false
  }
})

const debouncedSearch = useDebounceFn((value: string) => {
  mediaStore.setFilter('q', value)
}, 250)

const debouncedSuggest = useDebounceFn(async (value: string) => {
  const term = value.trim()
  if (!term) {
    suggestions.value = []
    activeIndex.value = -1
    showSuggestions.value = false
    return
  }

  const [tagsRes, titlesRes] = await Promise.allSettled([
    tagApi.search(term),
    mediaApi.suggestions(term),
  ])

  const tagSuggestions: SearchSuggestion[] = (tagsRes.status === 'fulfilled' ? (tagsRes.value.data ?? []) : []).map((tag) => ({
    type: 'tag',
    value: tag.name,
    label: tag.name,
  }))

  const titleSuggestions = titlesRes.status === 'fulfilled' ? (titlesRes.value.data ?? []) : []
  const merged = [...tagSuggestions, ...titleSuggestions]
  const seen = new Set<string>()
  suggestions.value = merged.filter((item) => {
    const key = `${item.type}:${item.value.toLowerCase()}`
    if (seen.has(key)) return false
    seen.add(key)
    return true
  }).slice(0, 8)
  activeIndex.value = suggestions.value.length ? 0 : -1
  showSuggestions.value = suggestions.value.length > 0
}, 200)

function onInput() {
  debouncedSearch(query.value)
  void debouncedSuggest(query.value)
}

function onEnter() {
  if (showSuggestions.value && activeIndex.value >= 0 && suggestions.value[activeIndex.value]) {
    applySuggestion(suggestions.value[activeIndex.value])
    return
  }
  mediaStore.setFilter('q', query.value.trim())
  showSuggestions.value = false
}

function moveSelection(direction: number) {
  if (!suggestions.value.length) return
  showSuggestions.value = true
  activeIndex.value = (activeIndex.value + direction + suggestions.value.length) % suggestions.value.length
}

function applySuggestion(suggestion: SearchSuggestion) {
  if (suggestion.type === 'tag') {
    query.value = ''
    mediaStore.filters.q = ''
    addTag(suggestion.value)
  } else {
    mediaStore.filters.tag = ''
    mediaStore.filters.tags = ''
    query.value = suggestion.value
    mediaStore.setFilter('q', suggestion.value)
    void router.replace({ path: '/', query: {} })
  }
  suggestions.value = []
  activeIndex.value = -1
  showSuggestions.value = false
}

function suggestionId(index: number) {
  return `global-search-suggestion-${index}`
}

function addTag(tag: string) {
  const next = [...activeTags.value, tag].filter((item, index, arr) => arr.indexOf(item) === index)
  mediaStore.filters.tag = ''
  mediaStore.setFilter('tags', next.join(','))
  void router.replace({ path: '/', query: { tags: next.join(',') } })
}

function removeTag(tag: string) {
  const next = activeTags.value.filter((item) => item !== tag)
  mediaStore.filters.tag = ''
  mediaStore.setFilter('tags', next.join(','))
  void router.replace({ path: '/', query: next.length ? { tags: next.join(',') } : {} })
}

function clearAll() {
  query.value = ''
  mediaStore.filters.q = ''
  mediaStore.filters.tag = ''
  mediaStore.filters.tags = ''
  mediaStore.fetchList()
  suggestions.value = []
  showSuggestions.value = false
  void router.replace({ path: '/', query: {} })
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
.search-shell {
  position: relative;
}

.search-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 6px 12px;
}

.search-bar:focus-within {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px rgba(245, 158, 11, 0.16);
}

.search-icon {
  display: inline-flex;
  color: var(--text-muted);
}

.active-tag-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border: 1px solid var(--accent);
  background: var(--accent-dimmed);
  color: var(--accent);
  padding: 4px 10px;
  border-radius: 999px;
  font-size: 12px;
  white-space: nowrap;
  cursor: pointer;
}

.active-tag-chip:focus-visible,
.clear-btn:focus-visible,
.suggestion-item:focus-visible {
  outline: 2px solid var(--focus-ring);
  outline-offset: 2px;
}

.active-tag-chip__close {
  display: inline-flex;
}

.search-input {
  flex: 1;
  background: transparent;
  border: none;
  outline: none;
  color: var(--text-primary);
  font-size: 14px;
}

.search-input::placeholder { color: var(--text-muted); }

.clear-btn {
  background: transparent;
  border: none;
  cursor: pointer;
  color: var(--text-muted);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 2px 4px;
}
.clear-btn:hover { color: var(--text-primary); }

.search-suggestions {
  position: absolute;
  top: calc(100% + 8px);
  left: 0;
  right: 0;
  z-index: 30;
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 8px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  box-shadow: 0 14px 30px rgba(0, 0, 0, 0.28);
}

.suggestion-item {
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

.suggestion-item:hover,
.suggestion-item--active {
  background: var(--bg-hover);
}

.suggestion-type {
  width: 40px;
  flex-shrink: 0;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
}

.suggestion-label {
  min-width: 0;
  font-size: 13px;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
