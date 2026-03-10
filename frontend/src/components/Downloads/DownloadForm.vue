<template>
  <div class="download-form card">
    <h3 class="form-title">Add Download</h3>

    <div class="form-field">
      <label>URL</label>
      <input
        v-model="url"
        type="url"
        placeholder="https://example.com/gallery/123"
        class="input"
      />
    </div>

    <div class="form-field">
      <label>Target Directory (optional)</label>
      <input
        v-model="targetDir"
        type="text"
        placeholder="/downloads/gallery"
        class="input"
      />
    </div>

    <div class="form-row">
      <button class="btn btn-primary" :disabled="!url || loading" @click="submit">
        {{ loading ? 'Adding…' : '⬇️ Download' }}
      </button>
      <button class="btn btn-ghost" @click="openBatch">Batch</button>
    </div>

    <!-- Batch input -->
    <div v-if="batchMode" class="form-field">
      <label>URLs (one per line)</label>
      <textarea v-model="batchUrls" rows="5" class="input" placeholder="https://…"></textarea>
      <button class="btn btn-primary" @click="submitBatch">Add all</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useDownloadStore } from '@/stores/downloadStore'

const store = useDownloadStore()
const url = ref('')
const targetDir = ref('')
const loading = ref(false)
const batchMode = ref(false)
const batchUrls = ref('')

async function submit() {
  if (!url.value) return
  loading.value = true
  try {
    await store.enqueue({ url: url.value, target_directory: targetDir.value || undefined })
    url.value = ''
    targetDir.value = ''
  } finally {
    loading.value = false
  }
}

function openBatch() {
  batchMode.value = !batchMode.value
}

async function submitBatch() {
  const urls = batchUrls.value.split('\n').map((u) => u.trim()).filter(Boolean)
  if (!urls.length) return
  await store.enqueueBatch(urls, targetDir.value || undefined)
  batchUrls.value = ''
  batchMode.value = false
}
</script>

<style scoped>
.download-form { display: flex; flex-direction: column; gap: 16px; }
.form-title { font-size: 16px; font-weight: 600; }
.form-field { display: flex; flex-direction: column; gap: 6px; }
.form-field label { font-size: 12px; color: var(--text-secondary); }
.form-row { display: flex; gap: 8px; }

.input {
  background: var(--bg-hover);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  color: var(--text-primary);
  padding: 8px 12px;
  font-size: 14px;
  outline: none;
  width: 100%;
}
.input:focus { border-color: var(--accent); }

textarea.input { resize: vertical; font-family: inherit; }
</style>
