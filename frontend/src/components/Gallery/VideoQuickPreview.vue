<template>
  <Teleport to="body">
    <div class="video-preview" @click.self="emit('close')">
      <section
        ref="dialogRef"
        class="video-preview__panel"
        role="dialog"
        aria-modal="true"
        :aria-labelledby="titleId"
        tabindex="-1"
      >
        <div v-if="poster" class="video-preview__backdrop" :style="{ backgroundImage: `url(${poster})` }" />

        <header class="video-preview__header">
          <div class="video-preview__copy">
            <div class="video-preview__eyebrow">
              <span class="video-preview__type">{{ media.type }}</span>
              <span v-if="media.work_title" class="video-preview__work">{{ media.work_title }}</span>
              <span v-if="resumeLabel" class="video-preview__resume">{{ resumeLabel }}</span>
            </div>
            <h2 :id="titleId">{{ media.title }}</h2>
          </div>

          <div class="video-preview__header-actions">
            <button type="button" class="btn btn-secondary" @click="emit('open-details')">Open details</button>
            <button type="button" class="video-preview__close" aria-label="Close preview" @click="emit('close')">
              <AppIcon name="close" :size="18" />
            </button>
          </div>
        </header>

        <div class="video-preview__body">
          <div class="video-preview__stage">
            <VideoPlayer
              :src="mediaAssetUrl(media.id, 'file')"
              :poster="poster"
              :initial-time="media.read_progress || 0"
            />
          </div>

          <div v-if="hasMeta" class="video-preview__meta">
            <div v-if="media.collections?.length" class="video-preview__meta-row">
              <span class="video-preview__label">Collections</span>
              <div class="video-preview__chips">
                <span v-for="collection in media.collections.slice(0, 6)" :key="collection.id" class="video-preview__chip">
                  {{ collection.name }}
                </span>
              </div>
            </div>

            <div v-if="media.tags?.length" class="video-preview__meta-row">
              <span class="video-preview__label">Tags</span>
              <div class="video-preview__chips">
                <span v-for="tag in media.tags.slice(0, 10)" :key="tag.id" class="video-preview__chip video-preview__chip--muted">
                  {{ tag.category !== 'general' ? `${tag.category}:` : '' }}{{ tag.name }}
                </span>
              </div>
            </div>
          </div>
        </div>
      </section>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import AppIcon from '@/components/Layout/AppIcon.vue'
import VideoPlayer from '@/components/Player/VideoPlayer.vue'
import { mediaAssetUrl, type Media } from '@/api/mediaApi'

const props = defineProps<{
  media: Media
  poster?: string
}>()

const emit = defineEmits<{
  close: []
  'open-details': []
}>()

const dialogRef = ref<HTMLElement | null>(null)
const titleId = `video-preview-title-${Math.random().toString(36).slice(2, 8)}`
const previouslyFocused = ref<HTMLElement | null>(null)
const hasMeta = computed(() => Boolean(props.media.collections?.length || props.media.tags?.length))

const resumeLabel = computed(() => {
  if (!props.media.read_progress || props.media.read_progress <= 0) {
    return ''
  }
  return `Resume ${formatTime(props.media.read_progress)}`
})

function focusableElements() {
  if (!dialogRef.value) return [] as HTMLElement[]
  return Array.from(
    dialogRef.value.querySelectorAll<HTMLElement>(
      'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
    ),
  ).filter((element) => !element.hasAttribute('aria-hidden'))
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    event.preventDefault()
    emit('close')
    return
  }

  if (event.key !== 'Tab') return

  const elements = focusableElements()
  if (!elements.length) {
    event.preventDefault()
    dialogRef.value?.focus()
    return
  }

  const first = elements[0]
  const last = elements[elements.length - 1]
  const active = document.activeElement as HTMLElement | null

  if (event.shiftKey && (active === first || active === dialogRef.value)) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && active === last) {
    event.preventDefault()
    first.focus()
  }
}

function formatTime(secs: number): string {
  if (!isFinite(secs) || secs < 0) return '0:00'
  const hours = Math.floor(secs / 3600)
  const minutes = Math.floor((secs % 3600) / 60)
  const seconds = Math.floor(secs % 60)
  const mm = String(minutes).padStart(2, '0')
  const ss = String(seconds).padStart(2, '0')
  return hours > 0 ? `${hours}:${mm}:${ss}` : `${minutes}:${ss}`
}

onMounted(() => {
  previouslyFocused.value = document.activeElement as HTMLElement | null
  document.body.style.overflow = 'hidden'
  const first = focusableElements()[0]
  ;(first ?? dialogRef.value)?.focus()
  document.addEventListener('keydown', onKeydown)
})

