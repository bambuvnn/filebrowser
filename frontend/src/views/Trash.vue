<template>
  <div class="trash-wrapper">
    <!-- Header -->
    <div class="card-title">
      <h2>
        <i class="material-icons" style="vertical-align: middle; margin-right: 0.3em;">delete</i>
        {{ $t("trash.title") }}
      </h2>
    </div>

    <!-- Toolbar: admin toggle + bulk actions -->
    <div class="card-content" v-if="!loading">
      <div class="trash-toolbar">
        <!-- Admin: show all users toggle -->
        <div v-if="isAdmin" class="admin-toggle">
          <input
            id="trash-all-users"
            v-model="showAllUsers"
            type="checkbox"
            @change="loadTrash"
          />
          <label for="trash-all-users">{{ $t("trash.allUsers") }}</label>
        </div>

        <!-- Bulk actions -->
        <div v-if="selectedIds.size > 0" class="trash-bulk-actions">
          <span class="selection-info">
            {{ $t("trash.itemsSelected", { count: selectedIds.size }) }}
          </span>
          <button
            class="action"
            :title="$t('trash.restore')"
            @click="restoreSelected"
          >
            <i class="material-icons">restore_from_trash</i>
          </button>
          <button
            class="action"
            :title="$t('trash.deletePermanently')"
            @click="confirmPermanentDelete"
          >
            <i class="material-icons">delete_forever</i>
          </button>
        </div>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="loading-wrapper">
      <i class="material-icons spin">autorenew</i>
    </div>

    <!-- Empty state -->
    <h2 v-else-if="items.length === 0" class="message">
      <i class="material-icons">delete_outline</i>
      <span>{{ $t("trash.empty") }}</span>
    </h2>

    <!-- Items table -->
    <div v-else class="card-content full">
      <table aria-label="Trash items">
        <thead>
          <tr>
            <th class="col-check">
              <input
                type="checkbox"
                :checked="allSelected"
                :indeterminate.prop="someSelected && !allSelected"
                @change="toggleSelectAll"
                :title="allSelected ? $t('trash.unselectAll') : $t('trash.selectAll')"
              />
            </th>
            <th>{{ $t("trash.name") }}</th>
            <th class="hide-on-small">{{ $t("trash.originalPath") }}</th>
            <th v-if="showAllUsers" class="hide-on-small">{{ $t("trash.owner") }}</th>
            <th class="hide-on-small">{{ $t("trash.deletedAt") }}</th>
            <th class="hide-on-small">{{ $t("trash.expiresAt") }}</th>
            <th></th>
            <th></th>
          </tr>
        </thead>
        <tbody class="settings-items">
          <tr
            v-for="item in items"
            :key="item.id"
            class="item"
            :class="{ 'selected-row': selectedIds.has(item.id) }"
            @click="toggleSelect(item.id)"
          >
            <td class="col-check" @click.stop>
              <input
                type="checkbox"
                :checked="selectedIds.has(item.id)"
                @change="toggleSelect(item.id)"
              />
            </td>
            <td>
              <div class="item-name-cell">
                <i class="material-icons item-icon">
                  {{ item.isDir ? 'folder' : 'insert_drive_file' }}
                </i>
                <span>{{ itemName(item) }}</span>
              </div>
            </td>
            <td class="hide-on-small">
              <code class="path-text">{{ item.originalPath }}</code>
            </td>
            <td v-if="showAllUsers" class="hide-on-small">{{ item.username }}</td>
            <td class="hide-on-small small-text">{{ formatDate(item.deletedAt) }}</td>
            <td class="hide-on-small small-text">
              <span :class="{ 'expires-soon': isExpiringSoon(item.expiresAt) }">
                {{ formatDate(item.expiresAt) }}
              </span>
            </td>
            <td class="small" @click.stop>
              <button
                class="action"
                :title="$t('trash.restore')"
                @click.stop="restoreItem(item)"
              >
                <i class="material-icons">restore_from_trash</i>
              </button>
            </td>
            <td class="small" @click.stop>
              <button
                class="action"
                :title="$t('trash.deletePermanently')"
                @click.stop="confirmDeleteItem(item)"
              >
                <i class="material-icons">delete_forever</i>
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Confirm delete dialog (using existing card/hover pattern) -->
    <div v-if="confirmDialog" class="overlay" @click.self="confirmDialog = null">
      <div class="card floating confirm-card">
        <div class="card-title">
          <h2>{{ $t("trash.confirmDeleteTitle") }}</h2>
        </div>
        <div class="card-content">
          <p v-if="confirmDialog.ids.length === 1">
            {{ $t("trash.confirmDeleteSingle") }}
          </p>
          <p v-else>
            {{ $t("trash.confirmDeleteMultiple", { count: confirmDialog.ids.length }) }}
          </p>
        </div>
        <div class="card-actions">
          <button class="button button--flat" @click="confirmDialog = null">
            {{ $t("general.cancel") }}
          </button>
          <button class="button button--flat button--red" @click="executeDelete">
            {{ $t("trash.deletePermanently") }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { listTrash, restoreTrash, permanentDelete } from '@/api/trash.js'
import { notify } from '@/notify'
import { getters } from '@/store'

export default {
  name: 'Trash',

  data() {
    return {
      items: [],
      loading: false,
      showAllUsers: false,
      selectedIds: new Set(),
      confirmDialog: null,
    }
  },

  computed: {
    isAdmin() {
      return getters.isAdmin()
    },
    allSelected() {
      return this.items.length > 0 && this.selectedIds.size === this.items.length
    },
    someSelected() {
      return this.selectedIds.size > 0
    },
  },

  mounted() {
    this.loadTrash()
  },

  methods: {
    async loadTrash() {
      this.loading = true
      this.selectedIds = new Set()
      try {
        this.items = await listTrash(this.showAllUsers)
      } catch {
        // error shown by api
      } finally {
        this.loading = false
      }
    },

    itemName(item) {
      const parts = (item.originalPath || '').split('/').filter(Boolean)
      return parts[parts.length - 1] || item.originalPath
    },

    formatDate(isoStr) {
      if (!isoStr) return '—'
      return new Date(isoStr).toLocaleString()
    },

    isExpiringSoon(isoStr) {
      if (!isoStr) return false
      const diff = new Date(isoStr) - Date.now()
      return diff > 0 && diff < 48 * 60 * 60 * 1000
    },

    toggleSelect(id) {
      const next = new Set(this.selectedIds)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      this.selectedIds = next
    },

    toggleSelectAll() {
      if (this.allSelected) {
        this.selectedIds = new Set()
      } else {
        this.selectedIds = new Set(this.items.map(i => i.id))
      }
    },

    async restoreItem(item) {
      this.loading = true
      try {
        await restoreTrash([item.id])
        notify.showSuccessToast(this.$t('trash.restoreSuccess'))
        await this.loadTrash()
      } catch {
        notify.showError(this.$t('trash.restoreFailed'))
        this.loading = false
      }
    },

    async restoreSelected() {
      this.loading = true
      const ids = Array.from(this.selectedIds)
      try {
        await restoreTrash(ids)
        notify.showSuccessToast(this.$t('trash.restoreSuccess'))
        await this.loadTrash()
      } catch {
        notify.showError(this.$t('trash.restoreFailed'))
        this.loading = false
      }
    },

    confirmDeleteItem(item) {
      this.confirmDialog = { ids: [item.id] }
    },

    confirmPermanentDelete() {
      this.confirmDialog = { ids: Array.from(this.selectedIds) }
    },

    async executeDelete() {
      if (!this.confirmDialog) return
      const ids = this.confirmDialog.ids
      this.confirmDialog = null
      this.loading = true
      try {
        await permanentDelete(ids)
        notify.showSuccessToast(this.$t('trash.deleteSuccess'))
        await this.loadTrash()
      } catch {
        notify.showError(this.$t('trash.deleteFailed'))
        this.loading = false
      }
    },
  },
}
</script>

<style scoped>
.trash-wrapper {
  width: 100%;
}

/* ── Toolbar ────────────────────────────────── */
.trash-toolbar {
  display: flex;
  align-items: center;
  gap: 1rem;
  flex-wrap: wrap;
  min-height: 2rem;
}

.admin-toggle {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.9rem;
  cursor: pointer;
  user-select: none;
}

.admin-toggle label {
  cursor: pointer;
}

.trash-bulk-actions {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  margin-left: auto;
}

.selection-info {
  font-size: 0.85rem;
  color: var(--textSecondary);
  margin-right: 0.5rem;
}

/* ── Loading ─────────────────────────────────── */
.loading-wrapper {
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 5rem 2rem;
  color: var(--textSecondary);
}

.loading-wrapper .material-icons {
  font-size: 2.5rem;
  opacity: 0.5;
}

/* ── Table tweaks ────────────────────────────── */
.col-check {
  width: 40px;
  text-align: center !important;
}

.item-name-cell {
  display: flex;
  align-items: center;
  gap: 0.4rem;
}

.item-icon {
  font-size: 1.1rem;
  color: var(--textSecondary);
  flex-shrink: 0;
}

.path-text {
  font-size: 0.8rem;
  color: var(--textSecondary);
  word-break: break-all;
  background: none;
  padding: 0;
  border: none;
  font-family: monospace;
}

.small-text {
  font-size: 0.85rem;
  color: var(--textSecondary);
  white-space: nowrap;
}

.expires-soon {
  color: #e65100;
  font-weight: 600;
}

/* Highlight selected rows */
.selected-row {
  background: var(--alt-background) !important;
}

/* ── Confirm dialog ───────────────────────────── */
.overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.45);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 999;
}

.confirm-card {
  max-width: 460px;
  width: 90%;
}

.confirm-card p {
  margin: 0;
  color: var(--textSecondary);
}

/* ── Animations ─────────────────────────────── */
.spin {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to   { transform: rotate(360deg); }
}

/* ── Responsive ─────────────────────────────── */
@media (max-width: 700px) {
  .hide-on-small {
    display: none;
  }
}
</style>
