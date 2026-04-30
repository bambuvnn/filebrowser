<template>
  <div class="trash-page no-select">
    <!-- Trash Toolbar -->
    <div class="trash-toolbar-wrapper">
      <div class="trash-toolbar">
        <div class="trash-toolbar-left">
          <i class="material-icons trash-title-icon">delete</i>
          <h2>{{ $t("trash.title") }}</h2>
        </div>

        <div class="trash-toolbar-right">
          <!-- Admin: show all users toggle -->
          <div v-if="isAdmin" class="admin-toggle">
            <label class="toggle-switch" for="trash-all-users">
              <input
                id="trash-all-users"
                v-model="showAllUsers"
                type="checkbox"
                @change="loadTrash"
              />
              <span class="toggle-slider"></span>
            </label>
            <label for="trash-all-users" class="toggle-label">{{ $t("trash.allUsers") }}</label>
          </div>

          <!-- Bulk actions (always in DOM to prevent layout shift) -->
          <div class="trash-bulk-actions" :class="{ 'bulk-hidden': selectedIds.size === 0 }">
            <span class="selection-badge">
              {{ $t("trash.itemsSelected", { count: selectedIds.size }) }}
            </span>
            <button
              class="action trash-action-btn"
              :title="$t('trash.restore')"
              @click="restoreSelected"
            >
              <i class="material-icons">restore_from_trash</i>
            </button>
            <button
              class="action trash-action-btn trash-action-btn--danger"
              :title="$t('trash.deletePermanently')"
              @click="confirmPermanentDelete"
            >
              <i class="material-icons">delete_forever</i>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="trash-content-area">
      <h2 class="message delayed">
        <LoadingSpinner size="medium" />
        <span>{{ $t("general.loading", { suffix: "..." }) }}</span>
      </h2>
    </div>

    <!-- Empty state -->
    <div v-else-if="trashItems.length === 0" class="trash-content-area">
      <div class="trash-empty">
        <i class="material-icons trash-empty-icon">delete_outline</i>
        <h2>{{ $t("trash.empty") }}</h2>
        <p>{{ $t("trash.emptyDescription") || "Items you delete will appear here." }}</p>
      </div>
    </div>

    <!-- Items listing - uses same structure as ListingView -->
    <div v-else class="trash-content-area">
      <div
        ref="listingView"
        :class="{
          [listingViewMode]: true,
          'rectangle-selecting': isRectangleSelecting,
        }"
        :style="itemStyles"
        class="listing-items file-icons"
        @contextmenu="openContextOnEmpty"
      >
        <!-- Rectangle selection overlay -->
        <div class="selection-rectangle"
          :style="rectangleStyle"
        ></div>
        <!-- Directories Section -->
        <div v-if="numDirs > 0">
          <h2 :class="{'dark-mode': isDarkMode}">{{ $t("general.folders") }}</h2>
        </div>
        <div
          v-if="numDirs > 0"
          class="folder-items"
          aria-label="Trash Folder Items"
          :class="{ lastGroup: numFiles === 0 }"
        >
          <item
            v-for="item in dirs"
            :key="item.trashId"
            v-bind:index="item.index"
            v-bind:name="item.name"
            v-bind:isDir="true"
            v-bind:source="item.sourceName"
            v-bind:modified="item.deletedAt"
            v-bind:type="'directory'"
            v-bind:size="item.fileSize"
            v-bind:path="item.originalPath"
            v-bind:reducedOpacity="false"
            v-bind:hasPreview="false"
            v-bind:hasDuration="false"
            v-bind:isShared="false"
            v-bind:updateGlobalState="false"
            v-bind:isSelectedProp="selectedIds.has(item.trashId)"
            v-bind:clickable="false"
            v-bind:disableContextMenu="true"
            @select="handleItemSelect($event, item)"
            @selectRange="handleSelectRange($event)"
            @contextmenu.prevent.stop="openTrashContextMenu($event, item)"
          />
        </div>

        <!-- Files Section -->
        <div v-if="numFiles > 0">
          <h2 :class="{'dark-mode': isDarkMode}">{{ $t("general.files") }}</h2>
        </div>
        <div
          v-if="numFiles > 0"
          class="file-items"
          :class="{ lastGroup: numFiles > 0 }"
          aria-label="Trash File Items"
        >
          <item
            v-for="item in files"
            :key="item.trashId"
            v-bind:index="item.index"
            v-bind:name="item.name"
            v-bind:isDir="false"
            v-bind:source="item.sourceName"
            v-bind:modified="item.deletedAt"
            v-bind:type="item.type"
            v-bind:size="item.fileSize"
            v-bind:path="item.originalPath"
            v-bind:reducedOpacity="false"
            v-bind:hasPreview="false"
            v-bind:hasDuration="false"
            v-bind:isShared="false"
            v-bind:updateGlobalState="false"
            v-bind:isSelectedProp="selectedIds.has(item.trashId)"
            v-bind:clickable="false"
            v-bind:disableContextMenu="true"
            @select="handleItemSelect($event, item)"
            @selectRange="handleSelectRange($event)"
            @contextmenu.prevent.stop="openTrashContextMenu($event, item)"
          />
        </div>
      </div>
    </div>

    <!-- Trash Context Menu -->
    <Teleport to="body">
      <transition
        name="trash-ctx"
        @before-enter="ctxBeforeEnter"
        @enter="ctxEnter"
        @leave="ctxLeave"
      >
        <div
          v-if="showTrashContextMenu"
          ref="trashCtxMenu"
          class="trash-context-menu no-select floating-window"
          :class="{ 'dark-mode': isDarkMode, 'centered': ctxCentered }"
          :style="ctxCentered ? {} : { top: ctxY + 'px', left: ctxX + 'px' }"
          @click.stop
        >
          <div class="context-menu-header" v-if="selectedIds.size > 0">
            <div class="selected-count-header">
              <span>{{ selectedIds.size }}</span>
            </div>
          </div>
          <hr class="divider">

          <div class="action clickable" @click="ctxRestore">
            <i class="material-icons">restore_from_trash</i>
            <span>{{ $t('trash.restore') }}</span>
          </div>
          <div class="action clickable" @click="ctxShowInfo" v-if="selectedIds.size === 1">
            <i class="material-icons">info</i>
            <span>{{ $t('general.info') }}</span>
          </div>
          <div class="action clickable" @click="ctxSelectAll">
            <i class="material-icons">select_all</i>
            <span>{{ $t('buttons.selectAll') }}</span>
          </div>
          <div class="action clickable trash-ctx-danger" @click="ctxDeletePermanently">
            <i class="material-icons">delete_forever</i>
            <span>{{ $t('trash.deletePermanently') }}</span>
          </div>
        </div>
      </transition>
    </Teleport>

    <!-- Click overlay to close context menu -->
    <Teleport to="body">
      <div
        v-if="showTrashContextMenu"
        class="trash-ctx-overlay"
        @click="closeTrashContextMenu"
        @contextmenu.prevent="closeTrashContextMenu"
      ></div>
    </Teleport>

    <!-- Trash Info Dialog -->
    <Teleport to="body">
      <div v-if="infoDialog" class="trash-overlay" :class="{ 'trash-dialog-dark': isDarkMode }" @click.self="infoDialog = null">
        <div class="trash-confirm-card trash-info-card">
          <div class="trash-confirm-header">
            <i class="material-icons" style="color: var(--primaryColor);">info</i>
            <h3>{{ $t("general.info") }}</h3>
          </div>
          <div class="trash-info-body">
            <div class="trash-info-row">
              <span class="trash-info-label">{{ $t("trash.name") }}</span>
              <span class="trash-info-value">{{ infoDialog.name }}</span>
            </div>
            <div class="trash-info-row">
              <span class="trash-info-label">{{ $t("trash.originalPath") }}</span>
              <code class="trash-info-value path-text">{{ infoDialog.originalPath }}</code>
            </div>
            <div class="trash-info-row" v-if="infoDialog.username">
              <span class="trash-info-label">{{ $t("trash.owner") }}</span>
              <span class="trash-info-value owner-badge">{{ infoDialog.username }}</span>
            </div>
            <div class="trash-info-row">
              <span class="trash-info-label">{{ $t("trash.deletedAt") }}</span>
              <span class="trash-info-value">{{ formatDate(infoDialog.deletedAt) }}</span>
            </div>
            <div class="trash-info-row">
              <span class="trash-info-label">{{ $t("trash.expiresAt") }}</span>
              <span class="trash-info-value" :class="{ 'expires-soon': isExpiringSoon(infoDialog.expiresAt) }">
                {{ formatDate(infoDialog.expiresAt) }}
              </span>
            </div>
          </div>
          <div class="trash-confirm-actions">
            <button class="button button--flat" @click="infoDialog = null">
              {{ $t("general.close") || "Close" }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Confirm delete dialog -->
    <Teleport to="body">
      <div v-if="confirmDialog" class="trash-overlay" :class="{ 'trash-dialog-dark': isDarkMode }" @click.self="confirmDialog = null">
        <div class="trash-confirm-card">
          <div class="trash-confirm-header">
            <i class="material-icons trash-confirm-icon">warning</i>
            <h3>{{ $t("trash.confirmDeleteTitle") }}</h3>
          </div>
          <div class="trash-confirm-body">
            <p v-if="confirmDialog.ids.length === 1">
              {{ $t("trash.confirmDeleteSingle") }}
            </p>
            <p v-else>
              {{ $t("trash.confirmDeleteMultiple", { count: confirmDialog.ids.length }) }}
            </p>
          </div>
          <div class="trash-confirm-actions">
            <button class="button button--flat" @click="confirmDialog = null">
              {{ $t("general.cancel") }}
            </button>
            <button class="button button--red" @click="executeDelete">
              <i class="material-icons" style="font-size: 1.1em; margin-right: 0.3em;">delete_forever</i>
              {{ $t("trash.deletePermanently") }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Trash Status Bar -->
    <div
      id="trash-status-bar"
      :style="trashStatusBarStyle"
      :class="{ 'dark-mode-header': isDarkMode, active: trashItems.length > 0 }"
    >
      <div class="status-content">
        <div v-if="selectedIds.size > 0" class="status-info">
          <span class="button">{{ selectedIds.size }}</span>
          <span>{{ selectedItemsText }}</span>
        </div>
        <div v-else class="status-info">
          <span class="directory-info">{{ trashDirectoryInfoText }}</span>
        </div>
        <div class="status-controls">
          <div v-if="isCardView" class="gallery-size-control">
            <span class="size-label">{{ $t("general.size") }}</span>
            <input
              v-model="gallerySizeLocal"
              type="range"
              min="1"
              max="9"
              @input="updateGallerySize"
              @change="commitGallerySize"
            />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { listTrash, restoreTrash, permanentDelete } from '@/api/trash.js'
import { notify } from '@/notify'
import { getters, mutations, state } from '@/store'
import { getTypeInfo } from '@/utils/mimetype'
import { getHumanReadableFilesize } from '@/utils/filesizes'
import Item from '@/components/files/ListingItem.vue'
import LoadingSpinner from '@/components/LoadingSpinner.vue'

export default {
  name: 'Trash',
  components: {
    Item,
    LoadingSpinner,
  },

  data() {
    return {
      trashItems: [],
      loading: false,
      showAllUsers: false,
      selectedIds: new Set(),
      confirmDialog: null,
      infoDialog: null,
      lastSelectedId: null,
      // Context menu
      showTrashContextMenu: false,
      ctxX: 0,
      ctxY: 0,
      ctxItem: null,
      // Rectangle selection
      isRectangleSelecting: false,
      rectangleStart: { x: 0, y: 0 },
      rectangleEnd: { x: 0, y: 0 },
      rafId: null,
      selectionUpdatePending: false,
      initialSelectionState: new Set(),
      gallerySizeLocal: state.user.gallerySize,
    }
  },

  computed: {
    isAdmin() {
      return getters.isAdmin()
    },
    isDarkMode() {
      return getters.isDarkMode()
    },
    isMobile() {
      return state.isMobile
    },
    ctxCentered() {
      return this.isMobile
    },
    listingViewMode() {
      return getters.viewMode()
    },
    rectangleStyle() {
      if (!this.isRectangleSelecting) return { display: 'none' }
      const left = Math.min(this.rectangleStart.x, this.rectangleEnd.x)
      const top = Math.min(this.rectangleStart.y, this.rectangleEnd.y)
      const width = Math.abs(this.rectangleStart.x - this.rectangleEnd.x)
      const height = Math.abs(this.rectangleStart.y - this.rectangleEnd.y)
      return {
        left: left + 'px',
        top: top + 'px',
        width: width + 'px',
        height: height + 'px',
      }
    },
    numDirs() {
      return this.trashItems.filter(i => i.isDir).length
    },
    numFiles() {
      return this.trashItems.filter(i => !i.isDir).length
    },
    dirs() {
      return this.trashItems.filter(i => i.isDir)
    },
    files() {
      return this.trashItems.filter(i => !i.isDir)
    },
    numColumns() {
      if (!getters.isCardView()) {
        return 1
      }
      const elem = document.querySelector('#main')
      if (!elem) {
        return 1
      }
      const columnWidth = 250 + state.user.gallerySize * 50
      if (getters.viewMode() === 'icons') {
        const containerSize = 70 + (state.user.gallerySize * 15)
        let columns = Math.floor(elem.offsetWidth / containerSize)
        if (columns === 0) columns = 1
        const minColumns = 3
        const maxColumns = 12
        columns = Math.max(minColumns, Math.min(columns, maxColumns))
        return columns
      }
      let columns = Math.floor(elem.offsetWidth / columnWidth)
      if (columns === 0) columns = 1
      return columns
    },
    itemStyles() {
      const viewMode = getters.viewMode()
      const styles = {}
      const size = state.user.gallerySize

      if (viewMode === 'icons') {
        const baseSize = 20 + (size * 15)
        const cellSize = baseSize + 30
        styles['--icon-size'] = `${baseSize}px`
        styles['--icon-font-size'] = `${baseSize}px`
        styles['--icons-view-cell-size'] = `${cellSize}px`
      } else if (viewMode === 'gallery') {
        const baseCalc = 80 + (size * 25)
        const extraScaling = Math.max(0, size - 5) * 15
        const baseSize = baseCalc + extraScaling
        const iconFontSize = (3 + (size * 0.5)).toFixed(2)
        styles['--icon-font-size'] = `${iconFontSize}em`

        if (state.isMobile) {
          const minWidth = size <= 3 ? 120 : size <= 7 ? 160 : 280
          const mobileHeight = 120 + (size * 20)
          styles['--gallery-mobile-min-width'] = `${minWidth}px`
          styles['--item-width'] = `${minWidth}px`
          styles['--item-height'] = `${mobileHeight}px`
        } else {
          styles['--item-width'] = `${baseSize}px`
          styles['--item-height'] = `${Math.round(baseSize * 1.2)}px`
        }
      } else if (viewMode === 'list' || viewMode === 'compact') {
        const baseHeight = viewMode === 'compact'
          ? 40 + (size * 2)
          : 50 + (size * 3)
        const iconSize = (2 + (size * 0.12)).toFixed(2)
        const iconFontSize = (1.5 + (size * 0.12)).toFixed(2)

        styles['--item-width'] = `calc(${(100 / this.numColumns).toFixed(2)}% - 1em)`
        styles['--item-height'] = `${baseHeight}px`
        styles['--icon-size'] = `${iconSize}em`
        styles['--icon-font-size'] = `${iconFontSize}em`
      } else {
        const iconSize = (3.2 + (size * 0.15)).toFixed(2)
        const iconFontSize = (2.2 + (size * 0.12)).toFixed(2)

        styles['--item-width'] = `calc(${(100 / this.numColumns)}% - 1em)`
        styles['--item-height'] = 'auto'
        styles['--icon-size'] = `${iconSize}em`
        styles['--icon-font-size'] = `${iconFontSize}em`
      }

      return styles
    },
    isCardView() {
      return getters.isCardView()
    },
    selectedItemsLabel() {
      return this.selectedIds.size === 1
        ? this.$t('files.itemSelected')
        : this.$t('files.itemsSelected')
    },
    totalSelectedSize() {
      if (this.selectedIds.size === 0) return 0
      let total = 0
      this.trashItems.forEach(item => {
        if (this.selectedIds.has(item.trashId) && item.fileSize) {
          total += item.fileSize
        }
      })
      return total
    },
    totalTrashSize() {
      return this.trashItems.reduce((total, item) => total + (item.fileSize || 0), 0)
    },
    displayTotalSize() {
      const size = this.selectedIds.size > 0 ? this.totalSelectedSize : this.totalTrashSize
      return getHumanReadableFilesize(size)
    },
    selectedItemsText() {
      return `${this.selectedItemsLabel} (${this.displayTotalSize})`
    },
    foldersLabel() {
      return this.numDirs === 1
        ? this.$t('general.folder')
        : this.$t('general.folders')
    },
    filesLabel() {
      return this.numFiles === 1
        ? this.$t('general.file')
        : this.$t('general.files')
    },
    trashDirectoryInfoText() {
      return `${this.numDirs} ${this.foldersLabel} | ${this.numFiles} ${this.filesLabel} (${this.displayTotalSize})`
    },
    trashStatusBarStyle() {
      if (getters.isStickySidebar() && getters.isSidebarVisible()) {
        return { left: state.sidebar.width + 'em' }
      }
      return {}
    },
  },

  mounted() {
    this.loadTrash()
    window.addEventListener('keydown', this.handleKeydown)
    window.addEventListener('click', this.handleGlobalClick)
    document.addEventListener('mousemove', this.updateRectangleSelection, { passive: true })
    document.addEventListener('mouseup', this.endRectangleSelection)
    this.$el.addEventListener('mousedown', this.startRectangleSelection)
  },

  beforeUnmount() {
    window.removeEventListener('keydown', this.handleKeydown)
    window.removeEventListener('click', this.handleGlobalClick)
    document.removeEventListener('mousemove', this.updateRectangleSelection)
    document.removeEventListener('mouseup', this.endRectangleSelection)
    this.$el.removeEventListener('mousedown', this.startRectangleSelection)
    if (this.rafId) {
      cancelAnimationFrame(this.rafId)
      this.rafId = null
    }
  },

  methods: {
    async loadTrash() {
      this.loading = true
      this.selectedIds = new Set()
      try {
        const rawItems = await listTrash(this.showAllUsers)
        // Map trash items to have proper props for ListingItem
        this.trashItems = rawItems.map((item, index) => {
          const name = this.extractName(item.originalPath)
          const ext = name.includes('.') ? name.split('.').pop().toLowerCase() : ''
          let type = 'directory'
          if (!item.isDir) {
            // Try to determine the mimetype from extension
            type = this.getMimeFromExt(ext) || 'file'
          }
          return {
            ...item,
            trashId: item.id,
            name: name,
            type: type,
            index: index,
            fileSize: item.size || 0,
            sourceName: item.sourceName,
          }
        })
      } catch {
        // error shown by api
      } finally {
        this.loading = false
      }
    },

    extractName(originalPath) {
      const parts = (originalPath || '').split('/').filter(Boolean)
      return parts[parts.length - 1] || originalPath
    },

    getMimeFromExt(ext) {
      if (!ext) return 'file'
      const info = getTypeInfo(ext)
      if (info && info.type) return info.type
      // Common fallbacks
      const mimeMap = {
        jpg: 'image/jpeg', jpeg: 'image/jpeg', png: 'image/png', gif: 'image/gif',
        webp: 'image/webp', svg: 'image/svg+xml', bmp: 'image/bmp',
        mp4: 'video/mp4', webm: 'video/webm', mkv: 'video/x-matroska', avi: 'video/x-msvideo', mov: 'video/quicktime',
        mp3: 'audio/mpeg', wav: 'audio/wav', ogg: 'audio/ogg', flac: 'audio/flac',
        pdf: 'application/pdf', zip: 'application/zip', tar: 'application/x-tar',
        doc: 'application/msword', docx: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
        xls: 'application/vnd.ms-excel', xlsx: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
        ppt: 'application/vnd.ms-powerpoint', pptx: 'application/vnd.openxmlformats-officedocument.presentationml.presentation',
        txt: 'text/plain', html: 'text/html', css: 'text/css', js: 'application/javascript',
        json: 'application/json', xml: 'application/xml', md: 'text/markdown',
      }
      return mimeMap[ext] || 'file'
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

    // ── Selection Logic (mimics ListingView) ──

    handleItemSelect(event, item) {
      const mouseEvent = event.originalEvent || {}
      const shiftKey = mouseEvent.shiftKey
      const ctrlOrMeta = mouseEvent.ctrlKey || mouseEvent.metaKey

      if (shiftKey && this.lastSelectedId !== null) {
        // Shift+click: select range
        const allItems = [...this.dirs, ...this.files]
        const lastIdx = allItems.findIndex(i => i.trashId === this.lastSelectedId)
        const currentIdx = allItems.findIndex(i => i.trashId === item.trashId)
        if (lastIdx !== -1 && currentIdx !== -1) {
          const start = Math.min(lastIdx, currentIdx)
          const end = Math.max(lastIdx, currentIdx)
          if (!ctrlOrMeta) {
            this.selectedIds = new Set()
          }
          const next = new Set(this.selectedIds)
          for (let i = start; i <= end; i++) {
            next.add(allItems[i].trashId)
          }
          this.selectedIds = next
        }
        return
      }

      if (ctrlOrMeta) {
        // Ctrl/Cmd+click: toggle this item
        const next = new Set(this.selectedIds)
        if (next.has(item.trashId)) {
          next.delete(item.trashId)
        } else {
          next.add(item.trashId)
        }
        this.selectedIds = next
        this.lastSelectedId = item.trashId
        return
      }

      // Simple click: select only this item
      const next = new Set()
      if (!this.selectedIds.has(item.trashId) || this.selectedIds.size > 1) {
        next.add(item.trashId)
      }
      this.selectedIds = next
      this.lastSelectedId = item.trashId
    },

    selectAll() {
      this.selectedIds = new Set(this.trashItems.map(i => i.trashId))
    },

    handleSelectRange(event) {
      // event has { startIndex, endIndex } where index is the item.index (array position)
      const allItems = [...this.dirs, ...this.files]
      const start = Math.min(event.startIndex, event.endIndex)
      const end = Math.max(event.startIndex, event.endIndex)
      const next = new Set()
      for (let i = start; i <= end; i++) {
        if (allItems[i]) {
          next.add(allItems[i].trashId)
        }
      }
      this.selectedIds = next
      if (allItems[event.endIndex]) {
        this.lastSelectedId = allItems[event.endIndex].trashId
      }
    },

    // ── Rectangle (lasso) drag-to-select ──

    startRectangleSelection(event) {
      // Only start on empty space, not on items, headers, toolbar, or status bar
      if (event.target.closest('.listing-item') || event.target.closest('.trash-toolbar') || event.target.closest('#status-bar') || event.target.closest('h2')) {
        return
      }
      if (event.button !== 0) return // left click only

      this.isRectangleSelecting = true

      const listingRect = this.$refs.listingView.getBoundingClientRect()
      this.rectangleStart = {
        x: event.clientX - listingRect.left,
        y: event.clientY - listingRect.top,
      }
      this.rectangleEnd = {
        x: event.clientX - listingRect.left,
        y: event.clientY - listingRect.top,
      }

      // Store current selection when starting rectangle (for additive mode)
      this.initialSelectionState = new Set(this.selectedIds)

      // Only clear selection when Ctrl/Cmd is NOT held
      const hasModifier = event.ctrlKey || event.metaKey
      if (!hasModifier) {
        this.selectedIds = new Set()
      }

      event.preventDefault()
    },

    updateRectangleSelection(event) {
      if (!this.isRectangleSelecting) return

      const listingRect = this.$refs.listingView.getBoundingClientRect()
      this.rectangleEnd = {
        x: event.clientX - listingRect.left,
        y: event.clientY - listingRect.top,
      }

      if (!this.selectionUpdatePending) {
        this.selectionUpdatePending = true
        this.rafId = requestAnimationFrame(() => {
          this.updateSelectedItemsInRectangle(event.ctrlKey || event.metaKey)
          this.selectionUpdatePending = false
        })
      }
    },

    endRectangleSelection(event) {
      if (!this.isRectangleSelecting) return

      if (this.rafId) {
        cancelAnimationFrame(this.rafId)
        this.rafId = null
      }

      this.isRectangleSelecting = false
      this.selectionUpdatePending = false
      this.updateSelectedItemsInRectangle(event.ctrlKey || event.metaKey)

      setTimeout(() => {
        this.rectangleStart = { x: 0, y: 0 }
        this.rectangleEnd = { x: 0, y: 0 }
        this.initialSelectionState = new Set()
      }, 100)
    },

    updateSelectedItemsInRectangle(isAdditive) {
      if (!this.isRectangleSelecting) return

      const listingRect = this.$refs.listingView.getBoundingClientRect()
      const rect = {
        left: Math.min(this.rectangleStart.x, this.rectangleEnd.x),
        top: Math.min(this.rectangleStart.y, this.rectangleEnd.y),
        right: Math.max(this.rectangleStart.x, this.rectangleEnd.x),
        bottom: Math.max(this.rectangleStart.y, this.rectangleEnd.y),
      }

      const allItems = [...this.dirs, ...this.files]
      const rectangleSelectedIds = new Set()

      // Query all listing-item elements with data-index
      const itemElements = this.$refs.listingView.querySelectorAll('.listing-item[data-index]')

      itemElements.forEach((element) => {
        const elementRect = element.getBoundingClientRect()

        // Convert to relative coordinates
        const elementRelativeRect = {
          left: elementRect.left - listingRect.left,
          top: elementRect.top - listingRect.top,
          right: elementRect.right - listingRect.left,
          bottom: elementRect.bottom - listingRect.top,
        }

        // Check intersection
        if (
          elementRelativeRect.left < rect.right &&
          elementRelativeRect.right > rect.left &&
          elementRelativeRect.top < rect.bottom &&
          elementRelativeRect.bottom > rect.top
        ) {
          const index = parseInt(element.getAttribute('data-index'))
          if (!isNaN(index) && allItems[index]) {
            rectangleSelectedIds.add(allItems[index].trashId)
          }
        }
      })

      if (isAdditive) {
        // Combine initial selection + new rectangle selection
        const combined = new Set(this.initialSelectionState)
        rectangleSelectedIds.forEach(id => combined.add(id))
        this.selectedIds = combined
      } else {
        // Only items inside the rectangle
        this.selectedIds = rectangleSelectedIds
      }
    },

    handleKeydown(event) {
      // Escape to clear selection
      if (event.key === 'Escape') {
        this.selectedIds = new Set()
        this.closeTrashContextMenu()
        return
      }

      // Ctrl/Cmd+A to select all
      if ((event.ctrlKey || event.metaKey) && event.key === 'a') {
        event.preventDefault()
        this.selectAll()
        return
      }

      // Delete key
      if (event.key === 'Delete' && this.selectedIds.size > 0) {
        this.confirmPermanentDelete()
      }
    },

    handleGlobalClick() {
      // Close context menu on outside click
      if (this.showTrashContextMenu) {
        this.closeTrashContextMenu()
      }
    },

    // ── Context Menu ──

    openTrashContextMenu(event, item) {
      // Ensure item is selected
      if (!this.selectedIds.has(item.trashId)) {
        if (this.selectedIds.size <= 1) {
          this.selectedIds = new Set([item.trashId])
          this.lastSelectedId = item.trashId
        }
      }
      this.ctxItem = item
      this.ctxX = event.clientX
      this.ctxY = event.clientY
      this.showTrashContextMenu = true
    },

    openContextOnEmpty(event) {
      event.preventDefault()
      if (this.trashItems.length === 0) return
      this.ctxItem = null
      this.ctxX = event.clientX
      this.ctxY = event.clientY
      this.showTrashContextMenu = true
    },

    closeTrashContextMenu() {
      this.showTrashContextMenu = false
      this.ctxItem = null
    },

    async ctxRestore() {
      this.closeTrashContextMenu()
      if (this.selectedIds.size > 0) {
        await this.restoreSelected()
      }
    },

    ctxShowInfo() {
      this.closeTrashContextMenu()
      if (this.selectedIds.size === 1) {
        const id = Array.from(this.selectedIds)[0]
        const item = this.trashItems.find(i => i.trashId === id)
        if (item) {
          this.infoDialog = item
        }
      }
    },

    ctxSelectAll() {
      this.closeTrashContextMenu()
      this.selectAll()
    },

    ctxDeletePermanently() {
      this.closeTrashContextMenu()
      if (this.selectedIds.size > 0) {
        this.confirmPermanentDelete()
      }
    },

    // Context menu transition
    ctxBeforeEnter(el) {
      el.style.height = '0'
      el.style.opacity = '0'
    },
    ctxEnter(el, done) {
      el.style.transition = ''
      el.style.height = '0'
      el.style.opacity = '0'
      void el.offsetHeight
      this.$nextTick(() => {
        el.style.height = 'auto'
        el.style.visibility = 'hidden'
        void el.offsetHeight
        const fullHeight = el.scrollHeight
        const fullWidth = el.scrollWidth

        const BUFFER = 8
        const screenWidth = window.innerWidth
        const screenHeight = window.innerHeight
        let newX = this.ctxX
        let newY = this.ctxY
        if (newX + fullWidth + BUFFER > screenWidth) newX = screenWidth - fullWidth - BUFFER
        if (newX < BUFFER) newX = BUFFER
        if (newY + fullHeight + BUFFER > screenHeight) newY = screenHeight - fullHeight - BUFFER
        if (newY < BUFFER) newY = BUFFER
        this.ctxX = newX
        this.ctxY = newY

        el.style.height = '0'
        el.style.visibility = 'visible'
        el.style.transition = 'height 0.3s, opacity 0.3s'
        void el.offsetHeight
        el.style.height = fullHeight + 'px'
        el.style.opacity = '1'
        setTimeout(done, 300)
      })
    },
    ctxLeave(el, done) {
      el.style.transition = 'height 0.3s, opacity 0.3s'
      el.style.height = el.scrollHeight + 'px'
      void el.offsetHeight
      el.style.height = '0'
      el.style.opacity = '0'
      setTimeout(done, 300)
    },

    // ── Trash Actions ──

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

    // ── Gallery Size Control (status bar) ──

    updateGallerySize(event) {
      this.gallerySizeLocal = parseInt(event.target.value, 10)
    },

    commitGallerySize() {
      mutations.setGallerySize(this.gallerySizeLocal)
    },
  },
}
</script>

<style scoped>
/* ── Page Layout ──────────────────────────────── */
.trash-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  width: 100%;
}

/* ── Toolbar ──────────────────────────────────── */
.trash-toolbar-wrapper {
  position: sticky;
  top: 0;
  z-index: 100;
  background: var(--surfacePrimary);
  border-bottom: 1px solid var(--divider);
  padding: 0.75em 1em;
  overflow: hidden;
}

.trash-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  height: 2.5em;
}

.trash-toolbar-left {
  display: flex;
  align-items: center;
  gap: 0.5em;
}

.trash-toolbar-left h2 {
  margin: 0;
  font-size: 1.1rem;
  font-weight: 600;
  color: var(--textPrimary);
}

.trash-title-icon {
  font-size: 1.3em;
  opacity: 0.7;
  color: var(--textSecondary);
}

.trash-toolbar-right {
  display: flex;
  align-items: center;
  gap: 1rem;
}

/* ── Toggle switch ──────────────────────────── */
.admin-toggle {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  user-select: none;
}

.toggle-switch {
  position: relative;
  display: inline-block;
  width: 36px;
  height: 20px;
  cursor: pointer;
}

.toggle-switch input {
  opacity: 0;
  width: 0;
  height: 0;
}

.toggle-slider {
  position: absolute;
  inset: 0;
  background-color: var(--surfaceSecondary);
  border-radius: 20px;
  transition: 0.3s ease all;
}

.toggle-slider::before {
  content: "";
  position: absolute;
  height: 14px;
  width: 14px;
  left: 3px;
  bottom: 3px;
  background-color: var(--textSecondary);
  border-radius: 50%;
  transition: 0.3s ease all;
}

.toggle-switch input:checked + .toggle-slider {
  background-color: var(--primaryColor);
}

.toggle-switch input:checked + .toggle-slider::before {
  transform: translateX(16px);
  background-color: white;
}

.toggle-label {
  font-size: 0.85rem;
  color: var(--textSecondary);
  cursor: pointer;
  transition: color 0.2s ease;
}

.toggle-label:hover {
  color: var(--textPrimary);
}

/* ── Bulk actions ───────────────────────────── */
.trash-bulk-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  transition: opacity 0.15s ease;
}

