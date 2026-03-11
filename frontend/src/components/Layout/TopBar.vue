<template>
  <header class="topbar">
    <button type="button" class="menu-btn" aria-label="Open navigation" @click="$emit('toggle-sidebar')">
      <AppIcon name="menu" :size="18" />
    </button>
    <div class="topbar-search">
      <SearchBar />
    </div>
    <div class="topbar-actions">
      <button
        v-if="showLibraryActions"
        type="button"
        class="btn btn-primary topbar-scan-btn"
        :disabled="scanning"
        @click="scanLibrary"
      >
        {{ scanning ? 'Queueing…' : 'Scan Library' }}
      </button>
      <button
        type="button"
        :class="['btn privacy-btn', privacyStore.enabled ? 'privacy-btn--active' : 'btn-ghost']"
        :aria-pressed="privacyStore.enabled"
        :title="privacyStore.enabled ? 'Disable privacy blur' : 'Enable privacy blur'"
        @click="privacyStore.toggle()"
      >
        <AppIcon :name="privacyStore.enabled ? 'eyeOff' : 'eye'" :size="15" />
        <span class="privacy-btn__label">{{ privacyStore.enabled ? 'Blur On' : 'Blur Off' }}</span>
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
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { libraryApi } from '@/api/libraryApi'
import AppIcon from '@/components/Layout/AppIcon.vue'
import SearchBar from '@/components/Search/SearchBar.vue'
import { useAuthStore } from '@/stores/authStore'
import { useNoticeStore } from '@/stores/noticeStore'
import { usePrivacyStore } from '@/stores/privacyStore'

defineEmits<{
  'toggle-sidebar': []
}>()

const authStore = useAuthStore()
const route = useRoute()
const privacyStore = usePrivacyStore()
const router = useRouter()
const { pushNotice } = useNoticeStore()
const scanning = ref(false)
const showLibraryActions = computed(() => route.name === 'library')

async function scanLibrary() {
  if (scanning.value) return
  scanning.value = true
  try {
    const response = await libraryApi.scan()
    pushNotice({
      type: 'success',
      message: `Library scan queued (${response.data.task_id.slice(0, 8)})`,
    })
  } catch (error) {
    pushNotice({
      type: 'error',
      message: error instanceof Error ? error.message : 'Failed to queue library scan',
    })
  } finally {
    scanning.value = false
  }
}

async function onLogout() {
  await authStore.logout()
  await router.push({ name: 'login' })
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
.topbar-actions { margin-left: auto; display: flex; align-items: center; gap: 12px; min-width: 0; }

.topbar-scan-btn {
  min-width: 108px;
  justify-content: center;
  padding: 6px 12px;
  border-radius: 6px;
  font-size: 13px;
  white-space: nowrap;
}

.privacy-btn {
  min-width: 108px;
  justify-content: center;
}

.privacy-btn__label {
  white-space: nowrap;
}

.privacy-btn--active {
  background: rgba(59, 130, 246, 0.12);
  color: #bfdbfe;
  border: 1px solid rgba(59, 130, 246, 0.24);
}

.privacy-btn--active:hover {
  background: rgba(59, 130, 246, 0.18);
  color: #dbeafe;
}

.user-info {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
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
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    align-items: center;
    gap: 10px 12px;
    padding: calc(10px + env(safe-area-inset-top)) 14px 10px;
  }

  .menu-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    grid-column: 1;
    grid-row: 1;
  }

  .topbar-search {
    grid-column: 1 / -1;
    grid-row: 2;
    max-width: none;
  }

  .topbar-actions {
    grid-column: 2;
    grid-row: 1;
    width: auto;
    margin-left: 0;
    flex-wrap: wrap;
    justify-content: flex-end;
  }

  .user-info {
    justify-content: flex-end;
  }

  .privacy-btn {
    min-width: 0;
  }

  .topbar-scan-btn {
    min-width: 0;
  }
}

@media (max-width: 640px) {
  .topbar {
    gap: 8px 10px;
    padding-inline: 12px;
  }

  .topbar-actions {
    gap: 8px;
  }

  .user-name {
    display: none;
  }
}

@media (max-width: 420px) {
  .privacy-btn {
    padding-inline: 12px;
  }

  .privacy-btn__label {
    display: none;
  }
}
</style>
