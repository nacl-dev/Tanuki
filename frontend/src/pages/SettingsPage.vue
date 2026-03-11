<template>
  <div class="settings-page">
    <div class="page-head">
      <div>
        <h2 class="page-title">Settings</h2>
        <p class="page-copy">Live runtime info, quick actions and maintenance tools.</p>
      </div>
      <button class="btn btn-primary" :disabled="scanning" @click="scanNow">
        {{ scanning ? 'Scanning…' : 'Scan Library Now' }}
      </button>
    </div>

    <div class="settings-grid">
      <section class="card settings-card">
        <div class="card-head">
          <div>
            <h3>Library Runtime</h3>
            <p class="card-copy">Current paths and scanner behavior from the running backend.</p>
          </div>
          <span class="status-pill" :class="{ 'status-pill--loading': loadingInfo }">
            {{ loadingInfo ? 'Refreshing' : 'Live' }}
          </span>
        </div>

        <div v-if="info" class="setting-list">
          <div class="setting-row">
            <div>
              <p class="setting-name">Media Path</p>
              <p class="setting-desc">Primary library location.</p>
            </div>
            <code class="setting-value">{{ info.media_path }}</code>
          </div>
          <div class="setting-row">
            <div>
              <p class="setting-name">Thumbnails</p>
              <p class="setting-desc">Generated covers and preview cache.</p>
            </div>
            <code class="setting-value">{{ info.thumbnails_path }}</code>
          </div>
          <div class="setting-row">
            <div>
              <p class="setting-name">Inbox</p>
              <p class="setting-desc">Drop zone for imports and staging.</p>
            </div>
            <code class="setting-value">{{ info.inbox_path }}</code>
          </div>
          <div class="setting-row">
            <div>
              <p class="setting-name">Scan Interval</p>
              <p class="setting-desc">Automatic scanner cadence.</p>
            </div>
            <span class="setting-value">{{ info.scan_interval }}s</span>
          </div>
        </div>
        <div v-else-if="loadError" class="panel-error">{{ loadError }}</div>
        <div v-else class="panel-empty">Loading runtime information…</div>

        <p v-if="scanMessage" class="scan-feedback">{{ scanMessage }}</p>
      </section>

      <section class="card settings-card">
        <div class="card-head">
          <div>
            <h3>Downloads</h3>
            <p class="card-copy">Queue settings currently active on the server.</p>
          </div>
        </div>

        <div v-if="info" class="setting-list">
          <div class="setting-row">
            <div>
              <p class="setting-name">Downloads Path</p>
              <p class="setting-desc">Default target base for scheduled and manual downloads.</p>
            </div>
            <code class="setting-value">{{ info.downloads_path }}</code>
          </div>
          <div class="setting-row">
            <div>
              <p class="setting-name">Concurrent Downloads</p>
              <p class="setting-desc">Parallel downloader workers.</p>
            </div>
            <span class="setting-value">{{ info.max_concurrent_downloads }}</span>
          </div>
          <div class="setting-row">
            <div>
              <p class="setting-name">Rate Limit Delay</p>
              <p class="setting-desc">Pause between requests to the same source.</p>
            </div>
            <span class="setting-value">{{ info.rate_limit_delay }}ms</span>
          </div>
        </div>
      </section>

      <section class="card settings-card">
        <div class="card-head">
          <div>
            <h3>Scope Model</h3>
            <p class="card-copy">Current product behavior for shared vs. personal data areas.</p>
          </div>
        </div>

        <div v-if="info" class="setting-list">
          <div class="setting-row">
            <div>
              <p class="setting-name">Library / Tags</p>
              <p class="setting-desc">Media files and tags currently behave as one shared vault.</p>
            </div>
            <span class="setting-value">{{ info.library_scope }} / {{ info.tag_scope }}</span>
          </div>
          <div class="setting-row">
            <div>
              <p class="setting-name">Collections</p>
              <p class="setting-desc">Saved collection definitions are currently user-scoped.</p>
            </div>
            <span class="setting-value">{{ info.collection_scope }}</span>
          </div>
          <div class="setting-row">
            <div>
              <p class="setting-name">Downloads / Schedules</p>
              <p class="setting-desc">Queue and schedules are currently separated per user account.</p>
            </div>
            <span class="setting-value">{{ info.download_scope }} / {{ info.schedule_scope }}</span>
          </div>
          <div class="setting-row">
            <div>
              <p class="setting-name">Owner Mode</p>
              <p class="setting-desc">Internal owner field is not part of the current product model.</p>
            </div>
            <span class="setting-value">{{ info.owner_mode }}</span>
          </div>
        </div>
      </section>

      <section class="card settings-card settings-card--overview">
        <div class="card-head">
          <div>
            <h3>Overview</h3>
            <p class="card-copy">Quick health snapshot for this instance.</p>
          </div>
        </div>

        <div v-if="info" class="overview-stats">
          <div class="overview-stat">
            <span class="overview-label">Version</span>
            <strong>{{ info.version }}</strong>
          </div>
          <div class="overview-stat">
            <span class="overview-label">Media</span>
            <strong>{{ info.media_count }}</strong>
          </div>
          <div class="overview-stat">
            <span class="overview-label">Plugins</span>
            <strong>{{ info.plugin_count }}</strong>
          </div>
          <div class="overview-stat">
            <span class="overview-label">Active downloads</span>
            <strong>{{ info.downloads_active }}</strong>
          </div>
          <div class="overview-stat">
            <span class="overview-label">Failed downloads</span>
            <strong>{{ info.downloads_failed }}</strong>
          </div>
          <div class="overview-stat">
            <span class="overview-label">Auto-tag queue</span>
            <strong>{{ info.autotag_pending }}</strong>
          </div>
          <div class="overview-stat">
            <span class="overview-label">Active tasks</span>
            <strong>{{ info.background_tasks_active }}</strong>
          </div>
          <div class="overview-stat">
            <span class="overview-label">Failed tasks</span>
            <strong>{{ info.background_tasks_failed }}</strong>
          </div>
        </div>

        <div v-if="info" class="status-list">
          <div class="status-row">
            <span>Plugin runtime</span>
            <strong>{{ info.plugins_enabled ? 'Enabled' : 'Disabled' }}</strong>
          </div>
          <div class="status-row">
            <span>Registration</span>
            <strong>{{ info.registration_enabled ? 'Open' : 'Closed' }}</strong>
          </div>
          <div class="status-row">
            <span>Schedules</span>
            <strong>{{ info.schedules_enabled }} / {{ info.schedules_total }} enabled</strong>
          </div>
          <div class="status-row">
            <span>Last completed download</span>
            <strong>{{ lastCompletedDownloadLabel }}</strong>
          </div>
        </div>
      </section>
    </div>

    <section v-if="info" class="card settings-card">
      <div class="card-head">
        <div>
          <h3>Path Health</h3>
          <p class="card-copy">Basic runtime checks for the managed directories used by Tanuki.</p>
        </div>
      </div>

      <div class="path-health-grid">
        <div v-for="(pathInfo, key) in info.path_health" :key="key" class="path-health-card">
          <div class="path-health-card__head">
            <span class="overview-label">{{ key }}</span>
            <strong :class="pathInfo.exists ? 'path-health-ok' : 'path-health-bad'">
              {{ pathInfo.exists ? 'Available' : 'Missing' }}
            </strong>
          </div>
          <code class="setting-value setting-value--block">{{ pathInfo.path }}</code>
          <p class="path-health-meta">
            {{ pathInfo.is_dir ? 'Directory' : 'File' }}
            ·
            {{ pathInfo.writable ? 'Writable' : 'Read-only or unavailable' }}
          </p>
          <p v-if="pathInfo.error" class="panel-error">{{ pathInfo.error }}</p>
        </div>
      </div>
    </section>

    <section class="maintenance-grid">
      <button
        type="button"
        :class="['card maintenance-card', { 'maintenance-card--active': activeSection === 'duplicates' }]"
        @click="openSection('duplicates')"
      >
        <div>
          <span class="maintenance-eyebrow">Maintenance</span>
          <h3>Duplicate review</h3>
          <p class="panel-copy">Open the resolver only when you need it, instead of keeping it permanently embedded here.</p>
        </div>
        <span class="maintenance-link">{{ activeSection === 'duplicates' ? 'Open below' : 'Open section' }}</span>
      </button>

      <button
        type="button"
        :class="['card maintenance-card', { 'maintenance-card--active': activeSection === 'plugins' }]"
        @click="openSection('plugins')"
      >
        <div>
          <span class="maintenance-eyebrow">Maintenance</span>
          <h3>Plugin management</h3>
          <p class="panel-copy">
            {{ authStore.isAdmin ? 'Admin-only plugin controls and discovery tools.' : 'Visible as a status card for non-admin accounts.' }}
          </p>
        </div>
        <span class="maintenance-link">{{ activeSection === 'plugins' ? 'Open below' : 'Open section' }}</span>
      </button>
    </section>

    <div v-if="activeSection === 'duplicates'" id="duplicates" class="card settings-panel">
      <DuplicatesPage embedded />
    </div>

    <div v-if="activeSection === 'plugins'" id="plugins" class="card settings-panel">
      <template v-if="authStore.isAdmin">
        <PluginsPage embedded />
      </template>
      <template v-else>
        <div class="locked-panel">
          <h3>Plugins</h3>
          <p class="panel-copy">Plugin management is available for admin accounts only.</p>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { libraryApi } from '@/api/libraryApi'