.trash-bulk-actions.bulk-hidden {
  visibility: hidden;
  opacity: 0;
  pointer-events: none;
}

.selection-badge {
  font-size: 0.8rem;
  color: var(--primaryColor);
  background: rgba(33, 150, 243, 0.1);
  padding: 0.25em 0.75em;
  border-radius: 1em;
  font-weight: 500;
}

/* ── Action buttons ─────────────────────────── */
.trash-action-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 0.6em;
  padding: 0.35em;
  transition: 0.2s ease all;
  background: transparent;
  color: var(--textSecondary);
}

.trash-action-btn i {
  font-size: 1.2rem;
}

.trash-action-btn:hover {
  background: var(--surfaceSecondary);
  color: var(--primaryColor);
}

.trash-action-btn--danger:hover {
  color: var(--red);
}

/* ── Content Area ────────────────────────────── */
.trash-content-area {
  flex: 1;
  padding: 1em;
  overflow-y: auto;
}

/* ── Empty state ─────────────────────────────── */
.trash-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 5rem 2rem;
  text-align: center;
}

.trash-empty-icon {
  font-size: 5rem;
  color: var(--textSecondary);
  opacity: 0.3;
  margin-bottom: 0.5rem;
}

.trash-empty h2 {
  color: var(--textPrimary);
  font-weight: 500;
  margin: 0.25em 0;
  opacity: 0.8;
}

