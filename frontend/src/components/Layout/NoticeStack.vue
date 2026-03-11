<template>
  <div v-if="notices.length" class="notice-stack" aria-live="polite" aria-atomic="true">
    <div
      v-for="notice in notices"
      :key="notice.id"
      :class="['notice-card', `notice-card--${notice.type}`]"
    >
      <span class="notice-message">{{ notice.message }}</span>
      <button type="button" class="notice-close" aria-label="Dismiss notice" @click="removeNotice(notice.id)">
        <AppIcon name="close" :size="12" />
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import AppIcon from '@/components/Layout/AppIcon.vue'
import { useNoticeStore } from '@/stores/noticeStore'

const { notices, removeNotice } = useNoticeStore()
</script>

<style scoped>
.notice-stack {
  position: fixed;
  right: 16px;
  bottom: 16px;
  z-index: 1200;
  display: flex;
  flex-direction: column;
  gap: 10px;
  width: min(360px, calc(100vw - 24px));
}

.notice-card {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 14px;
  border-radius: 14px;
  border: 1px solid var(--border);
  background: color-mix(in srgb, var(--bg-card) 96%, black);
  box-shadow: 0 18px 40px rgba(0, 0, 0, 0.28);
}

.notice-card--success {
  border-color: rgba(34, 197, 94, 0.28);
}

.notice-card--error {
  border-color: rgba(239, 68, 68, 0.28);
}

.notice-card--info {
  border-color: rgba(59, 130, 246, 0.28);
}

.notice-message {
  font-size: 13px;
  line-height: 1.4;
  color: var(--text-primary);
}

.notice-close {
  appearance: none;
  border: none;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  padding: 0;
  line-height: 1;
}

@media (max-width: 900px) {
  .notice-stack {
    right: 12px;
    left: 12px;
    bottom: 12px;
    width: auto;
  }
}
</style>
