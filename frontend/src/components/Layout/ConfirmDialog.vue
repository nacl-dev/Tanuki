<template>
  <ModalShell :title="title" :description="message" size="sm" @close="$emit('cancel')">
    <template #actions>
      <button type="button" class="btn btn-ghost" @click="$emit('cancel')">
        {{ cancelLabel }}
      </button>
      <button type="button" class="btn btn-danger" @click="$emit('confirm')">
        {{ confirmLabel }}
      </button>
    </template>
  </ModalShell>
</template>

<script setup lang="ts">
import ModalShell from './ModalShell.vue'

withDefaults(defineProps<{
  title: string
  message: string
  cancelLabel?: string
  confirmLabel?: string
}>(), {
  cancelLabel: 'Cancel',
  confirmLabel: 'Confirm',
})

defineEmits<{
  cancel: []
  confirm: []
}>()
</script>