.trash-empty p {
  color: var(--textSecondary);
  font-size: 0.9rem;
  margin: 0;
}

/* ── Listing items overrides ─────────────────── */
.trash-content-area .listing-items {
  min-height: auto !important;
  padding-bottom: 2em;
  position: relative;
}

/* ── Context menu: trash-specific danger action ── */
.trash-ctx-danger {
  color: var(--red, #d32f2f) !important;
}

.trash-ctx-danger:hover {
  background: rgba(211, 47, 47, 0.1) !important;
}

/* ── Context menu overlay ────────────────────── */
.trash-ctx-overlay {
  position: fixed;
  inset: 0;
  z-index: 9998;
}

/* ── Trash info card ─────────────────────────── */
.trash-info-body {
  padding: 0.5em 1.5em 1em;
}

.trash-info-row {
  display: flex;
  flex-direction: column;
  gap: 0.2em;
  padding: 0.5em 0;
  border-bottom: 1px solid var(--divider);
}

.trash-info-row:last-child {
  border-bottom: none;
}

.trash-info-label {
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--textSecondary);
  font-weight: 600;
}

.trash-info-value {
  color: var(--textPrimary);
  font-size: 0.9rem;
  word-break: break-all;
}

.path-text {
  font-size: 0.8rem;
  color: var(--textSecondary);
  background: rgba(128, 128, 128, 0.08);
  padding: 0.15em 0.4em;
  border-radius: 0.3em;
  font-family: monospace;
  border: none;
}

