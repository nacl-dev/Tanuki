<template>
  <div
    ref="container"
    class="vp-container"
    :class="{ 'vp-fullscreen': isFullscreen, 'vp-controls-hidden': controlsHidden }"
    tabindex="0"
    aria-label="Video player"
    @mousemove="onActivity"
    @mouseenter="onActivity"
    @keydown="onKey"
    @click="onContainerClick"
  >
    <video
      ref="video"
      class="vp-video"
      :src="src"
      :poster="poster"
      @timeupdate="onTimeUpdate"
      @ended="onEnded"
      @error="onError"
      @loadedmetadata="onMetadata"
      @waiting="buffering = true"
      @canplay="buffering = false"
      @play="playing = true"
      @pause="playing = false"
    />

    <div v-if="buffering" class="vp-spinner" aria-hidden="true">
      <AppIcon name="refresh" :size="34" class="vp-spinner-icon" />
    </div>

    <div class="vp-controls" @click.stop>
      <div class="vp-progress" ref="progressBar" @mousedown="onSeekStart" @click="onSeekClick">
        <div class="vp-progress-track">
          <div class="vp-progress-fill" :style="{ width: progressPct + '%' }" />
          <div class="vp-progress-thumb" :style="{ left: progressPct + '%' }" />
        </div>
      </div>

      <div class="vp-bar">
        <div class="vp-bar-left">
          <button type="button" class="vp-btn" aria-label="Rewind 10 seconds" title="Back 10s (J)" @click="seek(-10)">
            <AppIcon name="rewind" :size="18" />
          </button>
          <button
            type="button"
            class="vp-btn"
            :aria-label="playing ? 'Pause video' : 'Play video'"
            :title="playing ? 'Pause (Space)' : 'Play (Space)'"
            @click="togglePlay"
          >
            <AppIcon :name="playing ? 'pause' : 'play'" :size="18" />
          </button>
          <button type="button" class="vp-btn" aria-label="Forward 10 seconds" title="Forward 10s (L)" @click="seek(10)">
            <AppIcon name="forward" :size="18" />
          </button>

          <div class="vp-volume">
            <button
              type="button"
              class="vp-btn"
              :aria-label="muted ? 'Unmute video' : 'Mute video'"
              :title="muted ? 'Unmute (M)' : 'Mute (M)'"
              @click="toggleMute"
            >
              <AppIcon :name="volumeIcon" :size="18" />
            </button>
            <input
              class="vp-slider vp-volume-slider"
              type="range"
              min="0"
              max="1"
              step="0.02"
              :value="muted ? 0 : volume"
              @input="onVolumeInput"
            />
          </div>

          <button
            type="button"
            class="vp-time vp-time-btn"
            :title="showRemaining ? 'Show elapsed / total time' : 'Show time remaining'"
            @click="showRemaining = !showRemaining"
          >
            {{ timeLabel }}
          </button>
        </div>

        <div class="vp-bar-right">
          <select class="vp-speed" :value="playbackRate" title="Playback speed" @change="onSpeedChange">
            <option v-for="s in speeds" :key="s" :value="s">{{ s }}x</option>
          </select>
          <button
            v-if="supportsPictureInPicture"
            type="button"
            class="vp-btn"
            :aria-label="isPictureInPicture ? 'Exit picture in picture' : 'Enter picture in picture'"
            :title="isPictureInPicture ? 'Exit Picture-in-Picture (I)' : 'Picture-in-Picture (I)'"
            @click="togglePictureInPicture"
          >
            <AppIcon name="pip" :size="18" />
          </button>
          <button
            type="button"
            class="vp-btn"
            :aria-label="isFullscreen ? 'Exit fullscreen' : 'Enter fullscreen'"
            :title="isFullscreen ? 'Exit Fullscreen (F)' : 'Fullscreen (F)'"
            @click="toggleFullscreen"
          >
            <AppIcon :name="isFullscreen ? 'collapse' : 'expand'" :size="18" />
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import AppIcon from '@/components/Layout/AppIcon.vue'

const PLAYER_SETTINGS_KEY = 'tanuki:video-player:settings'

const props = defineProps<{
  src: string
  poster?: string
  initialTime?: number
}>()

const emit = defineEmits<{
  (e: 'timeupdate', time: number): void
  (e: 'ended'): void
  (e: 'error', err: Event): void
}>()

const container = ref<HTMLDivElement>()
const video = ref<HTMLVideoElement>()
const progressBar = ref<HTMLDivElement>()

