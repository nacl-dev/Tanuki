<template>
  <div class="duplicates-page">
    <div class="page-header" :class="{ 'page-header--embedded': embedded }">
      <div>
        <h2 class="page-title">
          <AppIcon name="search" :size="18" />
          Duplicate Detection
        </h2>
        <p class="page-copy">Review perceptual matches in batches, keep the best source, and merge tags before cleanup.</p>
      </div>
      <button class="btn btn-primary" :disabled="store.loading" @click="store.fetchGroups()">
        <AppIcon name="refresh" :size="14" />
        {{ store.loading ? 'Scanning…' : 'Refresh' }}
      </button>
    </div>

    <div v-if="store.error" class="error-banner">
      {{ store.error }}
    </div>

    <div v-if="store.loading" class="loading">Loading duplicate groups…</div>

    <div v-else-if="store.groups.length === 0" class="empty-state">
      <p>No duplicates found in your library.</p>
      <p class="sub">
        Make sure perceptual hashes have been computed.
        They are automatically generated when <code>PHASH_ON_SCAN=true</code> (default).
      </p>
    </div>

    <div v-else class="groups-list">
      <p class="groups-count">{{ store.groups.length }} duplicate group{{ store.groups.length !== 1 ? 's' : '' }} found</p>
      <DuplicateGroup
        v-for="group in store.groups"
        :key="group.group_id"
        :group="group"
        @resolved="onResolved"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useDedupStore } from '@/stores/dedupStore'
import AppIcon from '@/components/Layout/AppIcon.vue'
import DuplicateGroup from '@/components/Duplicates/DuplicateGroup.vue'

withDefaults(defineProps<{ embedded?: boolean }>(), {
  embedded: false,
})

const store = useDedupStore()

onMounted(() => {
  store.fetchGroups()
})

async function onResolved(keepId: string, deleteIds: string[], mergeTags: boolean) {
  await store.resolve(keepId, deleteIds, mergeTags)
}
</script>

<style scoped>
.duplicates-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.page-header--embedded .page-title {
  font-size: 18px;
}

.page-title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 22px;
  font-weight: 700;
}

.page-copy {
  margin-top: 6px;
  max-width: 720px;
  color: var(--text-muted);
  font-size: 13px;
  line-height: 1.6;
}

.error-banner {
  background: rgba(239, 68, 68, 0.15);
  border: 1px solid #ef4444;
  border-radius: var(--radius);
  padding: 12px 16px;
  color: #ef4444;
  font-size: 14px;
}

.loading {
  text-align: center;
  padding: 48px;
  color: var(--text-muted);
}

.empty-state {
  text-align: center;
  padding: 64px 24px;
  color: var(--text-secondary);
}

.empty-state p { font-size: 16px; margin-bottom: 8px; }
.empty-state .sub { font-size: 13px; color: var(--text-muted); }
.empty-state code {
  background: var(--bg-surface);
  padding: 1px 5px;
  border-radius: 4px;
  font-size: 12px;
}

.groups-count { font-size: 13px; color: var(--text-muted); }

.groups-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

@media (max-width: 720px) {
  .page-header {
    align-items: stretch;
  }

  .page-header .btn {
    width: 100%;
    justify-content: center;
  }
}
</style>
