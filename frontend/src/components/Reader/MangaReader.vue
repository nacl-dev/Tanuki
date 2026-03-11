<template>
  <div
    ref="overlay"
    class="mr-overlay"
    tabindex="0"
    aria-label="Manga reader"
    @keydown="onKey"
    @wheel="onWheel"
  >
    <div class="mr-toolbar" :class="{ 'mr-toolbar-hidden': toolbarHidden }">
      <div class="mr-toolbar-left">
        <button type="button" class="mr-btn" @click="emit('close')" title="Back">Back</button>
        <span class="mr-page-indicator">Page {{ displayPageNumber }} / {{ totalPages }}</span>
        <span class="mr-page-progress">{{ progressLabel }}</span>
      </div>

      <div class="mr-toolbar-center">
        <button
          v-for="item in readingModes"
          :key="item.value"
          type="button"
          :class="['mr-btn', { 'mr-btn-active': mode === item.value }]"
          :title="item.label"
          @click="mode = item.value"
        >
          {{ item.label }}
        </button>
        <button type="button" :class="['mr-btn', { 'mr-btn-active': rtl }]" title="RTL (right-to-left)" @click="rtl = !rtl">
          RTL
        </button>
        <button
          v-if="mode === 'double'"
          type="button"
          :class="['mr-btn', { 'mr-btn-active': separateCover }]"
          title="Treat the cover as a standalone page"
          @click="separateCover = !separateCover"
        >
          Cover Solo
        </button>
      </div>

      <div class="mr-toolbar-right">
        <button type="button" class="mr-btn" :class="{ 'mr-btn-active': zoomMode === 'fit-width' }" @click="setZoom('fit-width')">Fit W</button>
        <button type="button" class="mr-btn" :class="{ 'mr-btn-active': zoomMode === 'fit-height' }" @click="setZoom('fit-height')">Fit H</button>
        <button type="button" class="mr-btn" aria-label="Zoom in" title="Zoom in (+)" @click="zoomIn">+</button>
        <button type="button" class="mr-btn" aria-label="Zoom out" title="Zoom out (-)" @click="zoomOut">-</button>
        <button type="button" class="mr-btn" title="Show shortcuts and controls (?)" @click="showHelp = true">?</button>
        <button type="button" class="mr-btn" :title="isFullscreen ? 'Exit fullscreen (F)' : 'Fullscreen (F)'" @click="toggleFullscreen">
          {{ isFullscreen ? 'Exit Full' : 'Fullscreen' }}
        </button>
      </div>
    </div>

    <div
      ref="viewport"
      :class="['mr-viewport', { 'mr-viewport-scroll': mode === 'scroll' }]"
      @mousemove="onActivity"
      @click="onViewportClick"
    >
      <template v-if="mode === 'single'">
        <div class="mr-single-page">
          <img :src="pageUrl(currentPage)" :alt="`Page ${currentPage + 1}`" class="mr-page-img" :style="imgStyle" />
        </div>
      </template>

      <template v-else-if="mode === 'double'">
        <div class="mr-double-page" :class="{ 'mr-rtl': rtl }">
          <img
            v-if="firstSpreadPage !== null"
            :src="pageUrl(firstSpreadPage)"
            :alt="`Page ${firstSpreadPage + 1}`"
            class="mr-page-img"
            :style="imgStyle"
          />
          <img
            v-if="secondSpreadPage !== null"
            :src="pageUrl(secondSpreadPage)"
            :alt="`Page ${secondSpreadPage + 1}`"
            class="mr-page-img"
            :style="imgStyle"
          />
        </div>
      </template>

      <template v-else>
        <div class="mr-scroll-list">
          <div
            v-for="(page, idx) in pages"
            :key="page.index"
            :ref="(el) => setPageRef(el as HTMLElement | null, idx)"
            class="mr-scroll-item"
          >
            <img
              v-if="isPageVisible(idx)"
              :src="pageUrl(idx)"
              :alt="`Page ${idx + 1}`"
              class="mr-page-img mr-scroll-img"
              :style="scrollImgStyle"
              loading="lazy"
            />
            <div v-else class="mr-page-placeholder" :style="scrollImgStyle" />
          </div>
        </div>
      </template>
    </div>

    <template v-if="mode !== 'scroll'">
      <div class="mr-click-zone mr-click-prev" :title="rtl ? 'Next' : 'Previous'" @click.stop="prevPage" />
      <div class="mr-click-zone mr-click-next" :title="rtl ? 'Previous' : 'Next'" @click.stop="nextPage" />
    </template>

    <div class="mr-scrubber" :class="{ 'mr-scrubber-hidden': toolbarHidden }">
      <div class="mr-scrubber-row">
        <input
          type="range"
          class="mr-scrubber-input"
          min="0"
          :max="totalPages - 1"
          :value="currentPage"
          @input="onScrub"
        />
        <form class="mr-page-jump" @submit.prevent="jumpToPage">
          <label class="mr-page-jump-label" for="mr-page-jump">Jump to page</label>
          <input
            id="mr-page-jump"
            v-model="pageInput"
            type="number"
            class="mr-page-jump-input"
            min="1"
            :max="totalPages"
          />
          <button type="submit" class="mr-btn">Go</button>
        </form>
      </div>
    </div>
  </div>

  <ModalShell
    v-if="showHelp"
    title="Reader shortcuts"
    description="Keyboard, click, and layout controls available while reading."
    size="sm"
    @close="showHelp = false"
  >
    <div class="mr-help-list">
      <div class="mr-help-row"><strong>Left / Right</strong><span>Previous or next page</span></div>
      <div class="mr-help-row"><strong>Home / End</strong><span>Jump to first or last page</span></div>
      <div class="mr-help-row"><strong>+ / -</strong><span>Zoom in or out</span></div>
      <div class="mr-help-row"><strong>F</strong><span>Toggle fullscreen reader</span></div>
      <div class="mr-help-row"><strong>?</strong><span>Open or close this help</span></div>
      <div class="mr-help-row"><strong>Ctrl + wheel</strong><span>Free zoom mode</span></div>
    </div>
  </ModalShell>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { mediaPageUrl, type PageInfo } from '@/api/mediaApi'
