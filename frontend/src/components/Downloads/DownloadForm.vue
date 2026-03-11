<template>
  <div class="download-form card">
    <div class="form-header">
      <div class="form-icon">
        <AppIcon name="download" :size="18" />
      </div>
      <div>
        <h3 class="form-title">Add Download</h3>
        <p class="form-copy">Queue a single URL or paste a batch of links into the same target root.</p>
      </div>
    </div>

    <div class="form-field">
      <label for="download-url">URL</label>
      <input
        id="download-url"
        v-model="url"
        type="url"
        placeholder="https://example.com/gallery/123"
        class="input"
      />
    </div>

    <div class="form-field">
      <label for="download-target">Target Root (optional)</label>
      <input
        id="download-target"
        v-model="targetDir"
        type="text"
        placeholder="/media"
        class="input"
      />
    </div>

    <div class="form-row">
      <button type="button" class="btn btn-primary" :disabled="!url || loading" @click="submit">
        {{ loading ? 'Adding…' : 'Add Download' }}
      </button>
      <button type="button" class="btn btn-ghost" @click="openBatch">
        {{ batchMode ? 'Hide Batch' : 'Paste Batch' }}
      </button>
    </div>

    <div v-if="batchMode" class="form-field">
      <label for="download-batch">URLs (one per line)</label>
      <textarea id="download-batch" v-model="batchUrls" rows="5" class="input" placeholder="https://…"></textarea>
      <button type="button" class="btn btn-primary" @click="submitBatch">Add all</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useDownloadStore } from '@/stores/downloadStore'
import AppIcon from '@/components/Layout/AppIcon.vue'

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
.form-header { display: flex; align-items: flex-start; gap: 12px; }
.form-icon {
  width: 36px;
  height: 36px;
  border-radius: 12px;
  background: rgba(245, 158, 11, 0.12);
  color: var(--accent);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.form-title { font-size: 16px; font-weight: 600; }
.form-copy {
  margin-top: 4px;
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.5;
}
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
.input:focus {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px rgba(245, 158, 11, 0.14);
}

textarea.input { resize: vertical; font-family: inherit; }
</style>
