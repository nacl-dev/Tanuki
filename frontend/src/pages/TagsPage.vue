<template>
  <div class="tags-page">
    <div class="tags-header">
      <div>
        <h2 class="page-title">Tags</h2>
        <p class="page-copy">Namespaces, aliases and implications stay together here, so search, imports and auto-tagging converge on the same metadata.</p>
      </div>
    </div>

    <div class="tags-layout">
      <aside class="tags-sidebar">
        <div class="card rules-form-card">
          <div class="section-header">
            <div>
              <h3>Create Alias</h3>
              <p class="section-copy">Map site-specific names, typos or alternate namespaces to one canonical tag.</p>
            </div>
          </div>
          <form class="compact-form" @submit.prevent="submitAlias">
            <input
              v-model="aliasDraft.alias_name"
              class="input"
              type="text"
              placeholder="creator:john_doe"
            />
            <TagSuggestInput
              v-model="aliasDraft.target"
              placeholder="artist:john doe"
            />
            <button class="btn btn-primary" type="submit" :disabled="aliasSaving || !canCreateAlias">
              {{ aliasSaving ? 'Saving…' : 'Create Alias' }}
            </button>
          </form>
        </div>

        <div class="card rules-form-card">
          <div class="section-header">
            <div>
              <h3>Create Implication</h3>
              <p class="section-copy">Add broader metadata automatically when a tag should always expand into another one.</p>
            </div>
          </div>
          <form class="compact-form" @submit.prevent="submitImplication">
            <TagSuggestInput
              v-model="implicationDraft.source"
              placeholder="character:asuka"
            />
            <TagSuggestInput
              v-model="implicationDraft.implied"
              placeholder="series:evangelion"
            />
            <button class="btn btn-secondary" type="submit" :disabled="implicationSaving || !canCreateImplication">
              {{ implicationSaving ? 'Saving…' : 'Create Implication' }}
            </button>
          </form>
        </div>

        <div class="card rules-form-card">
          <div class="section-header">
            <div>
              <h3>Merge Tag</h3>
              <p class="section-copy">Preview how many media links and rules move before replacing one tag with another canonical tag.</p>
            </div>
          </div>
          <form class="compact-form" @submit.prevent="previewMerge">
            <TagSuggestInput
              v-model="mergeDraft.source"
              placeholder="old tag or alias"
            />
            <TagSuggestInput
              v-model="mergeDraft.target"
              placeholder="canonical:tag"
            />
            <label class="checkbox-row">
              <input v-model="mergeDraft.create_alias" type="checkbox" />
              <span>Keep the old expression as alias after merge</span>
            </label>
            <button class="btn btn-primary" type="submit" :disabled="store.mergeLoading || !canPreviewMerge">
              {{ store.mergeLoading ? 'Preparing…' : 'Preview Merge' }}
            </button>
          </form>

          <div v-if="store.mergePreview" class="merge-preview">
            <div class="merge-preview__headline">
              <span class="rule-token">{{ store.mergePreview.source.category !== 'general' ? `${store.mergePreview.source.category}:` : '' }}{{ store.mergePreview.source.name }}</span>
              <span class="rule-arrow">→</span>
              <TagBadge :tag="store.mergePreview.target" />
            </div>
            <div class="merge-preview__stats">
              <div class="merge-stat">
                <strong>{{ store.mergePreview.source_media_count }}</strong>
                <span>source media links</span>
              </div>
              <div class="merge-stat">
                <strong>{{ store.mergePreview.overlapping_media_count }}</strong>
                <span>already on target</span>
              </div>
              <div class="merge-stat">
                <strong>{{ store.mergePreview.source_alias_count }}</strong>
                <span>aliases moved</span>
              </div>
              <div class="merge-stat">
                <strong>{{ store.mergePreview.source_outbound_implications + store.mergePreview.source_inbound_implications }}</strong>
                <span>implications re-linked</span>
              </div>
            </div>
            <p class="merge-preview__copy">
              <template v-if="store.mergePreview.target_created">The target tag will be created during this merge.</template>
              <template v-else>The target already has {{ store.mergePreview.target_media_count }} media links.</template>
            </p>
            <button class="btn btn-danger" type="button" :disabled="store.mergeLoading" @click="runMerge">
              {{ store.mergeLoading ? 'Merging…' : 'Merge Tags' }}
            </button>
          </div>
        </div>
      </aside>

      <section class="tags-main">
        <div class="card rules-card">
          <div class="section-header">
            <div>
              <h3>Rule Overview</h3>
              <p class="section-copy">Aliases normalize names, implications enrich them with related metadata.</p>
            </div>
          </div>

          <div class="rule-columns">
            <section class="rule-panel">
              <div class="rule-panel__header">
                <h4>Aliases</h4>
                <span class="rule-count">{{ store.aliases.length }}</span>
              </div>

              <div v-if="store.rulesLoading && !store.aliases.length" class="loading-inline">Loading…</div>
              <div v-else-if="store.aliases.length" class="rule-list">
                <div v-for="rule in store.aliases" :key="rule.id" class="rule-row">
                  <div class="rule-row__body">
                    <span class="rule-token">{{ rule.alias_name }}</span>
                    <span class="rule-arrow">→</span>
                    <TagBadge :tag="rule.tag" />
                  </div>
                  <button type="button" class="tag-delete" :aria-label="`Delete alias ${rule.alias_name}`" @click="removeAlias(rule)">
                    Remove
                  </button>
                </div>
              </div>
              <div v-else class="empty-state">No aliases yet.</div>
            </section>

            <section class="rule-panel">
              <div class="rule-panel__header">
                <h4>Implications</h4>
                <span class="rule-count">{{ store.implications.length }}</span>
              </div>

              <div v-if="store.rulesLoading && !store.implications.length" class="loading-inline">Loading…</div>
              <div v-else-if="store.implications.length" class="rule-list">
                <div v-for="rule in store.implications" :key="rule.id" class="rule-row rule-row--stacked">
                  <div class="rule-row__body">
                    <TagBadge :tag="rule.tag" />
                    <span class="rule-arrow">implies</span>
                    <TagBadge :tag="rule.implied_tag" />
                  </div>
                  <button
                    type="button"
                    class="tag-delete"
                    :aria-label="`Delete implication from ${rule.tag.name} to ${rule.implied_tag.name}`"
                    @click="removeImplication(rule)"
                  >
                    Remove
                  </button>
                </div>
              </div>
              <div v-else class="empty-state">No implications yet.</div>
            </section>
          </div>
        </div>

        <div class="card tags-card">
          <div class="section-header">
            <div>
              <h3>Tag Inventory</h3>
              <p class="section-copy">{{ store.tags.length }} tags in the current category view.</p>
            </div>
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
              <button type="button" class="tag-delete" :aria-label="`Delete tag ${tag.name}`" @click="removeTag(tag.id, tag.name)">Remove</button>
            </div>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useTagStore } from '@/stores/tagStore'
