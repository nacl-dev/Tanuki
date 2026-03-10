<template>
  <header class="topbar">
    <div class="topbar-search">
      <SearchBar @search="onSearch" />
    </div>
    <div class="topbar-actions">
      <button class="btn btn-primary" @click="triggerScan">🔍 Scan Library</button>
      <div class="user-info" v-if="authStore.user">
        <span class="user-name">
          {{ authStore.user.display_name || authStore.user.username }}
          <span v-if="authStore.isAdmin" class="role-badge">admin</span>
        </span>
        <button class="btn btn-ghost" @click="onLogout">Sign Out</button>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'
import SearchBar from '@/components/Search/SearchBar.vue'
import { useMediaStore } from '@/stores/mediaStore'
import { useAuthStore } from '@/stores/authStore'
import axios from 'axios'

const store = useMediaStore()
const authStore = useAuthStore()
const router = useRouter()

function onSearch(q: string) {
  store.setFilter('q', q)
}

async function triggerScan() {
  await axios.post('/api/library/scan')
}

function onLogout() {
  authStore.logout()
  router.push('/login')
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
.topbar-actions { margin-left: auto; display: flex; align-items: center; gap: 12px; }

.user-info {
  display: flex;
  align-items: center;
  gap: 10px;
}

.user-name {
  font-size: 14px;
  color: var(--text-secondary);
  display: flex;
  align-items: center;
  gap: 6px;
}

.role-badge {
  font-size: 10px;
  padding: 2px 6px;
  background: var(--accent-dimmed);
  color: var(--accent);
  border-radius: 4px;
  font-weight: 600;
  text-transform: uppercase;
}

.btn-ghost {
  background: transparent;
  border: 1px solid var(--border);
  color: var(--text-secondary);
  padding: 6px 12px;
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
  transition: background 0.15s, color 0.15s;
}

.btn-ghost:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}
</style>
