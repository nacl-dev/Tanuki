<template>
  <header class="topbar">
    <div class="topbar-search">
      <SearchBar @search="onSearch" />
    </div>
    <div class="topbar-actions">
      <button class="btn btn-primary" @click="triggerScan">🔍 Scan Library</button>
    </div>
  </header>
</template>

<script setup lang="ts">
import SearchBar from '@/components/Search/SearchBar.vue'
import { useMediaStore } from '@/stores/mediaStore'
import axios from 'axios'

const store = useMediaStore()

function onSearch(q: string) {
  store.setFilter('q', q)
}

async function triggerScan() {
  await axios.post('/api/library/scan')
}
</script>

<style scoped>
.topbar {
  height: var(--topbar-height);
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 20px;
  background: var(--bg-surface);
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.topbar-search { flex: 1; max-width: 480px; }
.topbar-actions { margin-left: auto; }
</style>
