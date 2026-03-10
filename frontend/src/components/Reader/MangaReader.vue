<template>
  <div
    class="mr-overlay"
    tabindex="0"
    ref="overlay"
    @keydown="onKey"
    @wheel.prevent="onWheel"
  >
    <!-- Toolbar -->
    <div class="mr-toolbar" :class="{ 'mr-toolbar-hidden': toolbarHidden }">
      <div class="mr-toolbar-left">
        <button class="mr-btn" @click="emit('close')" title="Back">← Back</button>
        <span class="mr-page-indicator">Page {{ currentPage + 1 }} / {{ totalPages }}</span>
      </div>
      <div class="mr-toolbar-center">
        <button
          v-for="m in readingModes"
          :key="m.value"
          :class="['mr-btn', { 'mr-btn-active': mode === m.value }]"
          @click="mode = m.value"
          :title="m.label"
        >{{ m.label }}</button>
        <button
          :class="['mr-btn', { 'mr-btn-active': rtl }]"
          @click="rtl = !rtl"
          title="RTL (right-to-left)"
        >RTL</button>
      </div>
      <div class="mr-toolbar-right">
        <button class="mr-btn" @click="setZoom('fit-width')" :class="{ 'mr-btn-active': zoomMode === 'fit-width' }">Fit W</button>
        <button class="mr-btn" @click="setZoom('fit-height')" :class="{ 'mr-btn-active': zoomMode === 'fit-height' }">Fit H</button>
        <button class="mr-btn" @click="zoomIn" title="Zoom in (+)">＋</button>
        <button class="mr-btn" @click="zoomOut" title="Zoom out (-)">－</button>
      </div>
    </div>

    <!-- Viewport -->
    <div
      class="mr-viewport"
      @mousemove="onActivity"
      @click="onViewportClick"
    >
      <!-- Single page mode -->
      <template v-if="mode === 'single'">
        <div class="mr-single-page">
          <img
            :src="pageUrl(currentPage)"
            :alt="`Page ${currentPage + 1}`"
            class="mr-page-img"
            :style="imgStyle"
            @load="onImgLoad"
          />
        </div>
      </template>

      <!-- Double page mode -->
      <template v-else-if="mode === 'double'">
        <div class="mr-double-page" :class="{ 'mr-rtl': rtl }">
          <img
            v-if="!rtl ? leftPage >= 0 : rightPage >= 0"
            :src="pageUrl(!rtl ? leftPage : rightPage)"
            :alt="`Page ${(!rtl ? leftPage : rightPage) + 1}`"
            class="mr-page-img"
            :style="imgStyle"
          />
          <img
            v-if="!rtl ? rightPage < totalPages : leftPage < totalPages"
            :src="pageUrl(!rtl ? rightPage : leftPage)"
            :alt="`Page ${(!rtl ? rightPage : leftPage) + 1}`"
            class="mr-page-img"
            :style="imgStyle"
          />
        </div>
      </template>

      <!-- Continuous scroll mode -->
      <template v-else>
        <div class="mr-scroll-list">
          <div
            v-for="(page, idx) in pages"
            :key="page.index"
            :ref="el => setPageRef(el as HTMLElement, idx)"
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

    <!-- Click zones (single / double mode) -->
    <template v-if="mode !== 'scroll'">
      <div
        class="mr-click-zone mr-click-prev"
        @click.stop="prevPage"
        :title="rtl ? 'Next' : 'Previous'"
      />
      <div
        class="mr-click-zone mr-click-next"
        @click.stop="nextPage"
        :title="rtl ? 'Previous' : 'Next'"
      />
    </template>

    <!-- Page scrubber -->
    <div class="mr-scrubber" :class="{ 'mr-scrubber-hidden': toolbarHidden }">
      <input
        type="range"
        class="mr-scrubber-input"
        min="0"
        :max="totalPages - 1"
        :value="currentPage"
        @input="onScrub"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { mediaPageUrl, type PageInfo } from '@/api/mediaApi'

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

type ReadingMode = 'single' | 'double' | 'scroll'
type ZoomMode = 'fit-width' | 'fit-height' | 'custom'

const overlay = ref<HTMLDivElement>()
const mode = ref<ReadingMode>('single')
const rtl = ref(false)
const currentPage = ref(props.initialPage ?? 0)
const zoomMode = ref<ZoomMode>('fit-width')
const zoomScale = ref(1)
const toolbarHidden = ref(false)
const pageRefs = ref<(HTMLElement | null)[]>([])
const visiblePages = ref<Set<number>>(new Set([0, 1, 2]))

let hideTimer: ReturnType<typeof setTimeout> | null = null
let progressObserver: IntersectionObserver | null = null

const readingModes: { value: ReadingMode; label: string }[] = [
  { value: 'single', label: 'Single' },
  { value: 'double', label: 'Double' },
  { value: 'scroll', label: 'Scroll' },
]

function pageUrl(idx: number): string {
  return mediaPageUrl(props.mediaId, idx)
}

const leftPage = computed(() => currentPage.value % 2 === 0 ? currentPage.value : currentPage.value - 1)
const rightPage = computed(() => leftPage.value + 1)

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
    currentPage.value = Math.max(0, currentPage.value - 2)
  } else {
    currentPage.value = Math.max(0, currentPage.value - 1)
  }
  emit('pagechange', currentPage.value)
}

