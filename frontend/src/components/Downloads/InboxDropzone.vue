<template>
  <div class="inbox-dropzone card">
    <div class="dropzone-header">
      <div class="dropzone-header__icon">
        <AppIcon name="upload" :size="18" />
      </div>
      <div>
        <h3>Inbox Upload</h3>
        <p>Drop files or folders from this device into a remote inbox batch, then organize them into the library.</p>
      </div>
    </div>

    <div
      :class="['dropzone-surface', { 'dropzone-surface--active': isDragging, 'dropzone-surface--busy': uploading }]"
      @dragenter.prevent="onDragEnter"
      @dragover.prevent="onDragOver"
      @dragleave.prevent="onDragLeave"
      @drop.prevent="onDrop"
    >
      <div class="dropzone-surface__copy">
        <span class="dropzone-kicker">{{ isDragging ? 'Release to upload' : 'Remote intake' }}</span>
        <strong>{{ uploading ? `Uploading ${uploadingCount} item${uploadingCount === 1 ? '' : 's'}...` : 'Drag files or folders here' }}</strong>
        <p>Tanuki stores each drop as its own inbox batch so the organizer can process it safely afterwards.</p>
      </div>

      <div class="dropzone-actions">
        <button type="button" class="btn btn-primary" :disabled="uploading" @click="openFilePicker">
          Choose Files
        </button>
        <button type="button" class="btn btn-ghost" :disabled="uploading" @click="openFolderPicker">
          Choose Folder
        </button>
      </div>

      <label class="dropzone-toggle">
        <input v-model="organizeAfterUpload" type="checkbox" />
        <span>Organize into the library automatically after upload</span>
      </label>
    </div>

    <div v-if="lastUpload" class="upload-summary">
      <div class="upload-summary__meta">
        <span class="upload-summary__chip">{{ lastUpload.file_count }} item{{ lastUpload.file_count === 1 ? '' : 's' }}</span>
        <span class="upload-summary__chip">{{ formatBytes(lastUpload.total_bytes) }}</span>
        <span class="upload-summary__chip">{{ lastUpload.source_path }}</span>
      </div>

      <p class="upload-summary__copy">
        {{ lastOrganizedSourcePath === lastUpload.source_path
          ? 'Organize was already queued for this inbox batch.'
          : 'Use organize to move recognized files out of the inbox and refresh the library index automatically.' }}
      </p>

      <div class="upload-summary__actions">
        <button
          type="button"
          class="btn btn-primary"
          :disabled="organizing || lastOrganizedSourcePath === lastUpload.source_path"
          @click="organizeLastUpload"
        >
          {{ organizing ? 'Queueing...' : lastOrganizedSourcePath === lastUpload.source_path ? 'Organize Queued' : 'Organize to Library' }}
        </button>
        <button type="button" class="btn btn-ghost" :disabled="uploading" @click="clearLastUpload">
          Clear
        </button>
      </div>
    </div>

    <input ref="filesInput" class="sr-only" type="file" multiple @change="onFileSelection" />
    <input ref="folderInput" class="sr-only" type="file" multiple @change="onFolderSelection" />
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import AppIcon from '@/components/Layout/AppIcon.vue'
import { libraryApi, type InboxUploadFile, type InboxUploadResult } from '@/api/libraryApi'
import { useNoticeStore } from '@/stores/noticeStore'

interface FileWithRelativePath extends File {
  webkitRelativePath?: string
}

interface WebkitFileSystemEntry {
  isFile: boolean
  isDirectory: boolean
  name: string
}

interface WebkitFileSystemFileEntry extends WebkitFileSystemEntry {
  file: (successCallback: (file: File) => void, errorCallback?: (error: DOMException) => void) => void
}

interface WebkitFileSystemDirectoryReader {
  readEntries: (
    successCallback: (entries: WebkitFileSystemEntry[]) => void,
    errorCallback?: (error: DOMException) => void,
  ) => void
}

interface WebkitFileSystemDirectoryEntry extends WebkitFileSystemEntry {
  createReader: () => WebkitFileSystemDirectoryReader
}

