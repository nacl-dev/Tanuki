<template>
  <div class="library-page">
    <!-- Filters sidebar -->
    <aside class="filter-sidebar">
      <h4>Type</h4>
      <label v-for="t in types" :key="t.value" class="filter-option">
        <input
          type="radio"
          name="type"
          :value="t.value"
          :checked="store.filters.type === t.value"
          @change="store.setFilter('type', t.value)"
        />
        {{ t.label }}
      </label>

      <h4>Show</h4>
      <label class="filter-option">
        <input
          type="checkbox"
          :checked="store.filters.favorite"
          @change="store.setFilter('favorite', ($event.target as HTMLInputElement).checked || undefined)"
        />
        Favorites only
      </label>
    </aside>

    <!-- Gallery -->
    <section class="gallery-section">
      <div class="gallery-header">
        <span class="gallery-count">{{ store.total }} items</span>
      </div>
      <MediaGrid :items="store.items" :loading="store.loading" />
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useMediaStore } from '@/stores/mediaStore'
import MediaGrid from '@/components/Gallery/MediaGrid.vue'

const store = useMediaStore()

const types = [
  { value: '',          label: 'All'        },
  { value: 'video',     label: '🎬 Videos'  },
  { value: 'image',     label: '🖼️ Images'  },
  { value: 'manga',     label: '📖 Manga'   },
  { value: 'comic',     label: '📕 Comics'  },
  { value: 'doujinshi', label: '📗 Doujin'  },
]

onMounted(() => store.fetchList())
</script>

<style scoped>
.library-page {
  display: flex;
  gap: 24px;
  align-items: flex-start;
}

.filter-sidebar {
  width: 180px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.filter-sidebar h4 {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
  margin-top: 12px;
}

.filter-option {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  font-size: 13px;
  color: var(--text-secondary);
}

.filter-option:hover { color: var(--text-primary); }

.gallery-section { flex: 1; display: flex; flex-direction: column; gap: 16px; }

.gallery-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.gallery-count { font-size: 13px; color: var(--text-muted); }
</style>