import { useNoticeStore } from '@/stores/noticeStore'
import TagBadge from '@/components/Tags/TagBadge.vue'
import TagSuggestInput from '@/components/Tags/TagSuggestInput.vue'
import type { TagAlias, TagImplication } from '@/api/tagApi'

const store = useTagStore()
const { pushNotice } = useNoticeStore()
const activeCategory = ref('')
const aliasSaving = ref(false)
const implicationSaving = ref(false)
const aliasDraft = ref({ alias_name: '', target: '' })
const implicationDraft = ref({ source: '', implied: '' })
const mergeDraft = ref({ source: '', target: '', create_alias: true })

const categories = [
  { value: '', label: 'All' },
  { value: 'general', label: 'General' },
  { value: 'artist', label: 'Artists' },
  { value: 'character', label: 'Characters' },
  { value: 'parody', label: 'Parodies' },
  { value: 'genre', label: 'Genres' },
  { value: 'meta', label: 'Meta' },
]

const canCreateAlias = computed(() =>
  aliasDraft.value.alias_name.trim().length > 0 && aliasDraft.value.target.trim().length > 0,
)

const canCreateImplication = computed(() =>
  implicationDraft.value.source.trim().length > 0 && implicationDraft.value.implied.trim().length > 0,
)

const canPreviewMerge = computed(() =>
  mergeDraft.value.source.trim().length > 0 && mergeDraft.value.target.trim().length > 0,
)