interface DataTransferItemWithEntry extends DataTransferItem {
  webkitGetAsEntry?: () => WebkitFileSystemEntry | null
}

const { pushNotice } = useNoticeStore()
const filesInput = ref<HTMLInputElement | null>(null)
const folderInput = ref<HTMLInputElement | null>(null)
const organizeAfterUpload = ref(true)
const lastUpload = ref<InboxUploadResult | null>(null)
const lastOrganizedSourcePath = ref('')
const uploading = ref(false)
const uploadingCount = ref(0)
const organizing = ref(false)
const isDragging = ref(false)
let dragDepth = 0

onMounted(() => {
  if (folderInput.value) {
    folderInput.value.setAttribute('webkitdirectory', '')
    folderInput.value.setAttribute('directory', '')
  }
})

function openFilePicker() {
  filesInput.value?.click()
}

function openFolderPicker() {
  folderInput.value?.click()
}

async function onFileSelection(event: Event) {
  const input = event.target as HTMLInputElement | null
  const files = collectSelectedFiles(input?.files)
  if (input) {
    input.value = ''
  }
  await startUpload(files)
}

async function onFolderSelection(event: Event) {
  const input = event.target as HTMLInputElement | null
  const files = collectSelectedFiles(input?.files)
  if (input) {
    input.value = ''
  }
  await startUpload(files)
}

function onDragEnter() {
  dragDepth += 1
  isDragging.value = true
}

function onDragOver(event: DragEvent) {
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = 'copy'
  }
}

function onDragLeave() {
  dragDepth = Math.max(0, dragDepth - 1)
  if (dragDepth === 0) {
    isDragging.value = false
  }
}

async function onDrop(event: DragEvent) {
  dragDepth = 0
  isDragging.value = false
  const files = await collectDroppedFiles(event.dataTransfer)
  await startUpload(files)
}

async function startUpload(files: InboxUploadFile[]) {
  if (uploading.value || files.length === 0) {
    return
  }

  uploading.value = true
  uploadingCount.value = files.length
  lastOrganizedSourcePath.value = ''

  try {
    const response = await libraryApi.uploadInbox(files)
    lastUpload.value = response.data
    pushNotice({
      type: 'success',
      message: `Inbox upload finished (${response.data.file_count} item${response.data.file_count === 1 ? '' : 's'})`,
    })

    if (organizeAfterUpload.value) {
      await queueOrganize(response.data)
    }
  } catch (error) {
    pushNotice({
      type: 'error',
      message: error instanceof Error ? error.message : 'Failed to upload to the inbox',
    })
  } finally {
    uploading.value = false
    uploadingCount.value = 0
  }
}

async function organizeLastUpload() {
  if (!lastUpload.value) {
    return
  }
  await queueOrganize(lastUpload.value)
}

async function queueOrganize(upload: InboxUploadResult) {
  if (organizing.value) {
    return
  }

  organizing.value = true
  try {
    const response = await libraryApi.organize(upload.source_path, 'move', false)
    const taskId = 'task_id' in response.data ? response.data.task_id : ''
    lastOrganizedSourcePath.value = upload.source_path
    pushNotice({
      type: 'success',
      message: taskId
        ? `Organize queued (${taskId.slice(0, 8)})`
        : 'Organize queued',
    })
  } catch (error) {
    pushNotice({
      type: 'error',
      message: error instanceof Error ? error.message : 'Failed to queue organize',
    })
  } finally {
    organizing.value = false
  }
}

function clearLastUpload() {
  lastUpload.value = null
  lastOrganizedSourcePath.value = ''
}

function collectSelectedFiles(fileList: FileList | null | undefined): InboxUploadFile[] {
  return Array.from(fileList ?? []).map((file) => {
    const withRelativePath = file as FileWithRelativePath
    return {
      file,
      relativePath: withRelativePath.webkitRelativePath?.trim() || file.name,
    }
  })
}

