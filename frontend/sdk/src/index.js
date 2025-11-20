/**
 * Vea SDK - 统一入口文件
 */

// 导入核心模块
import { VeaClient, VeaError, createAPI, createNodeManager } from './client.js'

// 导入工具函数
import {
  formatTime,
  formatBytes,
  formatInterval,
  formatLatency,
  formatSpeed,
  escapeHtml,
  sleep,
  parseList,
  parseNumber,
  debounce,
  throttle,
  createPoller,
  retry
} from './utils.js'

// 导入状态管理
import {
  createNodeStateManager,
  resolvePreferredNode,
  createNodeIdStorage,
  createThemeManager,
  extractNodeTags,
  filterNodesByTag
} from './state.js'

// 统一导出
const utils = {
  formatTime,
  formatBytes,
  formatInterval,
  formatLatency,
  formatSpeed,
  escapeHtml,
  sleep,
  parseList,
  parseNumber,
  debounce,
  throttle,
  createPoller,
  retry
}

const state = {
  createNodeStateManager,
  resolvePreferredNode,
  createNodeIdStorage,
  createThemeManager,
  extractNodeTags,
  filterNodesByTag
}

// ES Module 导出
export {
  VeaClient,
  VeaError,
  createAPI,
  createNodeManager,
  utils,
  state,
  // 工具函数直接导出
  formatTime,
  formatBytes,
  formatInterval,
  formatLatency,
  formatSpeed,
  escapeHtml,
  sleep,
  parseList,
  parseNumber,
  debounce,
  throttle,
  createPoller,
  retry,
  // 状态管理直接导出
  createNodeStateManager,
  resolvePreferredNode,
  createNodeIdStorage,
  createThemeManager,
  extractNodeTags,
  filterNodesByTag
}

export default VeaClient
