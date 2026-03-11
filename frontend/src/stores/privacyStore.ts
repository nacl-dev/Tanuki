import { defineStore } from 'pinia'
import { ref } from 'vue'

const storageKey = 'tanuki:privacy-blur-enabled'

export const usePrivacyStore = defineStore('privacy', () => {
  const enabled = ref(false)
  const hydrated = ref(false)

  function hydrate() {
    if (hydrated.value || typeof window === 'undefined') return
    enabled.value = window.localStorage.getItem(storageKey) === 'true'
    hydrated.value = true
  }

  function setEnabled(next: boolean) {
    enabled.value = next
    persist()
  }

  function toggle() {
    setEnabled(!enabled.value)
  }

  function persist() {
    if (typeof window === 'undefined') return
    window.localStorage.setItem(storageKey, String(enabled.value))
  }

  return {
    enabled,
    hydrated,
    hydrate,
    setEnabled,
    toggle,
  }
})
