<template>
  <svg
    class="app-icon"
    :class="{ 'app-icon--filled': filled }"
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    aria-hidden="true"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
  >
    <template v-if="filled">
      <path v-for="(path, index) in icon.fill" :key="index" :d="path" fill="currentColor" />
    </template>
    <template v-else>
      <path
        v-for="(path, index) in icon.stroke"
        :key="index"
        :d="path"
        stroke="currentColor"
        stroke-width="1.75"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
    </template>
  </svg>
</template>

<script setup lang="ts">
import { computed } from 'vue'

type IconName =
  | 'menu'
  | 'close'
  | 'search'
  | 'library'
  | 'download'
  | 'collection'
  | 'tag'
  | 'settings'
  | 'logout'
  | 'heart'
  | 'video'
  | 'image'
  | 'book'
  | 'link'
  | 'check'
  | 'spark'
  | 'play'
  | 'pause'
  | 'refresh'
  | 'trash'
  | 'volumeOff'
  | 'volumeLow'
  | 'volumeHigh'
  | 'expand'
  | 'collapse'
  | 'rewind'
  | 'forward'
  | 'pip'
  | 'question'

const props = withDefaults(defineProps<{
  name: IconName
  size?: number | string
  filled?: boolean
}>(), {
  size: 18,
  filled: false,
})

