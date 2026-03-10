<template>
  <aside class="sidebar">
    <div class="sidebar-logo">
      <span class="logo-icon">🦝</span>
      <span class="logo-text">Tanuki</span>
    </div>

    <nav class="sidebar-nav">
      <RouterLink
        v-for="item in navItems"
        :key="item.name"
        :to="item.to"
        class="nav-item"
        active-class="nav-item--active"
      >
        <span class="nav-icon">{{ item.icon }}</span>
        <span>{{ item.label }}</span>
      </RouterLink>
    </nav>

    <div class="sidebar-footer">
      <button class="nav-item nav-item--logout" @click="onLogout">
        <span class="nav-icon">🚪</span>
        <span>Sign Out</span>
      </button>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/authStore'

const authStore = useAuthStore()
const router = useRouter()

const navItems = [
  { name: 'library',    to: '/',           icon: '📚', label: 'Library'    },
  { name: 'downloads',  to: '/downloads',  icon: '⬇️',  label: 'Downloads'  },
  { name: 'tags',       to: '/tags',       icon: '🏷️',  label: 'Tags'       },
  { name: 'duplicates', to: '/duplicates', icon: '🔍',  label: 'Duplicates' },
  { name: 'plugins',    to: '/plugins',    icon: '🧩',  label: 'Plugins'    },
  { name: 'settings',   to: '/settings',   icon: '⚙️',  label: 'Settings'   },
]

function onLogout() {
  authStore.logout()
  router.push('/login')
}
</script>

<style scoped>
.sidebar {
  width: var(--sidebar-width);
  background: var(--bg-surface);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
}

.sidebar-logo {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 18px 20px;
  font-size: 20px;
  font-weight: 700;
  border-bottom: 1px solid var(--border);
}

.logo-icon { font-size: 26px; }
.logo-text  { color: var(--accent); }

.sidebar-nav {
  display: flex;
  flex-direction: column;
  padding: 12px 0;
  flex: 1;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 20px;
  color: var(--text-secondary);
  font-weight: 500;
  border-radius: 0;
  transition: background 0.15s, color 0.15s;
}

.nav-item:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.nav-item--active {
  background: var(--accent-dimmed);
  color: var(--accent);
  border-right: 3px solid var(--accent);
}

.nav-icon { font-size: 18px; }

.sidebar-footer {
  border-top: 1px solid var(--border);
  padding: 8px 0;
}

.nav-item--logout {
  width: 100%;
  background: transparent;
  border: none;
  cursor: pointer;
  font-size: inherit;
  text-align: left;
}
</style>
