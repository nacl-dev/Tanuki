<template>
  <div class="search-bar">
    <span class="search-icon">🔍</span>
    <input
      v-model="query"
      type="text"
      placeholder="Search by title or tag…"
      class="search-input"
      @input="onInput"
      @keydown.enter="emit('search', query)"
    />
    <button v-if="query" class="clear-btn" @click="clear">✕</button>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useDebounceFn } from '@vueuse/core'

const emit = defineEmits<{ (e: 'search', q: string): void }>()

const query = ref('')

const debouncedEmit = useDebounceFn((q: string) => emit('search', q), 300)

function onInput() {
  debouncedEmit(query.value)
}

function clear() {
  query.value = ''
  emit('search', '')
}
</script>

<style scoped>
.search-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 6px 12px;
}

.search-icon { color: var(--text-muted); }

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
  font-size: 12px;
}
.clear-btn:hover { color: var(--text-primary); }
</style>