function goNext() {
  if (mode.value === 'double') {
    currentPage.value = Math.min(props.totalPages - 1, currentPage.value + 2)
  } else {
    currentPage.value = Math.min(props.totalPages - 1, currentPage.value + 1)
  }
  emit('pagechange', currentPage.value)
}

function goFirst() {
  currentPage.value = 0
  emit('pagechange', 0)
}

function goLast() {
  currentPage.value = props.totalPages - 1
  emit('pagechange', props.totalPages - 1)
}

function onScrub(e: Event) {
  const val = parseInt((e.target as HTMLInputElement).value, 10)
  currentPage.value = val
  emit('pagechange', val)
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

function setZoom(z: ZoomMode) {
  zoomMode.value = z
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
    zoomMode.value = 'custom'
    zoomScale.value = Math.max(0.2, Math.min(4, zoomScale.value - e.deltaY * 0.001))
  }
}

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

function onImgLoad() {
  // nothing special needed
}

function onActivity() {
  toolbarHidden.value = false
  if (hideTimer) clearTimeout(hideTimer)
  hideTimer = setTimeout(() => { toolbarHidden.value = true }, 3000)
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
  }
  onActivity()
}

function setPageRef(el: HTMLElement | null, idx: number) {
  pageRefs.value[idx] = el
}

function isPageVisible(idx: number): boolean {
  return visiblePages.value.has(idx)
}

function setupObserver() {
  if (progressObserver) progressObserver.disconnect()
  progressObserver = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        const idx = parseInt((entry.target as HTMLElement).dataset.pageIdx ?? '-1', 10)
        if (idx < 0) continue
        if (entry.isIntersecting) {
          // Load current ± 2 pages
          for (let i = Math.max(0, idx - 2); i <= Math.min(props.totalPages - 1, idx + 2); i++) {
            visiblePages.value.add(i)
          }
          currentPage.value = idx
          emit('pagechange', idx)
        }
      }
    },
    { threshold: 0.3 },
  )
}

watch(mode, async (newMode) => {
  if (newMode === 'scroll') {
    await nextTick()
    setupObserver()
    pageRefs.value.forEach((el, idx) => {
      if (el) {
        el.dataset.pageIdx = String(idx)
        progressObserver?.observe(el)
      }
    })
    // Seed initial visible pages
    const initial = currentPage.value
    for (let i = Math.max(0, initial - 2); i <= Math.min(props.totalPages - 1, initial + 2); i++) {
      visiblePages.value.add(i)
    }
  } else {
    progressObserver?.disconnect()
  }
})

onMounted(() => {
  overlay.value?.focus()
  onActivity()
  // Seed initial visible range
  for (let i = 0; i <= Math.min(2, props.totalPages - 1); i++) {
    visiblePages.value.add(i)
  }
})

onUnmounted(() => {
  if (hideTimer) clearTimeout(hideTimer)
  progressObserver?.disconnect()
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

/* Toolbar */
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
  background: linear-gradient(rgba(0,0,0,0.85), transparent);
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

.mr-toolbar-center { flex: 1; justify-content: center; }

.mr-page-indicator {
  font-size: 13px;
  color: rgba(255,255,255,0.8);
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}

.mr-btn {
  background: rgba(255,255,255,0.1);
  border: 1px solid rgba(255,255,255,0.15);
  color: #fff;
  padding: 4px 10px;
  border-radius: 4px;
  font-size: 12px;
  cursor: pointer;
  white-space: nowrap;
  transition: background 0.15s, color 0.15s;
}

.mr-btn:hover { background: rgba(245,158,11,0.25); color: #f59e0b; }
.mr-btn-active { background: rgba(245,158,11,0.3); color: #f59e0b; border-color: #f59e0b; }

/* Viewport */
.mr-viewport {
  flex: 1;
  overflow: auto;
  display: flex;
  align-items: center;
  justify-content: center;
}

.mr-single-page {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
}

.mr-double-page {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  width: 100%;
  height: 100%;
}

.mr-rtl { flex-direction: row-reverse; }

.mr-page-img {
  display: block;
  box-shadow: 0 4px 24px rgba(0,0,0,0.6);
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

.mr-scroll-item { width: 100%; display: flex; justify-content: center; }

.mr-scroll-img { max-width: 900px; margin: 0 auto; }

.mr-page-placeholder {
  max-width: 900px;
  width: 100%;
  aspect-ratio: 3 / 4;
  background: #1a1a1e;
  border-radius: 4px;
}

/* Click zones */
.mr-click-zone {
  position: fixed;
  top: 48px;
  bottom: 48px;
  width: 25%;
  z-index: 5;
  cursor: pointer;
}

.mr-click-prev { left: 0; }
.mr-click-next { right: 0; }

/* Scrubber */
.mr-scrubber {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  padding: 8px 16px 12px;
  background: linear-gradient(transparent, rgba(0,0,0,0.75));
  transition: opacity 0.25s;
}

.mr-scrubber-hidden {
  opacity: 0;
  pointer-events: none;
}

.mr-scrubber-input {
  width: 100%;
  -webkit-appearance: none;
  appearance: none;
  height: 4px;
  background: rgba(255,255,255,0.2);
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
</style>
