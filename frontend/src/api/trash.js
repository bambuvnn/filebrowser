import { fetchURL } from './utils'
import { getApiPath } from '@/utils/url.js'
import { notify } from '@/notify'

/**
 * List trash items for the current user.
 * @param {boolean} allUsers - Admin only: list all users' trash items
 * @returns {Promise<Array>}
 */
export async function listTrash(allUsers = false) {
  try {
    const params = allUsers ? { all: 'true' } : {}
    const apiPath = getApiPath('trash', params)
    const res = await fetchURL(apiPath)
    return await res.json()
  } catch (err) {
    notify.showError(err.message || 'Error fetching trash items')
    throw err
  }
}

/**
 * Move items to trash (soft delete).
 * @param {Array<{source: string, path: string}>} items
 * @returns {Promise}
 */
export async function moveToTrash(items) {
  if (!items || !Array.isArray(items) || items.length === 0) {
    throw new Error('items array is required and must not be empty')
  }
  try {
    const apiPath = getApiPath('trash/move')
    const response = await fetchURL(apiPath, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ items }),
    })
    return response
  } catch (err) {
    notify.showError(err.message || 'Error moving items to trash')
    throw err
  }
}

/**
 * Restore trash items back to their original location.
 * @param {string[]} ids - Array of trash item IDs to restore
 * @returns {Promise}
 */
export async function restoreTrash(ids) {
  if (!ids || !Array.isArray(ids) || ids.length === 0) {
    throw new Error('ids array is required and must not be empty')
  }
  try {
    const apiPath = getApiPath('trash/restore')
    const response = await fetchURL(apiPath, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ids }),
    })
    return response
  } catch (err) {
    notify.showError(err.message || 'Error restoring trash items')
    throw err
  }
}

/**
 * Permanently delete trash items.
 * @param {string[]} ids - Array of trash item IDs to permanently delete
 * @returns {Promise}
 */
export async function permanentDelete(ids) {
  if (!ids || !Array.isArray(ids) || ids.length === 0) {
    throw new Error('ids array is required and must not be empty')
  }
  try {
    const apiPath = getApiPath('trash')
    const response = await fetchURL(apiPath, {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ids }),
    })
    return response
  } catch (err) {
    notify.showError(err.message || 'Error permanently deleting trash items')
    throw err
  }
}
