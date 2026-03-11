<template>
  <aside :class="['sidebar', { 'sidebar--open': open }]">
    <div class="sidebar-logo">
      <div class="logo-mark">
        <AppIcon class="logo-icon" name="library" :size="22" />
      </div>
      <div class="logo-copy">
        <span class="logo-text">Tanuki</span>
        <span class="logo-subtitle">Media Vault</span>
      </div>
      <button type="button" class="sidebar-close" aria-label="Close navigation" @click="$emit('close')">
        <AppIcon name="close" :size="16" />
      </button>
    </div>

    <nav class="sidebar-nav">
      <span class="nav-section">Browse</span>
      <RouterLink
        v-for="item in navItems"
        :key="item.name"
        :to="item.to"
        class="nav-item"
        active-class="nav-item--active"
        @click="$emit('close')"
      >
        <span class="nav-icon-wrap">
          <AppIcon :name="item.icon" :size="16" />
        </span>
        <span class="nav-label">{{ item.label }}</span>
      </RouterLink>
    </nav>

    <div class="sidebar-footer">
      <button type="button" class="nav-item nav-item--logout" aria-label="Sign out" @click="onLogout">
        <span class="nav-icon-wrap">
          <AppIcon name="logout" :size="16" />
        </span>
        <span class="nav-label">Sign Out</span>
      </button>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'
import AppIcon from '@/components/Layout/AppIcon.vue'
import { useAuthStore } from '@/stores/authStore'

defineProps<{
  open: boolean
}>()

defineEmits<{
  close: []
}>()

const authStore = useAuthStore()
const router = useRouter()

const navItems = [
  { name: 'library',    to: '/',            icon: 'library',    label: 'Library'     },
  { name: 'downloads',  to: '/downloads',   icon: 'download',   label: 'Downloads'   },
  { name: 'collections',to: '/collections', icon: 'collection', label: 'Collections' },
  { name: 'tags',       to: '/tags',        icon: 'tag',        label: 'Tags'        },
  { name: 'settings',   to: '/settings',    icon: 'settings',   label: 'Settings'    },
]

function onLogout() {
  authStore.logout()
  router.push('/login')
}
</script>

<style scoped>
.sidebar {
  width: var(--sidebar-width);
  padding: 14px 12px;
  background:
    radial-gradient(circle at top left, rgba(245, 158, 11, 0.08), transparent 34%),
    linear-gradient(180deg, rgba(255,255,255,0.02), rgba(255,255,255,0)),
    var(--bg-surface);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  gap: 14px;
  z-index: 40;
}

.sidebar-logo {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 10px 8px;
}

.sidebar-close {
  display: none;
  margin-left: auto;
  appearance: none;
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text-secondary);
  width: 32px;
  height: 32px;
  border-radius: 10px;
  cursor: pointer;
  padding: 0;
}

.logo-mark {
  width: 42px;
  height: 42px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 14px;
  background: linear-gradient(180deg, rgba(245, 158, 11, 0.2), rgba(245, 158, 11, 0.08));
  border: 1px solid rgba(245, 158, 11, 0.18);
  box-shadow: inset 0 1px 0 rgba(255,255,255,0.06);
}

.logo-icon {
  color: var(--accent);
}

.logo-copy {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.logo-text {
  color: var(--text-primary);
  font-size: 18px;
  font-weight: 700;
  line-height: 1.1;
}

.logo-subtitle {
  color: var(--text-muted);
  font-size: 11px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.sidebar-nav {
  display: flex;
  flex-direction: column;
  padding: 8px 0;
  flex: 1;
  gap: 6px;
}

.nav-section {
  padding: 0 10px 8px;
  font-size: 11px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 0 4px;
  padding: 10px 12px;
  color: var(--text-secondary);
  font-weight: 500;
  border-radius: 14px;
  border: 1px solid transparent;
  transition: background 0.15s, color 0.15s, border-color 0.15s, transform 0.15s;
}

.nav-item:hover {
  background: rgba(255,255,255,0.03);
  color: var(--text-primary);
  border-color: rgba(255,255,255,0.04);
  transform: translateX(2px);
}

.nav-item--active {
  background: linear-gradient(180deg, rgba(245, 158, 11, 0.16), rgba(245, 158, 11, 0.08));
  color: var(--accent);
  border-color: rgba(245, 158, 11, 0.22);
  box-shadow: inset 0 1px 0 rgba(255,255,255,0.05);
}

.nav-icon-wrap {
  width: 30px;
  height: 30px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 10px;
  background: rgba(255,255,255,0.035);
  flex-shrink: 0;
}

.nav-item--active .nav-icon-wrap {
  background: rgba(245, 158, 11, 0.16);
}

.nav-label { min-width: 0; }

.sidebar-footer {
  padding-top: 6px;
  border-top: 1px solid rgba(255,255,255,0.05);
}

.nav-item--logout {
  width: 100%;
  background: transparent;
  border: none;
  cursor: pointer;
  font-size: inherit;
  text-align: left;
}

.nav-item--logout:hover {
  color: #f0b35b;
}

@media (max-width: 900px) {
  .sidebar {
    position: fixed;
    top: 0;
    bottom: 0;
    left: 0;
    width: min(82vw, 300px);
    transform: translateX(-100%);
    transition: transform 0.2s ease;
    box-shadow: 0 20px 40px rgba(0, 0, 0, 0.35);
  }

  .sidebar--open {
    transform: translateX(0);
  }

  .sidebar-close {
    display: inline-flex;
    align-items: center;
    justify-content: center;
  }
}
</style>
