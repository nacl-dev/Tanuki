<template>
  <header class="topbar">
    <div class="topbar-search">
      <SearchBar />
    </div>
    <div class="topbar-actions">
      <button class="btn btn-secondary" :disabled="tagging" @click="triggerAutoTag">
        {{ tagging ? 'Queuing…' : 'Auto-Tag' }}
      </button>
      <button class="btn btn-primary" :disabled="scanning" @click="triggerScan">
        {{ scanning ? 'Scanning…' : 'Scan Library' }}
      </button>
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
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import SearchBar from '@/components/Search/SearchBar.vue'
import { useMediaStore } from '@/stores/mediaStore'
import { useAuthStore } from '@/stores/authStore'
import { autotagApi } from '@/api/autotagApi'
import { libraryApi } from '@/api/libraryApi'

const mediaStore = useMediaStore()
const authStore = useAuthStore()
const router = useRouter()
const scanning = ref(false)
const tagging = ref(false)

async function triggerScan() {
  if (scanning.value) return
  scanning.value = true
  try {
    await libraryApi.scan()
    await mediaStore.fetchList()
  } finally {
    scanning.value = false
  }
}

async function triggerAutoTag() {
  if (tagging.value) return
  tagging.value = true
  try {
    await autotagApi.autotagBatch('all_untagged')
  } finally {
    tagging.value = false
  }
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

@media (max-width: 900px) {
  .topbar {
    height: auto;
    min-height: var(--topbar-height);
    align-items: flex-start;
    padding-top: 10px;
    padding-bottom: 10px;
  }

  .topbar-actions {
    flex-wrap: wrap;
    justify-content: flex-end;
  }
}
</style>