import ModalShell from '@/components/Layout/ModalShell.vue'

type ReadingMode = 'single' | 'double' | 'scroll'
type ZoomMode = 'fit-width' | 'fit-height' | 'custom'

const READER_SETTINGS_KEY = 'tanuki:manga-reader:settings'

const props = defineProps<{
  mediaId: string
  totalPages: number
  pages: PageInfo[]
  initialPage?: number
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'pagechange', page: number): void
}>()

const overlay = ref<HTMLDivElement>()
const viewport = ref<HTMLDivElement>()
const mode = ref<ReadingMode>('single')
const rtl = ref(false)
const currentPage = ref(props.initialPage ?? 0)
const zoomMode = ref<ZoomMode>('fit-width')
const zoomScale = ref(1)
const separateCover = ref(true)
const toolbarHidden = ref(false)
const isFullscreen = ref(false)
const showHelp = ref(false)
const pageInput = ref(String((props.initialPage ?? 0) + 1))
const pageRefs = ref<(HTMLElement | null)[]>([])
const visiblePages = ref<Set<number>>(new Set([0, 1, 2]))

let hideTimer: ReturnType<typeof setTimeout> | null = null
let progressObserver: IntersectionObserver | null = null
let syncingFromScroll = false

const readingModes: { value: ReadingMode; label: string }[] = [
  { value: 'single', label: 'Single' },
  { value: 'double', label: 'Double' },
  { value: 'scroll', label: 'Scroll' },
]

const displayPageNumber = computed(() => currentPage.value + 1)
const progressLabel = computed(() =>
  `${Math.round(((currentPage.value + 1) / Math.max(1, props.totalPages)) * 100)}%`,
)
const leftPage = computed(() => (currentPage.value % 2 === 0 ? currentPage.value : currentPage.value - 1))
const spreadStart = computed(() => {
  if (!separateCover.value || currentPage.value === 0 || props.totalPages <= 1) {
    return leftPage.value
  }
  if (currentPage.value <= 1) return 1
  return currentPage.value % 2 === 1 ? currentPage.value : currentPage.value - 1
})
const firstSpreadPage = computed<number | null>(() => {
  if (mode.value !== 'double') return null
  if (separateCover.value && currentPage.value === 0) return 0
  const page = rtl.value ? spreadStart.value + 1 : spreadStart.value
  return page >= 0 && page < props.totalPages ? page : null
})
const secondSpreadPage = computed<number | null>(() => {
  if (mode.value !== 'double') return null
  if (separateCover.value && currentPage.value === 0) return null
  const page = rtl.value ? spreadStart.value : spreadStart.value + 1
  return page >= 0 && page < props.totalPages ? page : null
})
const imgStyle = computed(() => {
  if (zoomMode.value === 'fit-width') {
    return { maxWidth: '100%', height: 'auto' }
  }
  if (zoomMode.value === 'fit-height') {
    return { maxHeight: '100%', width: 'auto' }
  }
  return { transform: `scale(${zoomScale.value})`, transformOrigin: 'top center' }
})
const scrollImgStyle = computed(() => {
  if (zoomMode.value === 'fit-width' || zoomMode.value === 'fit-height') {
    return { width: '100%', height: 'auto' }
  }
  return { width: `${zoomScale.value * 100}%`, height: 'auto' }
})

