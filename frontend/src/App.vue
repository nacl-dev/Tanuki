<template>
  <div class="app-layout" v-if="authStore.isAuthenticated">
    <Sidebar :open="sidebarOpen" @close="sidebarOpen = false" />
    <button
      v-if="sidebarOpen"
      type="button"
      class="sidebar-backdrop"
      aria-label="Close navigation"
      @click="sidebarOpen = false"
    />
    <div class="main-content">
      <TopBar @toggle-sidebar="sidebarOpen = !sidebarOpen" />
      <main class="page-content">
        <RouterView />
      </main>
    </div>
    <NoticeStack />
  </div>
  <RouterView v-else />
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import NoticeStack from '@/components/Layout/NoticeStack.vue'
import Sidebar from '@/components/Layout/Sidebar.vue'
import TopBar from '@/components/Layout/TopBar.vue'
import { useAuthStore } from '@/stores/authStore'

const authStore = useAuthStore()
const route = useRoute()
const sidebarOpen = ref(false)

watch(() => route.fullPath, () => {
  sidebarOpen.value = false
})
</script>

<style scoped>
.sidebar-backdrop {
  position: fixed;
  inset: 0;
  border: none;
  background: rgba(0, 0, 0, 0.5);
  z-index: 30;
}

@media (min-width: 901px) {
  .sidebar-backdrop {
    display: none;
  }
}
</style>