.owner-badge {
  font-size: 0.8rem;
  background: rgba(33, 150, 243, 0.1);
  color: var(--primaryColor);
  padding: 0.15em 0.5em;
  border-radius: 0.5em;
  font-weight: 500;
  display: inline-block;
}

.expires-soon {
  color: var(--icon-orange);
  font-weight: 600;
}

/* ── Responsive ─────────────────────────────── */
@media (max-width: 700px) {
  .trash-toolbar {
    flex-direction: column;
    align-items: flex-start;
  }

  .trash-toolbar-right {
    width: 100%;
    justify-content: space-between;
  }
}

/* ── Trash Status Bar ──────────────────────────── */
#trash-status-bar {
  background-color: rgb(37 49 55 / 5%) !important;
  height: 2.5em;
  display: flex;
  align-items: center;
  position: fixed;
  bottom: -2.5em;
  left: 0;
  right: 0;
  z-index: 2;
  border-radius: 2px;
  overflow: hidden;
  margin: 0;
  padding: 0;
  transition: bottom 0.5s ease, left 0.2s ease, width 0.2s ease;
  pointer-events: none;
}

#trash-status-bar.active {
  bottom: 0;
  pointer-events: auto;
}

#trash-status-bar .status-content {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 0 1em;
  height: 100%;
  font-size: 0.85em;
}

