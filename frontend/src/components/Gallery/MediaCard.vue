<template>
  <RouterLink
    :to="`/media/${media.id}`"
    class="media-card"
    @mouseenter="onHoverStart"
    @mouseleave="onHoverEnd"
  >
    <div class="media-card__thumb">
      <video
        v-if="showVideoPreview"
        ref="previewVideo"
        class="media-card__preview"
        :src="mediaAssetUrl(media.id, 'file')"
        :poster="thumbError ? undefined : thumbnailUrl"
        muted
        loop
        playsinline
        preload="metadata"
        @loadedmetadata="onPreviewLoaded"
      />
      <img
        v-if="!thumbError && !showVideoPreview"
        :src="thumbnailUrl"
        :alt="media.title"
        loading="lazy"
        @error="onThumbError"
      />
      <div v-if="thumbError && !showVideoPreview" class="media-card__placeholder">
        <span>{{ typeIcon }}</span>
      </div>

      <span class="media-card__type-badge">{{ media.type }}</span>

      <button
        class="media-card__fav"
        :class="{ 'media-card__fav--active': media.favorite }"
        @click.prevent="store.toggleFavorite(media.id)"
      >♥</button>
    </div>

    <div class="media-card__info">
      <p class="media-card__title">{{ media.title }}</p>
      <div class="media-card__tags">
        <TagBadge v-for="tag in media.tags?.slice(0, 3)" :key="tag.id" :tag="tag" />
      </div>
    </div>
  </RouterLink>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { mediaAssetUrl, type Media } from '@/api/mediaApi'
import TagBadge from '@/components/Tags/TagBadge.vue'
import { useMediaStore } from '@/stores/mediaStore'

const props = defineProps<{ media: Media }>()
const store = useMediaStore()
const thumbError = ref(false)
const hovering = ref(false)
const previewVideo = ref<HTMLVideoElement | null>(null)

const showVideoPreview = computed(() => props.media.type === 'video' && hovering.value)
const thumbnailUrl = computed(() => mediaAssetUrl(props.media.id, 'thumbnail', props.media.updated_at))

function onThumbError() {
  thumbError.value = true
}

function onHoverStart() {
  if (props.media.type !== 'video') return
  hovering.value = true
  requestAnimationFrame(() => {
    previewVideo.value?.play().catch(() => {})
  })
}

function onHoverEnd() {
  if (props.media.type !== 'video') return
  const video = previewVideo.value
  if (video) {
    video.pause()
    video.currentTime = 0
  }
  hovering.value = false
}

function onPreviewLoaded() {
  const video = previewVideo.value
  if (!video) return

  const duration = Number.isFinite(video.duration) ? video.duration : 0
  if (duration <= 0) return

  const target = Math.min(30, Math.max(20, duration * 0.2))
  video.currentTime = Math.min(target, Math.max(0, duration - 1))
}

const typeIcon = computed(() => {
  const icons: Record<string, string> = {
    video: '🎬', image: '🖼️', manga: '📖', comic: '📕', doujinshi: '📗',
  }
  return icons[props.media.type] ?? '📄'
})
</script>

<style scoped>
.media-card {
  display: flex;
  flex-direction: column;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  overflow: hidden;
  cursor: pointer;
  transition: border-color 0.15s, transform 0.15s;
}

.media-card:hover {
  border-color: var(--accent);
  transform: translateY(-2px);
}

.media-card__thumb {
  position: relative;
  aspect-ratio: 3 / 4;
  background: var(--bg-hover);
  overflow: hidden;
}

.media-card__thumb img,
.media-card__preview {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.media-card__placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  font-size: 40px;
}

.media-card__type-badge {
  position: absolute;
  bottom: 6px;
  left: 6px;
  background: rgba(0,0,0,0.7);
  color: #fff;
  font-size: 10px;
  padding: 2px 6px;
  border-radius: 4px;
  text-transform: uppercase;
}

.media-card__fav {
  position: absolute;
  top: 6px;
  right: 6px;
  background: rgba(0,0,0,0.5);
  border: none;
  cursor: pointer;
  font-size: 16px;
  color: var(--text-muted);
  width: 28px;
  height: 28px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: color 0.15s;
}

.media-card__fav--active,
.media-card__fav:hover { color: var(--danger); }

.media-card__info {
  padding: 10px 12px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.media-card__title {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.media-card__tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}
</style>
