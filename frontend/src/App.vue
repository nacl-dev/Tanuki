<template>
  <div class="app-layout">
    <Sidebar />
    <div class="main-content">
      <TopBar />
      <main class="page-content">
        <RouterView />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import Sidebar from '@/components/Layout/Sidebar.vue'
import TopBar from '@/components/Layout/TopBar.vue'
</script>
