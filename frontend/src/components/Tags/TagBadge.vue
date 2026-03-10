<template>
  <router-link
    :to="{ path: '/', query: { tag: tag.name } }"
    :class="['badge', `badge--${tag.category}`]"
  >
    {{ tag.category !== 'general' ? `${tag.category}:` : '' }}{{ tag.name }}
  </router-link>
</template>

<script setup lang="ts">
import type { Tag } from '@/api/mediaApi'
defineProps<{ tag: Tag }>()
</script>

<style scoped>
.badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 500;
  line-height: 1.6;
  white-space: nowrap;
  text-decoration: none;
  cursor: pointer;
  transition: opacity 0.15s;
}

.badge:hover { opacity: 0.8; }

.badge--general   { background: rgba(96,96,112,0.25);  color: #a0a0b8; }
.badge--artist    { background: rgba(239,68,68,0.2);   color: #f87171; }
.badge--character { background: rgba(34,197,94,0.2);   color: #4ade80; }
.badge--parody    { background: rgba(245,158,11,0.2);  color: #fbbf24; }
.badge--genre     { background: rgba(59,130,246,0.2);  color: #60a5fa; }
.badge--meta      { background: rgba(168,85,247,0.2);  color: #c084fc; }
</style>
