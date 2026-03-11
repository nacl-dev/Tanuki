<template>
  <div class="user-management">
    <div class="panel-head">
      <div>
        <h3>User Management</h3>
        <p class="panel-copy">Manage access, roles and active status for Tanuki accounts.</p>
      </div>
      <button class="btn btn-primary" :disabled="loading" @click="loadUsers">
        {{ loading ? 'Refreshing…' : 'Refresh Users' }}
      </button>
    </div>

    <div v-if="error" class="panel-error">{{ error }}</div>
    <div v-else-if="loading && !users.length" class="panel-empty">Loading users…</div>
    <div v-else-if="!users.length" class="panel-empty">No users found.</div>

    <div v-else class="user-grid">
      <article v-for="user in users" :key="user.id" class="card user-card">
        <div class="user-card__head">
          <div>
            <div class="user-card__title">
              <strong>{{ user.display_name || user.username }}</strong>
              <span :class="['user-role-badge', `user-role-badge--${user.role}`]">{{ user.role }}</span>
              <span v-if="user.id === authStore.user?.id" class="user-self-badge">You</span>
            </div>
            <p class="user-card__meta">@{{ user.username }} · {{ formatDate(user.created_at) }}</p>
          </div>
          <label class="user-active-toggle">
            <input v-model="user.is_active" type="checkbox" />
            <span>{{ user.is_active ? 'Active' : 'Disabled' }}</span>
          </label>
        </div>

        <div class="user-form">
          <label class="user-field">
            <span>Display name</span>
            <input v-model="user.display_name" class="input" type="text" placeholder="Optional display name" />
          </label>

          <label class="user-field">
            <span>Email</span>
            <input v-model="user.email" class="input" type="email" placeholder="user@example.com" />
          </label>

          <label class="user-field">
            <span>Role</span>
            <select v-model="user.role" class="input">
              <option value="user">User</option>
              <option value="admin">Admin</option>
            </select>
          </label>
        </div>

        <div class="user-card__actions">
          <button
            class="btn btn-secondary"
            :disabled="isSaving(user.id) || isDeleting(user.id)"
            @click="saveUser(user)"
          >
            {{ isSaving(user.id) ? 'Saving…' : 'Save Changes' }}
          </button>
          <button
            class="btn btn-danger"
            :disabled="user.id === authStore.user?.id || isSaving(user.id) || isDeleting(user.id)"
            @click="pendingDelete = user"
          >
            {{ isDeleting(user.id) ? 'Deleting…' : 'Delete User' }}
          </button>
        </div>
      </article>
    </div>
  </div>

  <ConfirmDialog
    v-if="pendingDelete"
    title="Delete User"
    :message="`Delete ${pendingDelete.display_name || pendingDelete.username}? Collections, download jobs and schedules owned by this user will also be removed.`"
    confirm-label="Delete User"
    @cancel="pendingDelete = null"
    @confirm="deleteUser"
  />
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { adminUserApi } from '@/api/adminUserApi'
import type { User } from '@/api/authApi'
import ConfirmDialog from '@/components/Layout/ConfirmDialog.vue'
import { useAuthStore } from '@/stores/authStore'
import { useNoticeStore } from '@/stores/noticeStore'

const authStore = useAuthStore()
const { pushNotice } = useNoticeStore()

const users = ref<User[]>([])
const loading = ref(false)
const error = ref('')
const pendingDelete = ref<User | null>(null)
const savingIds = ref<string[]>([])
const deletingIds = ref<string[]>([])

onMounted(() => {
  void loadUsers()
})

async function loadUsers() {
  loading.value = true
  error.value = ''
  try {
    users.value = await adminUserApi.list()
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to load users'
  } finally {
    loading.value = false
  }
}

async function saveUser(user: User) {
  savingIds.value = [...savingIds.value, user.id]
  try {
    const updated = await adminUserApi.update(user.id, {
      display_name: user.display_name,
      email: user.email,
      role: user.role,
      is_active: user.is_active,
    })
    users.value = users.value.map((item) => (item.id === updated.id ? updated : item))
    if (updated.id === authStore.user?.id) {
      authStore.user = updated
    }
    pushNotice({ type: 'success', message: `Updated ${updated.display_name || updated.username}.` })
  } catch (err) {
    pushNotice({
      type: 'error',
      message: err instanceof Error ? err.message : 'Failed to update user',
      durationMs: 5000,
    })
    await loadUsers()
  } finally {
    savingIds.value = savingIds.value.filter((id) => id !== user.id)
  }
}

async function deleteUser() {
  if (!pendingDelete.value) return
  const target = pendingDelete.value
  deletingIds.value = [...deletingIds.value, target.id]
  try {
    await adminUserApi.remove(target.id)
    users.value = users.value.filter((user) => user.id !== target.id)
    pushNotice({ type: 'success', message: `Deleted ${target.display_name || target.username}.` })
    pendingDelete.value = null
  } catch (err) {
    pushNotice({
      type: 'error',
      message: err instanceof Error ? err.message : 'Failed to delete user',
      durationMs: 5000,
    })
  } finally {
    deletingIds.value = deletingIds.value.filter((id) => id !== target.id)
  }
}

function isSaving(id: string) {
  return savingIds.value.includes(id)
}

function isDeleting(id: string) {
  return deletingIds.value.includes(id)
}

function formatDate(value: string) {
  return new Date(value).toLocaleDateString()
}
</script>

<style scoped>
.user-management {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.panel-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.user-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 16px;
}

.user-card {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.user-card__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.user-card__title {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.user-card__meta {
  margin: 6px 0 0;
  color: var(--text-muted);
  font-size: 12px;
}

.user-form {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.user-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.user-field:last-child {
  grid-column: 1 / -1;
}

.user-field span {
  font-size: 12px;
  color: var(--text-secondary);
}

.user-card__actions {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
}

.user-role-badge,
.user-self-badge {
  display: inline-flex;
  align-items: center;
  padding: 4px 8px;
  border-radius: 999px;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

.user-role-badge--admin {
  background: rgba(245, 158, 11, 0.12);
  border: 1px solid rgba(245, 158, 11, 0.22);
  color: var(--accent);
}

.user-role-badge--user {
  background: rgba(148, 163, 184, 0.12);
  border: 1px solid rgba(148, 163, 184, 0.2);
  color: #cbd5f5;
}

.user-self-badge {
  background: rgba(59, 130, 246, 0.12);
  border: 1px solid rgba(59, 130, 246, 0.2);
  color: #93c5fd;
}

.user-active-toggle {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--text-secondary);
  font-size: 12px;
}

@media (max-width: 720px) {
  .panel-head .btn,
  .user-card__actions .btn {
    width: 100%;
    justify-content: center;
  }

  .user-form {
    grid-template-columns: 1fr;
  }

  .user-card__head {
    flex-direction: column;
  }
}
</style>
