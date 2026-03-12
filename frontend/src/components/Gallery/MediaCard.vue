<template>
  <article :class="['media-card', { 'media-card--compact': compact }]" @mouseenter="onHoverStart" @mouseleave="onHoverEnd">
    <RouterLink :to="`/media/${media.id}`" class="media-card__link">
      <div class="media-card__thumb" :class="{ 'media-card__thumb--hover': showHoverPreview }">
        <img
          v-if="showThumbnail"
          :src="thumbnailUrl"
          :alt="media.title"
          loading="lazy"
          class="media-card__image"
          @error="onThumbError"
        />
        <div v-else-if="showPlaceholder" class="media-card__placeholder">
          <AppIcon :name="typeIcon" :size="30" />
        </div>

        <span class="media-card__type-badge">{{ media.type }}</span>
      </div>

      <div class="media-card__info">
        <p class="media-card__title">{{ media.title }}</p>
        <div v-if="media.collections?.length" class="media-card__collections">
          <span
            v-for="collection in media.collections.slice(0, 2)"
            :key="collection.id"
            class="media-card__collection-chip"
          >
            {{ collection.name }}
          </span>
          <span
            v-if="media.collections.length > 2"
            class="media-card__collection-chip media-card__collection-chip--muted"
          >
            +{{ media.collections.length - 2 }}
          </span>
        </div>
        <div v-if="showTags" class="media-card__tags">
          <TagBadge v-for="tag in media.tags?.slice(0, 3)" :key="tag.id" :tag="tag" />
        </div>
      </div>
    </RouterLink>

    <button
      v-if="canQuickPreview"
      type="button"
      :class="['media-card__preview-trigger', { 'media-card__preview-trigger--active': showHoverPreview }]"
      :aria-label="`Quick preview ${media.title}`"
      @click.stop="openPreview"
    >
      <AppIcon name="play" :size="14" />
      <span class="media-card__preview-label">Preview</span>
    </button>

    <button
      type="button"
      class="media-card__fav"
      :class="{ 'media-card__fav--active': media.favorite }"
      :aria-label="media.favorite ? `Remove ${media.title} from favorites` : `Add ${media.title} to favorites`"
      :aria-pressed="media.favorite"
      @click="store.toggleFavorite(media.id)"
    >
      <AppIcon name="heart" :size="15" :filled="media.favorite" />
      <span class="media-card__fav-label">{{ media.favorite ? 'Saved' : 'Save' }}</span>
    </button>

    <VideoQuickPreview
      v-if="previewOpen"
      :media="media"
      :poster="posterUrl"
      @close="closePreview"
      @open-details="openDetails"
    />
  </article>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import AppIcon from '@/components/Layout/AppIcon.vue'
import { mediaAssetUrl, type Media } from '@/api/mediaApi'
import TagBadge from '@/components/Tags/TagBadge.vue'
import { useMediaStore } from '@/stores/mediaStore'
import VideoQuickPreview from '@/components/Gallery/VideoQuickPreview.vue'

const props = withDefaults(defineProps<{
  media: Media
  showTags?: boolean
  compact?: boolean
}>(), {
  showTags: true,
  compact: false,
})
const store = useMediaStore()
const router = useRouter()
const thumbError = ref(false)
const hovering = ref(false)
const previewOpen = ref(false)

const canQuickPreview = computed(() => props.media.type === 'video')
const showHoverPreview = computed(() => canQuickPreview.value && hovering.value)
const thumbnailUrl = computed(() => {
  if (!props.media.thumbnail_path) return ''
  return mediaAssetUrl(props.media.id, 'thumbnail', props.media.updated_at)
})
const posterUrl = computed(() => (thumbError.value ? undefined : thumbnailUrl.value))
const showThumbnail = computed(() => Boolean(thumbnailUrl.value) && !thumbError.value)
const showPlaceholder = computed(() => props.media.type !== 'video' && !showThumbnail.value)

function onThumbError() {
  thumbError.value = true
}

function onHoverStart() {
  if (!canQuickPreview.value) return
  hovering.value = true
}