const icons: Record<IconName, { stroke?: string[]; fill?: string[] }> = {
  menu: { stroke: ['M4 7H20', 'M4 12H20', 'M4 17H20'] },
  close: { stroke: ['M6 6L18 18', 'M18 6L6 18'] },
  search: { stroke: ['M11 18A7 7 0 1 0 11 4A7 7 0 0 0 11 18Z', 'M20 20L16.65 16.65'] },
  library: { stroke: ['M6 5.5A2.5 2.5 0 0 1 8.5 3H19V19H8.5A2.5 2.5 0 0 0 6 21V5.5ZM6 5.5A2.5 2.5 0 0 0 3.5 8V19H19'] },
  download: { stroke: ['M12 4V15', 'M7.5 10.5L12 15L16.5 10.5', 'M5 20H19'] },
  collection: { stroke: ['M4 7A2 2 0 0 1 6 5H10A2 2 0 0 1 12 7V11A2 2 0 0 1 10 13H6A2 2 0 0 1 4 11V7Z', 'M12 7A2 2 0 0 1 14 5H18A2 2 0 0 1 20 7V11A2 2 0 0 1 18 13H14A2 2 0 0 1 12 11V7Z', 'M4 15A2 2 0 0 1 6 13H10A2 2 0 0 1 12 15V19A2 2 0 0 1 10 21H6A2 2 0 0 1 4 19V15Z', 'M12 15A2 2 0 0 1 14 13H18A2 2 0 0 1 20 15V19A2 2 0 0 1 18 21H14A2 2 0 0 1 12 19V15Z'] },
  tag: { stroke: ['M20.59 13.41L11 3.83H4V10.83L13.59 20.42A2 2 0 0 0 16.41 20.42L20.59 16.24A2 2 0 0 0 20.59 13.41Z', 'M7.5 7.5H7.51'] },
  settings: { stroke: ['M12 8.75A3.25 3.25 0 1 0 12 15.25A3.25 3.25 0 0 0 12 8.75Z', 'M19.4 15A1 1 0 0 0 19.6 16.1L20 16.82A1 1 0 0 1 19.63 18.18L18.18 19.63A1 1 0 0 1 16.82 20L16.1 19.6A1 1 0 0 0 15 19.4L14.2 19.73A1 1 0 0 0 13.56 20.64L13.48 21.5A1 1 0 0 1 12.48 22.4H11.52A1 1 0 0 1 10.52 21.5L10.44 20.64A1 1 0 0 0 9.8 19.73L9 19.4A1 1 0 0 0 7.9 19.6L7.18 20A1 1 0 0 1 5.82 19.63L4.37 18.18A1 1 0 0 1 4 16.82L4.4 16.1A1 1 0 0 0 4.2 15L3.87 14.2A1 1 0 0 0 2.96 13.56L2.1 13.48A1 1 0 0 1 1.2 12.48V11.52A1 1 0 0 1 2.1 10.52L2.96 10.44A1 1 0 0 0 3.87 9.8L4.2 9A1 1 0 0 0 4.4 7.9L4 7.18A1 1 0 0 1 4.37 5.82L5.82 4.37A1 1 0 0 1 7.18 4L7.9 4.4A1 1 0 0 0 9 4.2L9.8 3.87A1 1 0 0 0 10.44 2.96L10.52 2.1A1 1 0 0 1 11.52 1.2H12.48A1 1 0 0 1 13.48 2.1L13.56 2.96A1 1 0 0 0 14.2 3.87L15 4.2A1 1 0 0 0 16.1 4.4L16.82 4A1 1 0 0 1 18.18 4.37L19.63 5.82A1 1 0 0 1 20 7.18L19.6 7.9A1 1 0 0 0 19.8 9L20.13 9.8A1 1 0 0 0 21.04 10.44L21.9 10.52A1 1 0 0 1 22.8 11.52V12.48A1 1 0 0 1 21.9 13.48L21.04 13.56A1 1 0 0 0 20.13 14.2L19.4 15Z'] },
  logout: { stroke: ['M10 17L15 12L10 7', 'M15 12H4', 'M14 4H18A2 2 0 0 1 20 6V18A2 2 0 0 1 18 20H14'] },
  heart: { stroke: ['M12 20.5L10.55 19.18C5.4 14.53 2 11.46 2 7.7C2 4.63 4.42 2.2 7.5 2.2C9.24 2.2 10.91 3.01 12 4.29C13.09 3.01 14.76 2.2 16.5 2.2C19.58 2.2 22 4.63 22 7.7C22 11.46 18.6 14.53 13.45 19.19L12 20.5Z'], fill: ['M12 20.5L10.55 19.18C5.4 14.53 2 11.46 2 7.7C2 4.63 4.42 2.2 7.5 2.2C9.24 2.2 10.91 3.01 12 4.29C13.09 3.01 14.76 2.2 16.5 2.2C19.58 2.2 22 4.63 22 7.7C22 11.46 18.6 14.53 13.45 19.19L12 20.5Z'] },
  video: { stroke: ['M3 7A2 2 0 0 1 5 5H14A2 2 0 0 1 16 7V17A2 2 0 0 1 14 19H5A2 2 0 0 1 3 17V7Z', 'M16 10L21 7V17L16 14V10Z'] },
  image: { stroke: ['M4 6A2 2 0 0 1 6 4H18A2 2 0 0 1 20 6V18A2 2 0 0 1 18 20H6A2 2 0 0 1 4 18V6Z', 'M8.5 10A1.5 1.5 0 1 0 8.5 7A1.5 1.5 0 0 0 8.5 10Z', 'M5 17L10.5 11.5L14 15L16.5 12.5L19 15'] },
  book: { stroke: ['M6 5.5A2.5 2.5 0 0 1 8.5 3H18V19H8.5A2.5 2.5 0 0 0 6 21V5.5Z', 'M6 5.5A2.5 2.5 0 0 0 3.5 8V19H18', 'M9 7H15'] },
  link: { stroke: ['M10 14L14 10', 'M7.5 16.5L5.5 18.5A3 3 0 0 1 1.26 14.26L3.26 12.26A3 3 0 0 1 7.5 12', 'M16.5 7.5L18.5 5.5A3 3 0 0 1 22.74 9.74L20.74 11.74A3 3 0 0 1 16.5 12'] },
  check: { stroke: ['M5 12.5L9.5 17L19 7.5'] },
  spark: { stroke: ['M12 3L13.9 8.1L19 10L13.9 11.9L12 17L10.1 11.9L5 10L10.1 8.1L12 3Z'] },
  play: { stroke: ['M8 6L18 12L8 18V6Z'] },
  pause: { stroke: ['M8 5V19', 'M16 5V19'] },
  refresh: { stroke: ['M20 11A8 8 0 0 0 6.34 5.34L4 7.67', 'M4 4V8H8', 'M4 13A8 8 0 0 0 17.66 18.66L20 16.33', 'M16 16H20V20'] },
  trash: { stroke: ['M4 7H20', 'M9 7V4H15V7', 'M7 7L8 19H16L17 7', 'M10 11V16', 'M14 11V16'] },
  volumeOff: { stroke: ['M5 10H8L12 6V18L8 14H5V10Z', 'M17 9L21 15', 'M21 9L17 15'] },
  volumeLow: { stroke: ['M5 10H8L12 6V18L8 14H5V10Z', 'M16 9.5A4 4 0 0 1 16 14.5'] },
  volumeHigh: { stroke: ['M5 10H8L12 6V18L8 14H5V10Z', 'M16 8A6 6 0 0 1 16 16', 'M18.5 5.5A9 9 0 0 1 18.5 18.5'] },
  expand: { stroke: ['M9 4H4V9', 'M15 4H20V9', 'M4 15V20H9', 'M20 15V20H15', 'M4 9L10 3', 'M14 3L20 9', 'M4 15L10 21', 'M14 21L20 15'] },
  collapse: { stroke: ['M10 9H4V3', 'M14 9H20V3', 'M4 21V15H10', 'M20 21V15H14', 'M10 9L4 3', 'M14 9L20 3', 'M4 21L10 15', 'M14 15L20 21'] },
  rewind: { stroke: ['M11 7L5 12L11 17V7Z', 'M19 7L13 12L19 17V7Z'] },
  forward: { stroke: ['M5 7L11 12L5 17V7Z', 'M13 7L19 12L13 17V7Z'] },
  pip: { stroke: ['M4 6A2 2 0 0 1 6 4H18A2 2 0 0 1 20 6V18A2 2 0 0 1 18 20H6A2 2 0 0 1 4 18V6Z', 'M12 12H19V18H12V12Z'] },
  question: { stroke: ['M9.1 9A3 3 0 1 1 14.9 10.2C13.95 10.83 13 11.43 13 12.8V13.5', 'M12 18H12.01'] },
}

const icon = computed(() => icons[props.name] ?? icons.book)
</script>

<style scoped>
.app-icon {
  display: inline-block;
  flex-shrink: 0;
}
</style>