import { systemApi, type SystemInfo } from '@/api/systemApi'
import { useAuthStore } from '@/stores/authStore'
import DuplicatesPage from '@/pages/DuplicatesPage.vue'
import PluginsPage from '@/pages/PluginsPage.vue'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

const info = ref<SystemInfo | null>(null)
const loadingInfo = ref(true)
const loadError = ref('')
const scanning = ref(false)
const scanState = ref<'idle' | 'success' | 'error'>('idle')

const scanMessage = computed(() => {
  if (scanState.value === 'success') return 'Scan queued successfully.'
  if (scanState.value === 'error') return 'Scan could not be started.'
  return ''
})
const lastCompletedDownloadLabel = computed(() => {
  if (!info.value?.last_completed_download) return 'No completed downloads yet'
  return new Date(info.value.last_completed_download).toLocaleString()
})
const activeSection = computed(() =>
  route.query.section === 'duplicates' || route.query.section === 'plugins'
    ? route.query.section
    : '',
)

async function loadSystemInfo() {
  loadingInfo.value = true
  loadError.value = ''
  try {
    info.value = await systemApi.info()
  } catch (error) {
    loadError.value = error instanceof Error ? error.message : 'Failed to load system information'
  } finally {
    loadingInfo.value = false
  }
}

async function scanNow() {
  if (scanning.value) return
  scanning.value = true
  scanState.value = 'idle'
  try {
    await libraryApi.scan()
    scanState.value = 'success'
    await loadSystemInfo()
  } catch {
    scanState.value = 'error'
  } finally {
    scanning.value = false
  }
}

