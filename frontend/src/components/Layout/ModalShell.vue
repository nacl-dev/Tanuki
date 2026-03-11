<template>
  <div class="dialog-overlay" @click.self="emit('close')">
    <section
      ref="dialogRef"
      class="dialog-card"
      :class="sizeClass"
      role="dialog"
      aria-modal="true"
      :aria-labelledby="titleId"
      :aria-describedby="description ? descriptionId : undefined"
      tabindex="-1"
    >
      <header class="dialog-head">
        <div class="dialog-copy">
          <h3 :id="titleId">{{ title }}</h3>
          <p v-if="description" :id="descriptionId">{{ description }}</p>
        </div>
        <button
          v-if="showClose"
          type="button"
          class="dialog-close"
          :aria-label="closeLabel"
          @click="emit('close')"
        >
          <AppIcon name="close" :size="16" />
        </button>
      </header>

      <div class="dialog-body">
        <slot />
      </div>

      <footer v-if="$slots.actions" class="dialog-actions">
        <slot name="actions" />
      </footer>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import AppIcon from './AppIcon.vue'

const props = withDefaults(defineProps<{
  title: string
  description?: string
  closeLabel?: string
  size?: 'sm' | 'md' | 'lg'
  showClose?: boolean
}>(), {
  description: '',
  closeLabel: 'Close dialog',
  size: 'md',
  showClose: true,
})

const emit = defineEmits<{
  close: []
}>()

const dialogRef = ref<HTMLElement | null>(null)
const titleId = `dialog-title-${Math.random().toString(36).slice(2, 8)}`
const descriptionId = `dialog-description-${Math.random().toString(36).slice(2, 8)}`
const previouslyFocused = ref<HTMLElement | null>(null)

const sizeClass = computed(() => `dialog-card--${props.size}`)

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
.dialog-overlay {
  position: fixed;
  inset: 0;
  z-index: 1300;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
  background: rgba(5, 8, 14, 0.7);
  backdrop-filter: blur(10px);
}

.dialog-card {
  width: min(560px, 100%);
  max-height: min(88vh, 920px);
  overflow-y: auto;
  padding: 20px;
  border: 1px solid color-mix(in srgb, var(--border) 75%, rgba(255, 255, 255, 0.1));
  border-radius: 18px;
  background:
    linear-gradient(180deg, rgba(255,255,255,0.03), rgba(255,255,255,0)),
    color-mix(in srgb, var(--bg-card) 96%, black);
  box-shadow: 0 28px 60px rgba(0, 0, 0, 0.42);
}

.dialog-card--sm {
  width: min(460px, 100%);
}

.dialog-card--lg {
  width: min(760px, 100%);
}

.dialog-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.dialog-copy {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.dialog-copy h3 {
  margin: 0;
  font-size: 18px;
}

.dialog-copy p {
  margin: 0;
  color: var(--text-muted);
  font-size: 13px;
  line-height: 1.5;
}

.dialog-close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border-radius: 12px;
  border: 1px solid var(--border);
  background: var(--bg-surface);
  color: var(--text-secondary);
  cursor: pointer;
}

.dialog-close:focus-visible {
  outline: 2px solid var(--focus-ring);
  outline-offset: 2px;
}

.dialog-body {
  display: flex;
  flex-direction: column;
  gap: 14px;
  margin-top: 16px;
}

.dialog-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  flex-wrap: wrap;
  margin-top: 18px;
}
</style>