function errorMessage(error: unknown, fallback: string) {
  return error instanceof Error ? error.message : fallback
}

async function loadPage() {
  try {
    await Promise.all([
      store.fetchAll(activeCategory.value || undefined),
      store.fetchRules(),
    ])
  } catch (error) {
    pushNotice({ type: 'error', message: errorMessage(error, 'Failed to load tags') })
  }
}

async function selectCategory(cat: string) {
  activeCategory.value = cat
  try {
    await store.fetchAll(cat || undefined)
  } catch (error) {
    pushNotice({ type: 'error', message: errorMessage(error, 'Failed to load tags') })
  }
}

async function submitAlias() {
  if (!canCreateAlias.value) {
    pushNotice({ type: 'error', message: 'Please enter both an alias and a target tag.' })
    return
  }

  aliasSaving.value = true
  try {
    const rule = await store.createAlias(aliasDraft.value.alias_name.trim(), aliasDraft.value.target.trim())
    aliasDraft.value = { alias_name: '', target: '' }
    pushNotice({ type: 'success', message: `Alias "${rule.alias_name}" now resolves to ${rule.tag.name}.` })
  } catch (error) {
    pushNotice({ type: 'error', message: errorMessage(error, 'Failed to create alias') })
  } finally {
    aliasSaving.value = false
  }
}

async function submitImplication() {
  if (!canCreateImplication.value) {
    pushNotice({ type: 'error', message: 'Please enter both the source and implied tag.' })
    return
  }

  implicationSaving.value = true
  try {
    const rule = await store.createImplication(implicationDraft.value.source.trim(), implicationDraft.value.implied.trim())
    implicationDraft.value = { source: '', implied: '' }
    pushNotice({ type: 'success', message: `${rule.tag.name} now implies ${rule.implied_tag.name}.` })
  } catch (error) {
    pushNotice({ type: 'error', message: errorMessage(error, 'Failed to create implication') })
  } finally {
    implicationSaving.value = false
  }
}

async function removeAlias(rule: TagAlias) {
  try {
    await store.removeAlias(rule.id)
    pushNotice({ type: 'success', message: `Alias "${rule.alias_name}" removed.` })
  } catch (error) {
    pushNotice({ type: 'error', message: errorMessage(error, 'Failed to remove alias') })
  }
}

async function removeImplication(rule: TagImplication) {
  try {
    await store.removeImplication(rule.id)
    pushNotice({ type: 'success', message: `Implication ${rule.tag.name} → ${rule.implied_tag.name} removed.` })
  } catch (error) {
    pushNotice({ type: 'error', message: errorMessage(error, 'Failed to remove implication') })
  }
}

async function removeTag(id: string, name: string) {
  try {
    await store.remove(id)
    pushNotice({ type: 'success', message: `Tag "${name}" removed.` })
  } catch (error) {
    pushNotice({ type: 'error', message: errorMessage(error, 'Failed to remove tag') })
  }
}

async function previewMerge() {
  if (!canPreviewMerge.value) {
    pushNotice({ type: 'error', message: 'Please enter both the source and target tag.' })
    return
  }
  try {
    await store.previewMerge(mergeDraft.value.source.trim(), mergeDraft.value.target.trim())
  } catch (error) {
    pushNotice({ type: 'error', message: errorMessage(error, 'Failed to preview merge') })
  }
}

async function runMerge() {
  if (!store.mergePreview) return
  try {
    const result = await store.merge(
      mergeDraft.value.source.trim(),
      mergeDraft.value.target.trim(),
      mergeDraft.value.create_alias,
    )
    mergeDraft.value = { source: '', target: '', create_alias: true }
    pushNotice({
      type: 'success',
      message: `Merged ${result.source.name} into ${result.target.name}. ${result.moved_media_tags} media links moved.`,
    })
  } catch (error) {
    pushNotice({ type: 'error', message: errorMessage(error, 'Failed to merge tags') })
  }
}