function pageUrl(idx: number): string {
  return mediaPageUrl(props.mediaId, idx)
}

function persistSettings() {
  if (typeof window === 'undefined') return
  window.localStorage.setItem(
    READER_SETTINGS_KEY,
    JSON.stringify({
      mode: mode.value,
      rtl: rtl.value,
      zoomMode: zoomMode.value,
      zoomScale: zoomScale.value,
      separateCover: separateCover.value,
    }),
  )
}

function loadSettings() {
  if (typeof window === 'undefined') return
  try {
    const raw = window.localStorage.getItem(READER_SETTINGS_KEY)
    if (!raw) return
    const parsed = JSON.parse(raw) as {
      mode?: ReadingMode
      rtl?: boolean
      zoomMode?: ZoomMode
      zoomScale?: number
      separateCover?: boolean
    }
    if (parsed.mode && readingModes.some((item) => item.value === parsed.mode)) {
      mode.value = parsed.mode
    }
    if (typeof parsed.rtl === 'boolean') rtl.value = parsed.rtl
    if (parsed.zoomMode) zoomMode.value = parsed.zoomMode
    if (typeof parsed.zoomScale === 'number') {
      zoomScale.value = Math.max(0.2, Math.min(4, parsed.zoomScale))
    }
    if (typeof parsed.separateCover === 'boolean') separateCover.value = parsed.separateCover
  } catch {
    // Ignore invalid reader settings.
  }
}

function syncPageState(page: number) {
  currentPage.value = Math.max(0, Math.min(props.totalPages - 1, page))
  pageInput.value = String(currentPage.value + 1)
  emit('pagechange', currentPage.value)
}

function prevPage() {
  if (rtl.value) {
    goNext()
  } else {
    goPrev()
  }
}

function nextPage() {
  if (rtl.value) {
    goPrev()
  } else {
    goNext()
  }
}

function goPrev() {
  if (mode.value === 'double') {
    if (separateCover.value && currentPage.value <= 2) {
      syncPageState(0)
      return
    }
    syncPageState(currentPage.value - 2)
    return
  }
  syncPageState(currentPage.value - 1)
}

function goNext() {
  if (mode.value === 'double') {
    if (separateCover.value && currentPage.value === 0 && props.totalPages > 1) {
      syncPageState(1)
      return
    }
    syncPageState(currentPage.value + 2)
    return
  }
  syncPageState(currentPage.value + 1)
}

function goFirst() {
  syncPageState(0)
}

function goLast() {
  syncPageState(props.totalPages - 1)
}

function onScrub(e: Event) {
  syncPageState(parseInt((e.target as HTMLInputElement).value, 10))
}

function jumpToPage() {
  const parsed = Number(pageInput.value)
  if (!Number.isFinite(parsed)) return
  syncPageState(Math.round(parsed) - 1)
}

function onViewportClick(e: MouseEvent) {
  if (mode.value === 'scroll') return
  const target = e.currentTarget as HTMLElement
  const rect = target.getBoundingClientRect()
  const relX = (e.clientX - rect.left) / rect.width
  if (relX < 0.5) {
    prevPage()
  } else {
    nextPage()
  }
}

function setZoom(nextZoomMode: ZoomMode) {
  zoomMode.value = nextZoomMode
}

function zoomIn() {
  zoomMode.value = 'custom'
  zoomScale.value = Math.min(4, zoomScale.value + 0.2)
}

function zoomOut() {
  zoomMode.value = 'custom'
  zoomScale.value = Math.max(0.2, zoomScale.value - 0.2)
}

function onWheel(e: WheelEvent) {
  if (e.ctrlKey || e.metaKey) {
    e.preventDefault()
    zoomMode.value = 'custom'
    zoomScale.value = Math.max(0.2, Math.min(4, zoomScale.value - e.deltaY * 0.001))
    return
  }

  if (mode.value === 'scroll') {
    return
  }

  if (Math.abs(e.deltaY) < 8) {
    return
  }

  e.preventDefault()
  if (e.deltaY > 0) {
    nextPage()
  } else {
    prevPage()
  }
}