const playing = ref(false)
const currentTime = ref(0)
const duration = ref(0)
const volume = ref(1)
const muted = ref(false)
const playbackRate = ref(1)
const showRemaining = ref(false)
const isFullscreen = ref(false)
const isPictureInPicture = ref(false)
const buffering = ref(false)
const controlsHidden = ref(false)

const speeds = [0.25, 0.5, 0.75, 1, 1.25, 1.5, 2, 3]

let hideTimer: ReturnType<typeof setTimeout> | null = null
let seekDragging = false
let resumeApplied = false

const progressPct = computed(() => (duration.value > 0 ? (currentTime.value / duration.value) * 100 : 0))
const volumeIcon = computed<'volumeOff' | 'volumeLow' | 'volumeHigh'>(() => {
  if (muted.value || volume.value === 0) return 'volumeOff'
  if (volume.value < 0.5) return 'volumeLow'
  return 'volumeHigh'
})
const timeLabel = computed(() => {
  if (showRemaining.value && duration.value > 0) {
    return `${formatTime(Math.max(0, duration.value - currentTime.value))} left`
  }
  return `${formatTime(currentTime.value)} / ${formatTime(duration.value)}`
})
const supportsPictureInPicture = computed(() => {
  if (typeof document === 'undefined') return false
  const pipVideo = video.value as (HTMLVideoElement & { disablePictureInPicture?: boolean }) | undefined
  return Boolean(document.pictureInPictureEnabled && pipVideo && !pipVideo.disablePictureInPicture)
})

function onActivity() {
  controlsHidden.value = false
  if (hideTimer) clearTimeout(hideTimer)
  if (playing.value) {
    hideTimer = setTimeout(() => {
      controlsHidden.value = true
    }, 3000)
  }
}

function persistSettings() {
  if (typeof window === 'undefined') return
  window.localStorage.setItem(
    PLAYER_SETTINGS_KEY,
    JSON.stringify({
      volume: volume.value,
      muted: muted.value,
      playbackRate: playbackRate.value,
      showRemaining: showRemaining.value,
    }),
  )
}

function applySavedSettings() {
  const v = video.value
  if (!v || typeof window === 'undefined') return
  try {
    const raw = window.localStorage.getItem(PLAYER_SETTINGS_KEY)
    if (!raw) return
    const parsed = JSON.parse(raw) as {
      volume?: number
      muted?: boolean
      playbackRate?: number
      showRemaining?: boolean
    }
    if (typeof parsed.volume === 'number') {
      const nextVolume = Math.max(0, Math.min(1, parsed.volume))
      v.volume = nextVolume
      volume.value = nextVolume
    }
    if (typeof parsed.muted === 'boolean') {
      v.muted = parsed.muted
      muted.value = parsed.muted
    }
    if (typeof parsed.playbackRate === 'number' && speeds.includes(parsed.playbackRate)) {
      v.playbackRate = parsed.playbackRate
      playbackRate.value = parsed.playbackRate
    }
    if (typeof parsed.showRemaining === 'boolean') {
      showRemaining.value = parsed.showRemaining
    }
  } catch {
    // Ignore invalid local settings.
  }
}

function togglePlay() {
  const v = video.value
  if (!v) return
  if (v.paused) {
    void v.play()
    onActivity()
  } else {
    v.pause()
    controlsHidden.value = false
    if (hideTimer) clearTimeout(hideTimer)
  }
}

function onContainerClick() {
  togglePlay()
}

function toggleMute() {
  const v = video.value
  if (!v) return
  v.muted = !v.muted
  muted.value = v.muted
  persistSettings()
}

function onVolumeInput(e: Event) {
  const v = video.value
  if (!v) return
  const val = parseFloat((e.target as HTMLInputElement).value)
  v.volume = val
  volume.value = val
  v.muted = val === 0
  muted.value = val === 0
  persistSettings()
}

function onSpeedChange(e: Event) {
  const v = video.value
  if (!v) return
  const rate = parseFloat((e.target as HTMLSelectElement).value)
  v.playbackRate = rate
  playbackRate.value = rate
  persistSettings()
}

function seek(secs: number) {
  const v = video.value
  if (!v) return
  v.currentTime = Math.max(0, Math.min(v.duration || 0, v.currentTime + secs))
}

function seekTo(pct: number) {
  const v = video.value
  if (!v || !duration.value) return
  v.currentTime = pct * duration.value
}

function changeVolume(delta: number) {
  const v = video.value
  if (!v) return
  const newVol = Math.max(0, Math.min(1, v.volume + delta))
  v.volume = newVol
  volume.value = newVol
  if (newVol > 0) {
    v.muted = false
    muted.value = false
  }
  persistSettings()
}

