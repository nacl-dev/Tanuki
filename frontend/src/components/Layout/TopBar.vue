<template>
  <header class="topbar">
    <button type="button" class="menu-btn" aria-label="Open navigation" @click="$emit('toggle-sidebar')">
      <AppIcon name="menu" :size="18" />
    </button>
    <div class="topbar-search">
      <SearchBar />
    </div>
    <div class="topbar-actions">
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
import AppIcon from '@/components/Layout/AppIcon.vue'
import SearchBar from '@/components/Search/SearchBar.vue'
import { useAuthStore } from '@/stores/authStore'

defineEmits<{
  'toggle-sidebar': []
}>()

const authStore = useAuthStore()
const router = useRouter()

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

.menu-btn {
  display: none;
  appearance: none;
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text-primary);
  width: 40px;
  height: 40px;
  border-radius: 12px;
  cursor: pointer;
  flex-shrink: 0;
}

.topbar-search { flex: 1; min-width: 0; max-width: 560px; }
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
    flex-wrap: wrap;
    padding-top: 10px;
    padding-bottom: 10px;
  }

  .menu-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
  }

  .topbar-search {
    flex-basis: calc(100% - 52px);
    max-width: none;
  }

  .topbar-actions {
    width: 100%;
    margin-left: 0;
    flex-wrap: wrap;
    justify-content: space-between;
  }

  .user-info {
    width: 100%;
    justify-content: space-between;
  }
}
</style>