#trash-status-bar .status-info {
  display: flex;
  align-items: center;
  color: var(--textSecondary);
  font-weight: 500;
  gap: 0.25em;
}

#trash-status-bar .button {
  padding: 0 0.5em;
  font-size: 0.9em;
  font-weight: bold;
  cursor: unset;
}

#trash-status-bar .status-controls {
  display: flex;
  align-items: center;
  gap: 1.5em;
}

#trash-status-bar .gallery-size-control {
  display: flex;
  align-items: center;
  gap: 0.5em;
}

#trash-status-bar .size-label {
  color: var(--textSecondary);
  font-size: 0.875em;
  white-space: nowrap;
}

#trash-status-bar input[type="range"] {
  accent-color: var(--primaryColor);
  width: 8em;
}

@supports (backdrop-filter: none) {
  #trash-status-bar {
    backdrop-filter: blur(16px) invert(0.1);
  }
  #trash-status-bar.dark-mode-header {
    background-color: rgb(37 49 55 / 33%) !important;
  }
}

@media (max-width: 768px) {
  #trash-status-bar {
    height: 3em;
    bottom: -3em;
    font-size: 0.9em;
    box-shadow: 0 -2px 10px rgba(0, 0, 0, 0.1);
  }

  #trash-status-bar.active {
    bottom: 0;
    pointer-events: auto;
  }

  #trash-status-bar .status-content {
    padding: 0 0.8em;
  }

  #trash-status-bar .status-info {
    font-size: 1em;
  }

  #trash-status-bar .size-label {
    font-size: 0.9em;
  }

  #trash-status-bar input[type="range"] {
    width: 7em;
  }
}
</style>

