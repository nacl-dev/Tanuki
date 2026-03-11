<template>
  <div class="schedule-manager">
    <div class="sm-header">
      <h3>Schedules</h3>
      <button type="button" class="btn btn-primary btn-sm" @click="showForm = !showForm">New Schedule</button>
    </div>

    <!-- New schedule form -->
    <div v-if="showForm" class="sm-form card">
      <div class="form-field">
        <label>Name</label>
        <input v-model="form.name" class="input" placeholder="Daily gallery update" />
      </div>
      <div class="form-field">
        <label>URL pattern</label>
        <input v-model="form.url_pattern" class="input" placeholder="https://example.com/artist/xyz" />
      </div>
      <div class="form-field">
        <label>Cron expression</label>
        <input v-model="form.cron_expression" class="input" placeholder="0 3 * * *" />
      </div>
      <div class="form-row">
        <button type="button" class="btn btn-primary btn-sm" @click="save">Save</button>
        <button type="button" class="btn btn-ghost btn-sm" @click="showForm = false">Cancel</button>
      </div>
    </div>

    <div v-if="store.schedules.length === 0" class="sm-empty">No schedules configured.</div>

    <div v-else class="sm-list">
      <div v-for="sched in store.schedules" :key="sched.id" class="sm-item">
        <div class="sm-item__info">
          <span class="sm-item__name">{{ sched.name }}</span>
          <span class="sm-item__cron">{{ sched.cron_expression }}</span>
        </div>
        <div class="sm-item__actions">
          <button
            type="button"
            :class="['btn btn-ghost btn-sm', { 'active': sched.enabled }]"
            @click="store.updateSchedule(sched.id, { enabled: !sched.enabled })"
          >{{ sched.enabled ? 'Enabled' : 'Disabled' }}</button>
          <button type="button" class="btn btn-ghost btn-sm" :aria-label="`Delete schedule ${sched.name}`" @click="store.removeSchedule(sched.id)">Delete</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, reactive } from 'vue'
import { useDownloadStore } from '@/stores/downloadStore'

const store = useDownloadStore()
const showForm = ref(false)

const form = reactive({
  name: '',
  url_pattern: '',
  source_type: 'auto',
  cron_expression: '0 3 * * *',
  enabled: true,
  target_directory: '',
})

onMounted(() => store.fetchSchedules())

async function save() {
  await store.createSchedule(form as any)
  showForm.value = false
  Object.assign(form, { name: '', url_pattern: '', cron_expression: '0 3 * * *' })
}
</script>

<style scoped>
.schedule-manager { display: flex; flex-direction: column; gap: 12px; }
.sm-header { display: flex; justify-content: space-between; align-items: center; }
.sm-header h3 { font-size: 15px; font-weight: 600; }
.sm-form { display: flex; flex-direction: column; gap: 12px; }
.sm-list { display: flex; flex-direction: column; gap: 8px; }
.sm-empty { color: var(--text-muted); text-align: center; padding: 24px; }

.sm-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 10px 14px;
}

.sm-item__info { display: flex; flex-direction: column; gap: 2px; }
.sm-item__name { font-weight: 500; }
.sm-item__cron { font-size: 12px; color: var(--text-muted); font-family: monospace; }
.sm-item__actions { display: flex; gap: 6px; }
.btn-sm { padding: 4px 10px; font-size: 12px; }
.active { background: var(--accent-dimmed); color: var(--accent); }

.form-field { display: flex; flex-direction: column; gap: 4px; }
.form-field label { font-size: 12px; color: var(--text-secondary); }
.form-row { display: flex; gap: 8px; }

.input {
  background: var(--bg-hover);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  color: var(--text-primary);
  padding: 7px 10px;
  font-size: 13px;
  outline: none;
}
.input:focus {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px rgba(245, 158, 11, 0.14);
}
</style>
