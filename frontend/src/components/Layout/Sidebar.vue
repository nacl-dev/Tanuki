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
  </aside>
</template>

<script setup lang="ts">
const navItems = [
  { name: 'library',   to: '/',          icon: '📚', label: 'Library'   },
  { name: 'downloads', to: '/downloads', icon: '⬇️',  label: 'Downloads' },
  { name: 'tags',      to: '/tags',      icon: '🏷️',  label: 'Tags'      },
  { name: 'settings',  to: '/settings',  icon: '⚙️',  label: 'Settings'  },
]
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
</style>