async function toggleFullscreen() {
  const el = overlay.value
  if (!el) return
  if (!document.fullscreenElement) {
    await el.requestFullscreen()
    return
  }
  if (document.fullscreenElement === el) {
    await document.exitFullscreen()
  }
}

function onActivity() {
  toolbarHidden.value = false
  if (hideTimer) clearTimeout(hideTimer)
  hideTimer = setTimeout(() => {
    toolbarHidden.value = true
  }, 3000)
}

function onKey(e: KeyboardEvent) {
  switch (e.key) {
    case 'ArrowLeft':
      e.preventDefault()
      prevPage()
      break
    case 'ArrowRight':
      e.preventDefault()
      nextPage()
      break
    case 'Home':
      e.preventDefault()
      goFirst()
      break
    case 'End':
      e.preventDefault()
      goLast()
      break
    case '+':
    case '=':
      e.preventDefault()
      zoomIn()
      break
    case '-':
      e.preventDefault()
      zoomOut()
      break
    case 'f':
    case 'F':
      e.preventDefault()
      void toggleFullscreen()
      break
    case '?':
      e.preventDefault()
      showHelp.value = !showHelp.value
      break
    case 'Escape':
      if (showHelp.value) {
        e.preventDefault()
        showHelp.value = false
      } else if (document.fullscreenElement === overlay.value) {
        e.preventDefault()
        void document.exitFullscreen()
      }
      break
  }
  onActivity()
}

function onFullscreenChange() {
  isFullscreen.value = document.fullscreenElement === overlay.value
}

function setPageRef(el: HTMLElement | null, idx: number) {
  pageRefs.value[idx] = el
}

function isPageVisible(idx: number): boolean {
  return visiblePages.value.has(idx)
}

async function scrollCurrentPageIntoView(behavior: 'auto' | 'smooth' = 'auto') {
  await nextTick()
  const target = pageRefs.value[currentPage.value]
  const container = viewport.value
  if (!target || !container || mode.value !== 'scroll') return

  const top = Math.max(0, target.offsetTop - 60)
  container.scrollTo({ top, behavior })
}

function setupObserver() {
  if (progressObserver) progressObserver.disconnect()
  progressObserver = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        const idx = parseInt((entry.target as HTMLElement).dataset.pageIdx ?? '-1', 10)
        if (idx < 0) continue
        if (entry.isIntersecting) {
          for (let i = Math.max(0, idx - 2); i <= Math.min(props.totalPages - 1, idx + 2); i++) {
            visiblePages.value.add(i)
          }
          syncingFromScroll = true
          syncPageState(idx)
          requestAnimationFrame(() => {
            syncingFromScroll = false
          })
        }
      }
    },
    { root: viewport.value, threshold: 0.3 },
  )
}

async function attachScrollObserver() {
  await nextTick()
  setupObserver()
  pageRefs.value.forEach((el, idx) => {
    if (el) {
      el.dataset.pageIdx = String(idx)
      progressObserver?.observe(el)
    }
  })
  const initial = currentPage.value
  for (let i = Math.max(0, initial - 2); i <= Math.min(props.totalPages - 1, initial + 2); i++) {
    visiblePages.value.add(i)
  }
  await scrollCurrentPageIntoView()
}

watch(mode, async (newMode) => {
  if (newMode === 'scroll') {
    await attachScrollObserver()
  } else {
    progressObserver?.disconnect()
  }
})

watch(currentPage, async () => {
  if (mode.value !== 'scroll' || syncingFromScroll) return
  await scrollCurrentPageIntoView('smooth')
})

watch([mode, rtl, zoomMode, zoomScale, separateCover], () => {
  persistSettings()
})

onMounted(async () => {
  loadSettings()
  overlay.value?.focus()
  onActivity()
  document.addEventListener('fullscreenchange', onFullscreenChange)
  for (let i = 0; i <= Math.min(2, props.totalPages - 1); i++) {
    visiblePages.value.add(i)
  }
  pageInput.value = String(currentPage.value + 1)
  if (mode.value === 'scroll') {
    await attachScrollObserver()
  }
})

onUnmounted(() => {
  if (hideTimer) clearTimeout(hideTimer)
  progressObserver?.disconnect()
  document.removeEventListener('fullscreenchange', onFullscreenChange)
})
</script>

