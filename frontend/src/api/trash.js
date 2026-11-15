import { fetchURL } from '@/api/utils.ts'
import { getApiPath } from '@/utils/url.js'

// Deleting files through the normal delete endpoints moves them to the trash;
// these calls list, restore and permanently delete trashed items.

/**
 * List trash items for the current user.
 * @param {boolean} allUsers - Admin only: list all users' trash items
 * @returns {Promise<Array>}
 */
export async function listTrash(allUsers = false) {
  const params = allUsers ? { all: 'true' } : {}
  const res = await fetchURL(getApiPath('trash', params))
  return await res.json()
}

// The backend answers 207 with { succeeded, failed } on partial failure;
// surface any failures as an error so the UI does not report success.
async function bulkTrashRequest(path, method, ids) {
  if (!Array.isArray(ids) || ids.length === 0) {
    throw new Error('ids array is required and must not be empty')
  }
  const res = await fetchURL(getApiPath(path), {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ids }),
  })
  const body = await res.json().catch(() => ({}))
  const failed = body.failed || []
  if (failed.length > 0) {
    const error = new Error(failed.map((f) => f.message).filter(Boolean).join('; ') || 'request failed')
    error.result = body
    throw error
  }
  return body
}

/**
 * Restore trash items back to their original location.
 * @param {string[]} ids
 */
export function restoreTrash(ids) {
  return bulkTrashRequest('trash/restore', 'POST', ids)
}

/**
 * Permanently delete trash items.
 * @param {string[]} ids
 */
export function permanentDelete(ids) {
  return bulkTrashRequest('trash', 'DELETE', ids)
}