onMounted(async () => {
  await Promise.all([
    loadSystemInfo(),
    scrollToSection(route.query.section),
  ])
})

watch(() => route.query.section, (section) => {
  void scrollToSection(section)
})

function openSection(section: 'duplicates' | 'plugins') {
  const nextSection = activeSection.value === section ? undefined : section
  void router.replace({
    name: 'settings',
    query: {
      ...route.query,
      section: nextSection,
    },
  })
}

async function scrollToSection(section: unknown) {
  await nextTick()
  if (typeof section !== 'string' || !section) return
  const target = document.getElementById(section)
  target?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}
</script>

<style scoped>
.settings-page {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.page-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.page-title {
  margin: 0;
  font-size: 22px;
  font-weight: 700;
}

.page-copy {
  margin: 6px 0 0;
  color: var(--text-muted);
  font-size: 13px;
}

.settings-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 20px;
}

.settings-card,
.settings-panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.maintenance-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 16px;
}

.maintenance-card {
  appearance: none;
  text-align: left;
  cursor: pointer;
  color: var(--text-primary);
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: 18px;
  background:
    linear-gradient(180deg, rgba(255,255,255,0.02), rgba(255,255,255,0)),
    var(--bg-card);
}

.maintenance-card--active {
  border-color: rgba(245, 158, 11, 0.28);
  box-shadow: inset 0 0 0 1px rgba(245, 158, 11, 0.12);
}

