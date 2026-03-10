<template>
  <div class="settings-page">
    <h2 class="page-title">Settings</h2>

    <div class="settings-grid">
      <!-- Library -->
      <div class="card settings-card">
        <h3>Library</h3>
        <div class="setting-row">
          <div>
            <p class="setting-name">Media Path</p>
            <p class="setting-desc">Directory where your media files are stored.</p>
          </div>
          <input class="input" value="/media" disabled />
        </div>
        <div class="setting-row">
          <div>
            <p class="setting-name">Scan Interval</p>
            <p class="setting-desc">How often (seconds) the library is scanned.</p>
          </div>
          <input class="input" value="300" type="number" />
        </div>
        <button class="btn btn-primary" @click="scanNow">🔍 Scan Now</button>
      </div>

      <!-- Downloads -->
      <div class="card settings-card">
        <h3>Download Manager</h3>
        <div class="setting-row">
          <div>
            <p class="setting-name">Concurrent Downloads</p>
            <p class="setting-desc">Maximum parallel downloads.</p>
          </div>
          <input class="input" value="3" type="number" min="1" max="10" />
        </div>
        <div class="setting-row">
          <div>
            <p class="setting-name">Rate Limit Delay (ms)</p>
            <p class="setting-desc">Delay between requests to the same source.</p>
          </div>
          <input class="input" value="1000" type="number" />
        </div>
      </div>

      <!-- About -->
      <div class="card settings-card">
        <h3>About</h3>
        <p class="about-text">
          🦝 <strong>Tanuki</strong> – Self-Hosted Media Vault<br />
          Version 1.0.0
        </p>
      </div>
    </div>

    <div id="duplicates" class="card settings-panel">
      <DuplicatesPage embedded />
    </div>

    <div id="plugins" class="card settings-panel">
      <PluginsPage embedded />
    </div>
  </div>
</template>

<script setup lang="ts">
import { nextTick, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { libraryApi } from '@/api/libraryApi'
import DuplicatesPage from '@/pages/DuplicatesPage.vue'
import PluginsPage from '@/pages/PluginsPage.vue'

const route = useRoute()

async function scanNow() {
  await libraryApi.scan()
}

onMounted(async () => {
  await scrollToSection(route.query.section)
})

watch(() => route.query.section, (section) => {
  void scrollToSection(section)
})

async function scrollToSection(section: unknown) {
  await nextTick()
  if (typeof section !== 'string' || !section) return
  const target = document.getElementById(section)
  target?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}
</script>

<style scoped>
.settings-page { display: flex; flex-direction: column; gap: 24px; }
.page-title { font-size: 22px; font-weight: 700; }

.settings-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
  gap: 20px;
}

.settings-card {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.settings-panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.settings-card h3 { font-size: 15px; font-weight: 600; }

.setting-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 20px;
}

.setting-name { font-size: 14px; font-weight: 500; }
.setting-desc { font-size: 12px; color: var(--text-muted); }

.input {
  background: var(--bg-hover);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  color: var(--text-primary);
  padding: 7px 12px;
  font-size: 13px;
  outline: none;
  width: 120px;
}
.input:focus { border-color: var(--accent); }

.about-text { font-size: 14px; color: var(--text-secondary); line-height: 1.8; }
</style>