onUnmounted(() => {
  document.body.style.overflow = ''
  document.removeEventListener('keydown', onKeydown)
  previouslyFocused.value?.focus?.()
})
</script>

<style scoped>
.video-preview {
  position: fixed;
  inset: 0;
  z-index: 1450;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  background:
    radial-gradient(circle at top, rgba(245, 158, 11, 0.16), transparent 34%),
    rgba(4, 7, 12, 0.82);
  backdrop-filter: blur(16px);
}

.video-preview__panel {
  position: relative;
  width: min(960px, 100%);
  max-height: min(92vh, 860px);
  overflow: hidden;
  border-radius: 22px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  background:
    linear-gradient(180deg, rgba(255,255,255,0.04), rgba(255,255,255,0)),
    color-mix(in srgb, var(--bg-card) 94%, black);
  box-shadow: 0 30px 80px rgba(0, 0, 0, 0.48);
}

.video-preview__panel:focus-visible {
  outline: 2px solid var(--focus-ring);
  outline-offset: 2px;
}

.video-preview__backdrop {
  position: absolute;
  inset: 0;
  background-position: center;
  background-size: cover;
  opacity: 0.16;
  filter: blur(24px) saturate(1.1);
  transform: scale(1.08);
  pointer-events: none;
}

.video-preview__header,
.video-preview__body {
  position: relative;
  z-index: 1;
}

.video-preview__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 18px 20px 0;
}

.video-preview__copy {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-width: 0;
}

.video-preview__eyebrow {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.video-preview__type,
.video-preview__work,
.video-preview__resume {
  display: inline-flex;
  align-items: center;
  min-height: 24px;
  padding: 0 10px;
  border-radius: 999px;
  font-size: 11px;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.video-preview__type {
  background: rgba(245, 158, 11, 0.16);
  color: #f6c268;
}

.video-preview__work,
.video-preview__resume {
  background: rgba(255, 255, 255, 0.06);
  color: var(--text-secondary);
}

.video-preview__copy h2 {
  margin: 0;
  font-size: clamp(18px, 2.2vw, 26px);
  line-height: 1.15;
}

.video-preview__header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

.video-preview__close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 42px;
  height: 42px;
  border-radius: 14px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(12, 18, 27, 0.74);
  color: #fff;
  cursor: pointer;
}

.video-preview__body {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 14px 20px 20px;
  overflow: hidden;
}

.video-preview__stage {
  min-width: 0;
  flex: 0 0 auto;
}

.video-preview__stage :deep(.vp-container) {
  aspect-ratio: 16 / 9;
  border-radius: 18px;
  background: #05070b;
  max-height: min(68vh, 640px);
}

.video-preview__meta {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-width: 0;
}

.video-preview__meta-row {
  display: grid;
  grid-template-columns: 84px minmax(0, 1fr);
  gap: 10px;
  align-items: start;
  padding: 12px 14px;
  border-radius: 16px;
  border: 1px solid rgba(255, 255, 255, 0.06);
  background: rgba(11, 17, 26, 0.48);
}

.video-preview__label {
  font-size: 11px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--text-muted);
  padding-top: 4px;
}

.video-preview__chips {
  display: flex;
  flex-wrap: wrap;
  gap: 7px;
}

.video-preview__chip {
  display: inline-flex;
  align-items: center;
  min-height: 26px;
  padding: 0 10px;
  border-radius: 999px;
  background: rgba(245, 158, 11, 0.1);
  border: 1px solid rgba(245, 158, 11, 0.16);
  color: #f4c06a;
  font-size: 11px;
}

.video-preview__chip--muted {
  background: rgba(255, 255, 255, 0.05);
  border-color: rgba(255, 255, 255, 0.08);
  color: var(--text-secondary);
}

@media (max-width: 960px) {
  .video-preview {
    padding: 12px;
  }

  .video-preview__panel {
    max-height: 94vh;
    border-radius: 18px;
  }

  .video-preview__header {
    flex-direction: column;
    align-items: stretch;
  }

  .video-preview__header-actions {
    justify-content: space-between;
  }

  .video-preview__meta-row {
    grid-template-columns: 1fr;
    gap: 8px;
  }

  .video-preview__label {
    padding-top: 0;
  }

  .video-preview__stage :deep(.vp-container) {
    max-height: min(62vh, 520px);
  }
}

@media (max-width: 640px) {
  .video-preview__header {
    padding: 16px 16px 0;
  }

  .video-preview__body {
    padding: 12px 16px 16px;
  }

  .video-preview__header-actions .btn {
    flex: 1 1 100%;
    justify-content: center;
  }
}
</style>