function changeSpeed(delta: number) {
  const v = video.value
  if (!v) return
  const idx = speeds.indexOf(playbackRate.value)
  const next = Math.max(0, Math.min(speeds.length - 1, idx + delta))
  v.playbackRate = speeds[next]
  playbackRate.value = speeds[next]
  persistSettings()
}

function toggleFullscreen() {
  const el = container.value
  if (!el) return
  if (!document.fullscreenElement) {
    void el.requestFullscreen().then(() => {
      isFullscreen.value = true
    })
  } else {
    void document.exitFullscreen().then(() => {
      isFullscreen.value = false
    })
  }
}

async function togglePictureInPicture() {
  const v = video.value as (HTMLVideoElement & { requestPictureInPicture?: () => Promise<unknown> }) | undefined
  if (!v || !supportsPictureInPicture.value) return
  if (document.pictureInPictureElement === v) {
    await document.exitPictureInPicture()
    return
  }
  if (v.requestPictureInPicture) {
    await v.requestPictureInPicture()
  }
}

function onSeekClick(e: MouseEvent) {
  if (seekDragging) return
  const bar = progressBar.value
  if (!bar) return
  const rect = bar.getBoundingClientRect()
  seekTo((e.clientX - rect.left) / rect.width)
}

function onSeekStart(e: MouseEvent) {
  seekDragging = true
  const bar = progressBar.value
  if (!bar) return
  const rect = bar.getBoundingClientRect()
  seekTo((e.clientX - rect.left) / rect.width)

  const onMove = (ev: MouseEvent) => {
    const r = bar.getBoundingClientRect()
    seekTo(Math.max(0, Math.min(1, (ev.clientX - r.left) / r.width)))
  }
  const onUp = () => {
    seekDragging = false
    document.removeEventListener('mousemove', onMove)
    document.removeEventListener('mouseup', onUp)
  }
  document.addEventListener('mousemove', onMove)
  document.addEventListener('mouseup', onUp)
}

function onKey(e: KeyboardEvent) {
  switch (e.key) {
    case ' ':
    case 'k':
    case 'K':
      e.preventDefault()
      togglePlay()
      break
    case 'ArrowLeft':
      e.preventDefault()
      seek(-5)
      break
    case 'ArrowRight':
      e.preventDefault()
      seek(5)
      break
    case 'j':
    case 'J':
      e.preventDefault()
      seek(-10)
      break
    case 'l':
    case 'L':
      e.preventDefault()
      seek(10)
      break
    case 'ArrowUp':
      e.preventDefault()
      changeVolume(0.1)
      break
    case 'ArrowDown':
      e.preventDefault()
      changeVolume(-0.1)
      break
    case 'm':
    case 'M':
      e.preventDefault()
      toggleMute()
      break
    case 'f':
    case 'F':
      e.preventDefault()
      toggleFullscreen()
      break
    case 'i':
    case 'I':
      if (supportsPictureInPicture.value) {
        e.preventDefault()
        void togglePictureInPicture()
      }
      break
    case '<':
      e.preventDefault()
      changeSpeed(-1)
      break
    case '>':
      e.preventDefault()
      changeSpeed(1)
      break
  }
  onActivity()
}

function onTimeUpdate() {
  const v = video.value
  if (!v) return
  currentTime.value = v.currentTime
  emit('timeupdate', v.currentTime)
}

function onMetadata() {
  const v = video.value
  if (!v) return
  duration.value = v.duration
  applySavedSettings()
  if (!resumeApplied && (props.initialTime ?? 0) > 0) {
    const safeTime = Math.max(0, Math.min(props.initialTime ?? 0, Math.max(0, (v.duration || 0) - 2)))
    if (safeTime > 0) {
      v.currentTime = safeTime
      currentTime.value = safeTime
    }
  }
  resumeApplied = true
}

function onEnded() {
  playing.value = false
  controlsHidden.value = false
  emit('ended')
}

function onError(e: Event) {
  emit('error', e)
}

function formatTime(secs: number): string {
  if (!isFinite(secs) || secs < 0) return '0:00'
  const h = Math.floor(secs / 3600)
  const m = Math.floor((secs % 3600) / 60)
  const s = Math.floor(secs % 60)
  const mm = String(m).padStart(2, '0')
  const ss = String(s).padStart(2, '0')
  return h > 0 ? `${h}:${mm}:${ss}` : `${m}:${ss}`
}

function onFullscreenChange() {
  isFullscreen.value = !!document.fullscreenElement
}

function onEnterPictureInPicture() {
  isPictureInPicture.value = true
}

function onLeavePictureInPicture() {
  isPictureInPicture.value = false
}

