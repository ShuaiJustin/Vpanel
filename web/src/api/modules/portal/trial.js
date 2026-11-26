/**
 * 试用 API 模块
 * 处理用户试用相关的 API 请求
 */
import request from '@/api/base'

/**
 * 获取试用状态
 * @returns {Promise} 试用状态信息
 */
export function getTrialStatus() {
  return request({
    url: '/trial',
    method: 'get'
  })
}

/**
 * 激活试用
 * @returns {Promise} 激活结果
 */
export function activateTrial() {
  return request({
    url: '/trial/activate',
    method: 'post'
  })
}

export default {
  getTrialStatus,
  activateTrial
}