function onHoverEnd() {
  if (!canQuickPreview.value) return
  hovering.value = false
}

function openPreview() {
  previewOpen.value = true
}

function closePreview() {
  previewOpen.value = false
}

function openDetails() {
  closePreview()
  void router.push({ name: 'media-detail', params: { id: props.media.id } })
}

const typeIcon = computed(() => (props.media.type === 'video' ? 'video' : props.media.type === 'image' ? 'image' : 'book'))
</script>

<style scoped>
.media-card {
  position: relative;
  display: flex;
  flex-direction: column;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  overflow: hidden;
  cursor: pointer;
  transition: border-color 0.15s, transform 0.15s;
}

.media-card:hover,
.media-card:focus-within {
  border-color: var(--accent);
  transform: translateY(-2px);
}

.media-card__link {
  display: flex;
  flex-direction: column;
}

.media-card__thumb {
  position: relative;
  aspect-ratio: 3 / 4;
  background: var(--bg-hover);
  overflow: hidden;
}

.media-card__thumb--hover .media-card__image {
  transform: scale(1.04);
  filter: saturate(1.05);
}

.media-card__image {
  width: 100%;
  height: 100%;
  object-fit: cover;
  transition: transform 0.22s ease, filter 0.22s ease;
}

.media-card__preview-trigger {
  position: absolute;
  top: 6px;
  left: 6px;
  z-index: 3;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 30px;
  padding: 0 10px;
  border: 1px solid rgba(255,255,255,0.08);
  cursor: pointer;
  border-radius: 999px;
  background: rgba(16, 24, 39, 0.72);
  color: #f8fafc;
  font-size: 11px;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  transition: background 0.15s, border-color 0.15s, transform 0.15s;
}

.media-card__preview-trigger:hover,
.media-card__preview-trigger--active {
  background: rgba(16, 24, 39, 0.88);
  border-color: rgba(245, 158, 11, 0.32);
  transform: translateY(-1px);
}

.media-card__preview-trigger:focus-visible {
  outline: 2px solid var(--focus-ring);
  outline-offset: 2px;
}

.media-card__preview-label {
  line-height: 1;
}

.media-card__placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--text-muted);
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
  gap: 4px;
  background: rgba(0,0,0,0.62);
  border: 1px solid rgba(255,255,255,0.08);
  cursor: pointer;
  font-size: 12px;
  color: #ffffff;
  min-width: 30px;
  height: 30px;
  padding: 0 10px;
  border-radius: 999px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: color 0.15s, background 0.15s, border-color 0.15s;
}

.media-card__fav--active,
.media-card__fav:hover {
  color: var(--danger);
  background: rgba(0, 0, 0, 0.68);
}

.media-card__fav:focus-visible {
  outline: 2px solid var(--focus-ring);
  outline-offset: 2px;
}

.media-card__fav-label {
  font-size: 10px;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

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

.media-card__collections {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.media-card__collection-chip {
  display: inline-flex;
  align-items: center;
  max-width: 100%;
  padding: 3px 8px;
  border-radius: 999px;
  border: 1px solid rgba(245, 158, 11, 0.18);
  background: rgba(245, 158, 11, 0.1);
  color: #f4c06a;
  font-size: 10px;
  line-height: 1.2;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.media-card__collection-chip--muted {
  border-color: var(--border);
  background: rgba(255,255,255,0.04);
  color: var(--text-muted);
}

.media-card--compact .media-card__info {
  padding: 8px 10px;
  gap: 4px;
}

.media-card--compact .media-card__title {
  font-size: 12px;
}

.media-card--compact .media-card__collections {
  gap: 3px;
}

.media-card--compact .media-card__collection-chip {
  padding: 2px 7px;
  font-size: 9px;
}

@media (max-width: 640px) {
  .media-card__info {
    padding: 9px 10px;
    gap: 4px;
  }

  .media-card__title {
    font-size: 12px;
  }

  .media-card__preview-label,
  .media-card__fav-label {
    display: none;
  }

  .media-card__preview-trigger,
  .media-card__fav {
    min-width: 30px;
    padding: 0 8px;
  }
}
</style>