<style scoped>
.mr-overlay {
  position: fixed;
  inset: 0;
  z-index: 1000;
  background: #111113;
  display: flex;
  flex-direction: column;
  outline: none;
  overflow: hidden;
}

.mr-overlay:focus-visible {
  outline: 2px solid var(--focus-ring);
  outline-offset: 2px;
}

.mr-toolbar {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 16px;
  background: linear-gradient(rgba(0, 0, 0, 0.85), transparent);
  transition: opacity 0.25s;
}

.mr-toolbar-hidden {
  opacity: 0;
  pointer-events: none;
}

.mr-toolbar-left,
.mr-toolbar-center,
.mr-toolbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.mr-toolbar-center {
  flex: 1;
  justify-content: center;
}

.mr-page-indicator {
  font-size: 13px;
  color: rgba(255, 255, 255, 0.8);
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}

.mr-page-progress {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.58);
  white-space: nowrap;
}

.mr-btn {
  background: rgba(255, 255, 255, 0.1);
  border: 1px solid rgba(255, 255, 255, 0.15);
  color: #fff;
  padding: 4px 10px;
  border-radius: 4px;
  font-size: 12px;
  cursor: pointer;
  white-space: nowrap;
  transition: background 0.15s, color 0.15s;
}

.mr-btn:hover {
  background: rgba(245, 158, 11, 0.25);
  color: #f59e0b;
}

.mr-btn-active {
  background: rgba(245, 158, 11, 0.3);
  color: #f59e0b;
  border-color: #f59e0b;
}

.mr-btn:focus-visible,
.mr-scrubber-input:focus-visible,
.mr-page-jump-input:focus-visible {
  outline: 2px solid var(--focus-ring);
  outline-offset: 2px;
}

.mr-viewport {
  flex: 1;
  overflow: auto;
  display: flex;
  align-items: center;
  justify-content: center;
}

.mr-viewport-scroll {
  display: block;
}

.mr-single-page,
.mr-double-page {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
}

.mr-double-page {
  gap: 4px;
}

.mr-rtl {
  flex-direction: row-reverse;
}

.mr-page-img {
  display: block;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.6);
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}

.mr-scroll-list {
  width: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 60px 0 80px;
}

.mr-scroll-item {
  width: 100%;
  display: flex;
  justify-content: center;
}

.mr-scroll-img,
.mr-page-placeholder {
  max-width: 900px;
}

.mr-page-placeholder {
  width: 100%;
  aspect-ratio: 3 / 4;
  background: #1a1a1e;
  border-radius: 4px;
}

.mr-click-zone {
  position: fixed;
  top: 48px;
  bottom: 48px;
  width: 25%;
  z-index: 5;
  cursor: pointer;
}

.mr-click-prev {
  left: 0;
}

.mr-click-next {
  right: 0;
}

.mr-scrubber {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  padding: 8px 16px 12px;
  background: linear-gradient(transparent, rgba(0, 0, 0, 0.75));
  transition: opacity 0.25s;
}

.mr-scrubber-hidden {
  opacity: 0;
  pointer-events: none;
}

.mr-scrubber-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.mr-scrubber-input {
  width: 100%;
  -webkit-appearance: none;
  appearance: none;
  height: 4px;
  background: rgba(255, 255, 255, 0.2);
  border-radius: 2px;
  outline: none;
  cursor: pointer;
}

.mr-scrubber-input::-webkit-slider-thumb {
  -webkit-appearance: none;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: #f59e0b;
  cursor: pointer;
}

.mr-page-jump {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.mr-page-jump-label {
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

.mr-page-jump-input {
  width: 84px;
  padding: 6px 8px;
  border-radius: 8px;
  border: 1px solid rgba(255, 255, 255, 0.18);
  background: rgba(255, 255, 255, 0.08);
  color: #fff;
}

.mr-help-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.mr-help-row {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  font-size: 13px;
}

.mr-help-row span {
  color: var(--text-muted);
  text-align: right;
}

@media (max-width: 920px) {
  .mr-toolbar {
    flex-direction: column;
    align-items: stretch;
  }

  .mr-toolbar-left,
  .mr-toolbar-center,
  .mr-toolbar-right,
  .mr-scrubber-row {
    flex-wrap: wrap;
    justify-content: flex-start;
  }

  .mr-page-jump {
    width: 100%;
  }

  .mr-page-jump-input {
    flex: 1;
  }
}
</style>
