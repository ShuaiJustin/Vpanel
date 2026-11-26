import request from '../base'

/**
 * Get current pause status
 * @returns {Promise} Pause status response
 */
export function getPauseStatus() {
  return request({
    url: '/subscription/pause',
    method: 'get'
  })
}

/**
 * Pause subscription
 * @returns {Promise} Pause result response
 */
export function pauseSubscription() {
  return request({
    url: '/subscription/pause',
    method: 'post'
  })
}

/**
 * Resume subscription
 * @returns {Promise} Resume result response
 */
export function resumeSubscription() {
  return request({
    url: '/subscription/resume',
    method: 'post'
  })
}

/**
 * Get pause history
 * @param {Object} params - Query parameters
 * @param {number} params.page - Page number
 * @param {number} params.page_size - Page size
 * @returns {Promise} Pause history response
 */
export function getPauseHistory(params) {
  return request({
    url: '/subscription/pause/history',
    method: 'get',
    params
  })
}

// Admin APIs

/**
 * Get pause statistics (admin only)
 * @returns {Promise} Pause statistics response
 */
export function getAdminPauseStats() {
  return request({
    url: '/admin/subscription/pause/stats',
    method: 'get'
  })
}

/**
 * Trigger auto-resume (admin only)
 * @returns {Promise} Auto-resume result response
 */
export function triggerAutoResume() {
  return request({
    url: '/admin/subscription/pause/auto-resume',
    method: 'post'
  })
}

export default {
  getPauseStatus,
  pauseSubscription,
  resumeSubscription,
  getPauseHistory,
  getAdminPauseStats,
  triggerAutoResume
}
