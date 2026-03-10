<template>
  <div class="plugins-page">
    <div class="page-header" :class="{ 'page-header--embedded': embedded }">
      <h2 class="page-title">Plugins</h2>
      <button class="btn btn-primary" :disabled="store.loading" @click="store.scanPlugins()">
        🔄 Scan for Plugins
      </button>
    </div>

    <p class="page-desc">
      Community plugins extend Tanuki with metadata scrapers for external sources.
      Place <code>.py</code> plugin files in the <code>plugins/</code> directory and click
      <strong>Scan for Plugins</strong> to discover them.
    </p>

    <div v-if="store.loading" class="loading-indicator">Loading…</div>

    <div v-else-if="store.error" class="error-banner">
      ⚠️ {{ store.error }}
    </div>

    <div v-else-if="store.plugins.length === 0" class="empty-state">
      <div class="empty-icon">🧩</div>
      <p>No plugins installed</p>
      <p class="empty-hint">
        Drop a Python plugin file into <code>/app/config/plugins/</code>
        and click <strong>Scan for Plugins</strong>.
      </p>
    </div>

    <div v-else class="plugin-grid">
      <div
        v-for="plugin in store.plugins"
        :key="plugin.id"
        class="card plugin-card"
        :class="{ 'plugin-card--disabled': !plugin.enabled }"
      >
        <div class="plugin-header">
          <div class="plugin-icon">🧩</div>
          <div class="plugin-info">
            <h3 class="plugin-name">{{ plugin.name }}</h3>
            <span class="plugin-source">{{ plugin.source_name }}</span>
          </div>
          <label class="toggle" :title="plugin.enabled ? 'Disable' : 'Enable'">
            <input
              type="checkbox"
              :checked="plugin.enabled"
              @change="store.togglePlugin(plugin.id, !plugin.enabled)"
            />
            <span class="toggle-slider"></span>
          </label>
        </div>

        <div class="plugin-meta">
          <span v-if="plugin.source_url" class="meta-item">
            🌐 <a :href="plugin.source_url" target="_blank" rel="noopener">{{ plugin.source_url }}</a>
          </span>
          <span class="meta-item">📦 v{{ plugin.version }}</span>
        </div>

        <div class="plugin-actions">
          <button class="btn btn-danger-sm" @click="confirmDelete(plugin)">
            🗑️ Remove
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { usePluginStore } from '@/stores/pluginStore'
import type { Plugin } from '@/api/pluginApi'

withDefaults(defineProps<{ embedded?: boolean }>(), {
  embedded: false,
})

const store = usePluginStore()

onMounted(() => {
  store.fetchPlugins()
})

function confirmDelete(plugin: Plugin) {
  if (confirm(`Remove plugin "${plugin.name}"? This will also delete the plugin file.`)) {
    store.removePlugin(plugin.id)
  }
}
</script>

<style scoped>
.plugins-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.page-header--embedded .page-title {
  font-size: 18px;
}

.page-title {
  font-size: 22px;
  font-weight: 700;
}

.page-desc {
  font-size: 14px;
  color: var(--text-secondary);
  line-height: 1.7;
}

.page-desc code {
  background: var(--bg-hover);
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 13px;
}

/* ─── Loading / Error / Empty ─────────────────────────────────────────── */

.loading-indicator {
  text-align: center;
  padding: 48px;
  color: var(--text-muted);
  font-size: 15px;
}

.error-banner {
  background: rgba(220, 60, 60, 0.12);
  color: #ff6b6b;
  padding: 12px 16px;
  border-radius: var(--radius);
  font-size: 14px;
}

.empty-state {
  text-align: center;
  padding: 64px 24px;
  color: var(--text-secondary);
}

.empty-icon {
  font-size: 48px;
  margin-bottom: 12px;
}

.empty-hint {
  font-size: 13px;
  color: var(--text-muted);
  margin-top: 8px;
}

.empty-hint code {
  background: var(--bg-hover);
  padding: 2px 6px;
  border-radius: 4px;
}

/* ─── Plugin Grid ─────────────────────────────────────────────────────── */

.plugin-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 16px;
}

.plugin-card {
  display: flex;
  flex-direction: column;
  gap: 14px;
  transition: opacity 0.2s;
}

.plugin-card--disabled {
  opacity: 0.55;
}

.plugin-header {
  display: flex;
  align-items: center;
  gap: 12px;
}

.plugin-icon {
  font-size: 28px;
  flex-shrink: 0;
}

.plugin-info {
  flex: 1;
  min-width: 0;
}

.plugin-name {
  font-size: 15px;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.plugin-source {
  font-size: 12px;
  color: var(--text-muted);
  font-family: monospace;
}

.plugin-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
  font-size: 12px;
  color: var(--text-secondary);
}

.plugin-meta a {
  color: var(--accent);
  text-decoration: none;
}
.plugin-meta a:hover {
  text-decoration: underline;
}

.meta-item {
  display: flex;
  align-items: center;
  gap: 4px;
}

.plugin-actions {
  display: flex;
  justify-content: flex-end;
}

/* ─── Toggle Switch ───────────────────────────────────────────────────── */

.toggle {
  position: relative;
  display: inline-block;
  width: 42px;
  height: 24px;
  flex-shrink: 0;
  cursor: pointer;
}

.toggle input {
  opacity: 0;
  width: 0;
  height: 0;
}

.toggle-slider {
  position: absolute;
  inset: 0;
  background: var(--bg-hover);
  border-radius: 24px;
  transition: background 0.25s;
}

.toggle-slider::before {
  content: '';
  position: absolute;
  width: 18px;
  height: 18px;
  left: 3px;
  bottom: 3px;
  background: var(--text-secondary);
  border-radius: 50%;
  transition: transform 0.25s, background 0.25s;
}

.toggle input:checked + .toggle-slider {
  background: var(--accent);
}

.toggle input:checked + .toggle-slider::before {
  transform: translateX(18px);
  background: #fff;
}

/* ─── Buttons ─────────────────────────────────────────────────────────── */

.btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border: none;
  border-radius: var(--radius);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.15s, opacity 0.15s;
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-primary {
  background: var(--accent);
  color: #fff;
}

.btn-primary:hover:not(:disabled) {
  filter: brightness(1.1);
}

.btn-danger-sm {
  background: transparent;
  color: #ff6b6b;
  padding: 6px 12px;
  font-size: 12px;
  border: 1px solid rgba(220, 60, 60, 0.3);
  border-radius: var(--radius);
}

.btn-danger-sm:hover {
  background: rgba(220, 60, 60, 0.12);
}
</style>