async function collectDroppedFiles(dataTransfer: DataTransfer | null): Promise<InboxUploadFile[]> {
  if (!dataTransfer) {
    return []
  }

  const items = Array.from(dataTransfer.items ?? [])
  const entries = items
    .map((item) => (item as DataTransferItemWithEntry).webkitGetAsEntry?.() ?? null)
    .filter((entry): entry is WebkitFileSystemEntry => entry !== null)

  if (entries.length === 0) {
    return collectSelectedFiles(dataTransfer.files)
  }

  const batches = await Promise.all(entries.map((entry) => walkDroppedEntry(entry, '')))
  return batches.flat()
}

async function walkDroppedEntry(entry: WebkitFileSystemEntry, parentPath: string): Promise<InboxUploadFile[]> {
  const nextPath = parentPath ? `${parentPath}/${entry.name}` : entry.name
  if (entry.isFile) {
    const file = await readDroppedFile(entry as WebkitFileSystemFileEntry)
    return [{ file, relativePath: nextPath }]
  }
  if (!entry.isDirectory) {
    return []
  }

  const children = await readAllDirectoryEntries((entry as WebkitFileSystemDirectoryEntry).createReader())
  const batches = await Promise.all(children.map((child) => walkDroppedEntry(child, nextPath)))
  return batches.flat()
}

function readDroppedFile(entry: WebkitFileSystemFileEntry): Promise<File> {
  return new Promise((resolve, reject) => {
    entry.file(resolve, reject)
  })
}

function readAllDirectoryEntries(reader: WebkitFileSystemDirectoryReader): Promise<WebkitFileSystemEntry[]> {
  const entries: WebkitFileSystemEntry[] = []
  return new Promise((resolve, reject) => {
    const readBatch = () => {
      reader.readEntries((batch) => {
        if (batch.length === 0) {
          resolve(entries)
          return
        }
        entries.push(...batch)
        readBatch()
      }, reject)
    }
    readBatch()
  })
}

function formatBytes(value: number) {
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`
  if (value < 1024 ** 3) return `${(value / 1024 / 1024).toFixed(1)} MB`
  return `${(value / 1024 ** 3).toFixed(2)} GB`
}
</script>

<style scoped>
.inbox-dropzone {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.dropzone-header {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}

.dropzone-header__icon {
  width: 36px;
  height: 36px;
  border-radius: 12px;
  background: rgba(34, 197, 94, 0.12);
  color: #4ade80;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.dropzone-header h3 {
  font-size: 16px;
  font-weight: 600;
}

.dropzone-header p {
  margin-top: 4px;
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.5;
}

.dropzone-surface {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 18px;
  border-radius: var(--radius-lg);
  border: 1px dashed color-mix(in srgb, #4ade80 42%, var(--border));
  background:
    radial-gradient(circle at top right, rgba(74, 222, 128, 0.12), transparent 32%),
    linear-gradient(180deg, rgba(255,255,255,0.02), rgba(255,255,255,0)),
    var(--bg-surface);
  transition: border-color 0.15s ease, transform 0.15s ease, box-shadow 0.15s ease;
}

.dropzone-surface--active {
  border-color: #4ade80;
  transform: translateY(-1px);
  box-shadow: 0 18px 32px rgba(34, 197, 94, 0.14);
}

.dropzone-surface--busy {
  opacity: 0.85;
}

.dropzone-surface__copy {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.dropzone-kicker {
  font-size: 11px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: #4ade80;
}

.dropzone-surface__copy strong {
  font-size: 16px;
  font-weight: 600;
}

.dropzone-surface__copy p {
  color: var(--text-muted);
  font-size: 12px;
  line-height: 1.5;
}

.dropzone-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.dropzone-toggle {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--text-secondary);
  font-size: 12px;
}

.dropzone-toggle input {
  margin: 0;
}

.upload-summary {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 14px;
  border-radius: var(--radius-lg);
  border: 1px solid var(--border);
  background: var(--bg-surface);
}

.upload-summary__meta {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.upload-summary__chip {
  display: inline-flex;
  align-items: center;
  min-height: 28px;
  padding: 0 10px;
  border-radius: 999px;
  border: 1px solid var(--border);
  background: var(--bg-card);
  color: var(--text-secondary);
  font-size: 12px;
}

.upload-summary__copy {
  color: var(--text-muted);
  font-size: 12px;
  line-height: 1.5;
}

.upload-summary__actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}
</style>
