<template>
  <div class="app-layout" v-if="authStore.isAuthenticated">
    <Sidebar />
    <div class="main-content">
      <TopBar />
      <main class="page-content">
        <RouterView />
      </main>
    </div>
  </div>
  <RouterView v-else />
</template>

<script setup lang="ts">
import Sidebar from '@/components/Layout/Sidebar.vue'
import TopBar from '@/components/Layout/TopBar.vue'
import { useAuthStore } from '@/stores/authStore'

const authStore = useAuthStore()
</script>