onMounted(() => {
  document.addEventListener('fullscreenchange', onFullscreenChange)
  video.value?.addEventListener('enterpictureinpicture', onEnterPictureInPicture)
  video.value?.addEventListener('leavepictureinpicture', onLeavePictureInPicture)
  applySavedSettings()
  container.value?.focus()
})

watch(
  () => props.src,
  () => {
    resumeApplied = false
    playing.value = false
    buffering.value = false
    currentTime.value = 0
    duration.value = 0
  },
)

watch(showRemaining, () => {
  persistSettings()
})

onUnmounted(() => {
  document.removeEventListener('fullscreenchange', onFullscreenChange)
  video.value?.removeEventListener('enterpictureinpicture', onEnterPictureInPicture)
  video.value?.removeEventListener('leavepictureinpicture', onLeavePictureInPicture)
  if (hideTimer) clearTimeout(hideTimer)
})
</script>

<style scoped>
.vp-container {
  position: relative;
  background: #111113;
  width: 100%;
  aspect-ratio: 16 / 9;
  overflow: hidden;
  outline: none;
  cursor: default;
  border-radius: var(--radius-lg, 8px);
}

.vp-container:focus-visible {
  outline: 2px solid var(--focus-ring);
  outline-offset: 2px;
}

.vp-fullscreen {
  border-radius: 0;
}

.vp-video {
  width: 100%;
  height: 100%;
  object-fit: contain;
  display: block;
}

.vp-spinner {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: rgba(255, 255, 255, 0.6);
  pointer-events: none;
}

.vp-spinner-icon {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.vp-controls {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  background: linear-gradient(transparent, rgba(0, 0, 0, 0.85));
  padding: 12px 12px 8px;
  transition: opacity 0.25s;
  opacity: 1;
}

.vp-controls-hidden .vp-controls {
  opacity: 0;
  pointer-events: none;
}

.vp-progress {
  padding: 6px 0;
  cursor: pointer;
}

.vp-progress-track {
  position: relative;
  height: 4px;
  background: rgba(255, 255, 255, 0.2);
  border-radius: 2px;
  overflow: visible;
}

.vp-progress-fill {
  position: absolute;
  top: 0;
  left: 0;
  height: 100%;
  background: #f59e0b;
  border-radius: 2px;
  pointer-events: none;
}

.vp-progress-thumb {
  position: absolute;
  top: 50%;
  transform: translate(-50%, -50%);
  width: 12px;
  height: 12px;
  background: #f59e0b;
  border-radius: 50%;
  pointer-events: none;
  transition: transform 0.1s;
}

.vp-progress:hover .vp-progress-thumb {
  transform: translate(-50%, -50%) scale(1.3);
}

.vp-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-top: 4px;
}

.vp-bar-left,
.vp-bar-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.vp-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: none;
  border: none;
  color: #fff;
  cursor: pointer;
  font-size: 18px;
  padding: 2px 4px;
  line-height: 1;
  transition: color 0.15s;
}

.vp-btn:hover {
  color: #f59e0b;
}

.vp-btn :deep(.app-icon) {
  display: block;
}

.vp-btn:focus-visible,
.vp-slider:focus-visible,
.vp-speed:focus-visible,
.vp-progress:focus-visible,
.vp-time-btn:focus-visible {
  outline: 2px solid var(--focus-ring);
  outline-offset: 2px;
}

.vp-volume {
  display: flex;
  align-items: center;
  gap: 4px;
}

.vp-slider {
  -webkit-appearance: none;
  appearance: none;
  height: 4px;
  border-radius: 2px;
  background: rgba(255, 255, 255, 0.3);
  cursor: pointer;
  outline: none;
}

.vp-slider::-webkit-slider-thumb {
  -webkit-appearance: none;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: #f59e0b;
  cursor: pointer;
}

.vp-volume-slider {
  width: 72px;
}

.vp-time {
  font-size: 12px;
  color: #fff;
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}

.vp-time-btn {
  border: none;
  background: transparent;
  cursor: pointer;
  padding: 0;
}

.vp-speed {
  background: rgba(255, 255, 255, 0.15);
  border: 1px solid rgba(255, 255, 255, 0.2);
  color: #fff;
  font-size: 12px;
  padding: 2px 4px;
  border-radius: 4px;
  cursor: pointer;
}

.vp-speed:focus {
  outline: none;
  border-color: #f59e0b;
}

.vp-speed option {
  background: #222;
  color: #fff;
}

@media (max-width: 760px) {
  .vp-bar {
    flex-direction: column;
    align-items: stretch;
  }

  .vp-bar-left,
  .vp-bar-right {
    justify-content: space-between;
    flex-wrap: wrap;
  }
}
</style>