onMounted(() => {
  void loadPage()
})

watch(
  () => [mergeDraft.value.source, mergeDraft.value.target],
  () => {
    store.clearMergePreview()
  },
)
</script>

<style scoped>
.tags-page {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.page-title {
  margin: 0;
  font-size: 22px;
  font-weight: 700;
}

.page-copy {
  margin: 6px 0 0;
  color: var(--text-muted);
  font-size: 13px;
}

.tags-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
}

.tags-layout {
  display: grid;
  grid-template-columns: minmax(260px, 320px) minmax(0, 1fr);
  gap: 24px;
  align-items: flex-start;
}

.tags-sidebar,
.tags-main {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.tags-sidebar {
  position: sticky;
  top: 16px;
}

.rules-form-card,
.rules-card,
.tags-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.section-header {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: flex-start;
}

.section-copy {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--text-muted);
}

.category-filters {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.btn-sm {
  padding: 5px 12px;
  font-size: 12px;
}

.active {
  background: var(--accent-dimmed);
  color: var(--accent);
  border-color: var(--accent);
}

.loading {
  color: var(--text-muted);
  text-align: center;
  padding: 48px;
}

.compact-form {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.input {
  width: 100%;
  background: var(--bg-hover);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  color: var(--text-primary);
  padding: 8px 12px;
  font-size: 14px;
}

.checkbox-row {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--text-muted);
}

.rule-columns {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.rule-panel {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--bg-surface);
}

.rule-panel__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.rule-panel__header h4 {
  margin: 0;
}

.rule-count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 28px;
  padding: 4px 8px;
  border-radius: 999px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  color: var(--text-muted);
  font-size: 12px;
}

.rule-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.rule-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 10px;
  border-radius: 12px;
  border: 1px solid var(--border);
  background: var(--bg-card);
}

.rule-row--stacked {
  align-items: flex-start;
}

.rule-row__body {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  min-width: 0;
}

.rule-token {
  display: inline-flex;
  align-items: center;
  padding: 4px 8px;
  border-radius: 999px;
  background: var(--bg-hover);
  border: 1px solid var(--border);
  color: var(--text-primary);
  font-size: 12px;
  word-break: break-word;
}

.rule-arrow {
  color: var(--text-muted);
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.loading-inline,
.empty-state {
  color: var(--text-muted);
  font-size: 13px;
}

.merge-preview {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
  border: 1px solid rgba(245, 158, 11, 0.2);
  border-radius: 14px;
  background: rgba(245, 158, 11, 0.06);
}

.merge-preview__headline {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.merge-preview__stats {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.merge-stat {
  display: flex;
  flex-direction: column;
  gap: 3px;
  padding: 10px;
  border-radius: 12px;
  background: rgba(0, 0, 0, 0.14);
  border: 1px solid rgba(255, 255, 255, 0.06);
}

.merge-stat strong {
  font-size: 18px;
  color: var(--text-primary);
}

.merge-stat span,
.merge-preview__copy {
  font-size: 12px;
  color: var(--text-muted);
  margin: 0;
}

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

.tag-count {
  font-size: 11px;
  color: var(--text-muted);
}

.tag-delete {
  background: transparent;
  border: none;
  cursor: pointer;
  color: var(--text-muted);
  font-size: 10px;
  padding: 2px 4px;
}

.tag-delete:hover {
  color: var(--danger);
}

@media (max-width: 980px) {
  .tags-layout {
    grid-template-columns: 1fr;
  }

  .tags-sidebar {
    position: static;
  }

  .rule-columns {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .tags-page {
    gap: 16px;
  }

  .section-header {
    flex-direction: column;
  }

  .category-filters {
    width: 100%;
  }

  .compact-form .btn {
    width: 100%;
  }

  .rule-row {
    flex-direction: column;
    align-items: stretch;
  }

  .merge-preview__stats {
    grid-template-columns: 1fr;
  }
}
</style>