<style>
/* ── Trash Confirm Dialog ── */
.trash-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.55);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999;
  animation: trashOverlayFadeIn 0.2s ease;
  backdrop-filter: blur(2px);
}

@keyframes trashOverlayFadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.trash-confirm-card {
  max-width: 460px;
  width: 90%;
  background: var(--surfacePrimary);
  border: 1px solid var(--divider);
  border-radius: 1em;
  box-shadow: 0 16px 48px rgba(0, 0, 0, 0.3), 0 4px 12px rgba(0, 0, 0, 0.15);
  animation: trashCardSlideUp 0.25s ease;
  overflow: hidden;
}

@keyframes trashCardSlideUp {
  from {
    opacity: 0;
    transform: translateY(24px) scale(0.96);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

.trash-confirm-header {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  padding: 1.25em 1.5em 0.75em;
}

.trash-confirm-header h3 {
  margin: 0;
  font-size: 1.15rem;
  font-weight: 600;
  color: var(--textPrimary);
}

.trash-confirm-icon {
  color: #ff9800;
  font-size: 1.5rem;
}

.trash-confirm-body {
  padding: 0 1.5em 1em;
}

.trash-confirm-body p {
  margin: 0;
  color: var(--textSecondary);
  line-height: 1.6;
  font-size: 0.9rem;
}

.trash-confirm-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.5rem;
  padding: 0.75em 1.25em;
  border-top: 1px solid var(--divider);
  background: rgba(128, 128, 128, 0.04);
}

.trash-confirm-actions .button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0.5em 1.25em;
  border-radius: 0.5em;
  font-size: 0.85rem;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
  text-transform: none;
  letter-spacing: 0;
}

