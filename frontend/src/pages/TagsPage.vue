<template>
  <div class="tags-page">
    <div class="tags-header">
      <h2 class="page-title">Tags</h2>
      <div class="category-filters">
        <button
          v-for="cat in categories"
          :key="cat.value"
          :class="['btn btn-ghost btn-sm', { active: activeCategory === cat.value }]"
          @click="selectCategory(cat.value)"
        >{{ cat.label }}</button>
      </div>
    </div>

    <div v-if="store.loading" class="loading">Loading…</div>

    <div v-else class="tags-grid">
      <div v-for="tag in store.tags" :key="tag.id" class="tag-item">
        <TagBadge :tag="tag" />
        <span class="tag-count">{{ tag.usage_count }}</span>
        <button type="button" class="tag-delete" :aria-label="`Delete tag ${tag.name}`" @click="store.remove(tag.id)">Remove</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useTagStore } from '@/stores/tagStore'
import TagBadge from '@/components/Tags/TagBadge.vue'

const store = useTagStore()
const activeCategory = ref('')

const categories = [
  { value: '',          label: 'All'       },
  { value: 'general',   label: 'General'   },
  { value: 'artist',    label: 'Artists'   },
  { value: 'character', label: 'Characters'},
  { value: 'parody',    label: 'Parodies'  },
  { value: 'genre',     label: 'Genres'    },
  { value: 'meta',      label: 'Meta'      },
]

function selectCategory(cat: string) {
  activeCategory.value = cat
  store.fetchAll(cat || undefined)
}

onMounted(() => store.fetchAll())
</script>

<style scoped>
.tags-page { display: flex; flex-direction: column; gap: 24px; }
.page-title { font-size: 22px; font-weight: 700; }
.tags-header { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 12px; }
.category-filters { display: flex; flex-wrap: wrap; gap: 6px; }
.btn-sm { padding: 5px 12px; font-size: 12px; }
.active { background: var(--accent-dimmed); color: var(--accent); border-color: var(--accent); }
.loading { color: var(--text-muted); text-align: center; padding: 48px; }

.tags-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.tag-item {
  display: flex;
  align-items: center;
  gap: 6px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 999px;
  padding: 3px 10px 3px 4px;
}

.tag-count { font-size: 11px; color: var(--text-muted); }

.tag-delete {
  background: transparent;
  border: none;
  cursor: pointer;
  color: var(--text-muted);
  font-size: 10px;
  padding: 2px 4px;
}
.tag-delete:hover { color: var(--danger); }
</style>