.maintenance-eyebrow {
  display: inline-flex;
  margin-bottom: 8px;
  font-size: 11px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--accent);
}

.maintenance-card h3 {
  margin: 0;
  font-size: 18px;
  color: var(--text-primary);
}

.maintenance-link {
  font-size: 12px;
  color: var(--text-secondary);
}

.settings-card--overview {
  background:
    radial-gradient(circle at top right, rgba(245, 158, 11, 0.12), transparent 40%),
    var(--bg-card);
}

.card-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
}

.card-head h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
}

.card-copy,
.panel-copy {
  margin: 4px 0 0;
  color: var(--text-muted);
  font-size: 12px;
  line-height: 1.5;
}

.status-pill {
  display: inline-flex;
  align-items: center;
  padding: 5px 10px;
  border-radius: 999px;
  background: rgba(34, 197, 94, 0.12);
  border: 1px solid rgba(34, 197, 94, 0.22);
  color: #86efac;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

.status-pill--loading {
  background: rgba(148, 163, 184, 0.12);
  border-color: rgba(148, 163, 184, 0.18);
  color: var(--text-muted);
}

.setting-list {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.setting-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  padding: 12px 0;
  border-top: 1px solid rgba(255, 255, 255, 0.05);
}

.setting-row:first-child {
  border-top: none;
  padding-top: 0;
}

.setting-name {
  margin: 0;
  font-size: 14px;
  font-weight: 500;
}

.setting-desc {
  margin: 4px 0 0;
  color: var(--text-muted);
  font-size: 12px;
  line-height: 1.5;
}

.setting-value {
  max-width: 48%;
  padding: 7px 10px;
  border-radius: 10px;
  border: 1px solid var(--border);
  background: var(--bg-surface);
  color: var(--text-primary);
  font-size: 12px;
  text-align: right;
  word-break: break-word;
}

.overview-stats {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.overview-stat {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 14px;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.05);
}

.overview-label {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--text-muted);
}

.overview-stat strong {
  font-size: 20px;
  color: var(--text-primary);
}

.path-health-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 14px;
}

.path-health-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.05);
}

.path-health-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.path-health-ok {
  color: #86efac;
}

.path-health-bad {
  color: #fca5a5;
}

.path-health-meta {
  margin: 0;
  color: var(--text-muted);
  font-size: 12px;
}

.setting-value--block {
  max-width: none;
  width: 100%;
  text-align: left;
}

.status-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.status-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 12px;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.05);
  font-size: 13px;
}

.scan-feedback,
.panel-empty,
.panel-error {
  margin: 0;
  font-size: 12px;
  color: var(--text-muted);
}

.panel-error {
  color: #fca5a5;
}

.locked-panel {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.locked-panel h3 {
  margin: 0;
  font-size: 16px;
}

@media (max-width: 720px) {
  .page-head {
    align-items: stretch;
  }

  .page-head .btn {
    width: 100%;
    justify-content: center;
  }

  .setting-row {
    flex-direction: column;
    align-items: flex-start;
  }

  .setting-value {
    max-width: 100%;
    width: 100%;
    text-align: left;
  }

  .overview-stats {
    grid-template-columns: 1fr;
  }
}
</style>