.trash-confirm-actions .button--flat {
  background: transparent;
  border: 1px solid var(--divider);
  color: var(--textSecondary);
}

.trash-confirm-actions .button--flat:hover {
  background: rgba(128, 128, 128, 0.1);
  color: var(--textPrimary);
}

.trash-confirm-actions .button--red {
  background: #d32f2f;
  border: 1px solid #d32f2f;
  color: #fff;
}

.trash-confirm-actions .button--red:hover {
  background: #b71c1c;
  border-color: #b71c1c;
  box-shadow: 0 2px 8px rgba(211, 47, 47, 0.3);
}

/* Dark mode overrides for teleported dialog */
.trash-dialog-dark .trash-confirm-card {
  --surfacePrimary: #20292F;
  --surfaceSecondary: #3A4147;
  --divider: rgba(255, 255, 255, 0.12);
  --textPrimary: rgba(255, 255, 255, 0.87);
  --textSecondary: rgba(255, 255, 255, 0.6);
}

.trash-dialog-dark .trash-confirm-actions .button--flat {
  color: rgba(255, 255, 255, 0.6);
  border-color: rgba(255, 255, 255, 0.12);
}

.trash-dialog-dark .trash-confirm-actions .button--flat:hover {
  color: rgba(255, 255, 255, 0.87);
  background: rgba(255, 255, 255, 0.08);
}

/* ── Trash context menu ── */
.trash-context-menu {
  position: fixed;
  z-index: 9999;
  min-width: 180px;
  max-width: 280px;
  border-radius: 0.75em;
  padding: 0.3em 0;
  overflow: hidden;
  border-width: 1px;
  display: flex;
  flex-direction: column;
}

.trash-context-menu.centered {
  position: fixed;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
}

.trash-context-menu .action {
  display: flex;
  align-items: center;
  gap: 0.5em;
  padding: 0.5em 1em;
  cursor: pointer;
  transition: background 0.15s ease;
  font-size: 0.9rem;
  color: var(--textPrimary);
}

.trash-context-menu .action:hover {
  background: var(--surfaceSecondary);
}

.trash-context-menu .action i {
  font-size: 1.2em;
  color: var(--textSecondary);
}

.trash-context-menu .divider {
  margin: 0.25em 0;
  border: none;
  border-top: 1px solid var(--divider);
}

.trash-context-menu .context-menu-header {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  padding: 0.3em 0.5em;
}

.trash-context-menu .selected-count-header {
  display: flex;
  align-items: center;
  justify-content: center;
  min-width: 1.5em;
  height: 1.5em;
  border-radius: 50%;
  background: var(--primaryColor);
  color: white;
  font-size: 0.8em;
  font-weight: 600;
}

/* ── Rectangle (lasso) selection ── */
.selection-rectangle {
  position: absolute;
  border: 2px solid var(--primaryColor);
  background-color: color-mix(in srgb, var(--primaryColor) 25%, transparent);
  border-radius: 8px;
  pointer-events: none;
  z-index: 10;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
}

.rectangle-selecting {
  cursor: crosshair;
  user-select: none;
}

.rectangle-selecting .listing-item {
  pointer-events: none;
}
</style>
