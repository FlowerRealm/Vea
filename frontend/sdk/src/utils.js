/**
 * Vea SDK - 工具函数
 *
 * 从现有前端提取的实用工具函数
 */

/**
 * 格式化时间
 * @param {string|Date} value - ISO 8601 时间字符串或 Date 对象
 * @returns {string} 本地化时间字符串
 */
export function formatTime(value) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return String(value)
  return date.toLocaleString()
}

/**
 * 格式化字节数为可读格式
 * @param {number} bytes - 字节数
 * @returns {string} 格式化后的字符串（如 "1.5 MB"）
 */
export function formatBytes(bytes) {
  const size = Number(bytes)
  if (!size || size <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const index = Math.min(Math.floor(Math.log(size) / Math.log(1024)), units.length - 1)
  const value = size / Math.pow(1024, index)
  return `${value.toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}

/**
 * 格式化时间间隔（Go duration）
 * @param {number|string} duration - 纳秒（数字）或 Go duration 字符串
 * @returns {string} 格式化后的字符串
 */
export function formatInterval(duration) {
  if (!duration) return '未设置'

  if (typeof duration === 'number') {
    if (duration === 0) return '未设置'
    const minutes = Math.round(duration / 60000000000)
    return `${minutes} 分钟`
  }

  if (typeof duration === 'string') {
    if (duration === '0s' || duration === '0') return '未设置'
    return duration.replace('m0s', 'm').replace('h0m0s', 'h')
  }

  return String(duration)
}

/**
 * HTML 转义
 * @param {any} value - 需要转义的值
 * @returns {string} 转义后的字符串
 */
export function escapeHtml(value) {
  return String(value ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

/**
 * 格式化延迟（毫秒）
 * @param {number} ms - 毫秒数
 * @returns {string} 格式化后的字符串
 */
export function formatLatency(ms) {
  if (!ms || ms <= 0) return '-'
  if (ms < 1000) return `${Math.round(ms)} ms`
  return `${(ms / 1000).toFixed(2)} s`
}

/**
 * 格式化速度（MB/s）
 * @param {number} mbps - MB/s
 * @returns {string} 格式化后的字符串
 */
export function formatSpeed(mbps) {
  if (!mbps || mbps <= 0) return '-'
  if (mbps >= 10) return `${mbps.toFixed(1)} MB/s`
  return `${mbps.toFixed(2)} MB/s`
}

/**
 * 休眠指定时间
 * @param {number} ms - 毫秒数
 * @returns {Promise<void>}
 */
export function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms))
}

/**
 * 解析列表字符串（逗号、分号、换行分隔）
 * @param {string} input - 输入字符串
 * @returns {string[]} 解析后的数组
 */
export function parseList(input) {
  if (!input) return []
  return input
    .split(/[\s,;\n]+/)
    .map(item => item.trim())
    .filter(Boolean)
}

/**
 * 解析数字
 * @param {any} value - 需要解析的值
 * @returns {number} 数字（失败返回 0）
 */
export function parseNumber(value) {
  const num = Number(value)
  return Number.isFinite(num) ? num : 0
}

/**
 * 防抖函数
 * @param {Function} fn - 需要防抖的函数
 * @param {number} delay - 延迟时间（毫秒）
 * @returns {Function} 防抖后的函数
 */
export function debounce(fn, delay) {
  let timer = null
  return function(...args) {
    if (timer) clearTimeout(timer)
    timer = setTimeout(() => fn.apply(this, args), delay)
  }
}

/**
 * 节流函数
 * @param {Function} fn - 需要节流的函数
 * @param {number} delay - 延迟时间（毫秒）
 * @returns {Function} 节流后的函数
 */
export function throttle(fn, delay) {
  let last = 0
  return function(...args) {
    const now = Date.now()
    if (now - last >= delay) {
      last = now
      fn.apply(this, args)
    }
  }
}

/**
 * 创建轮询器
 * @param {Function} fn - 轮询执行的函数
 * @param {number} interval - 轮询间隔（毫秒）
 * @returns {Object} 包含 start/stop 方法的控制器
 */
export function createPoller(fn, interval) {
  let timer = null
  let running = false

  return {
    start() {
      if (running) return
      running = true

      // 立即执行一次
      Promise.resolve(fn()).catch(console.error)

      // 启动定时器
      timer = setInterval(() => {
        Promise.resolve(fn()).catch(console.error)
      }, interval)
    },

    stop() {
      if (!running) return
      running = false
      if (timer) {
        clearInterval(timer)
        timer = null
      }
    },

    isRunning() {
      return running
    }
  }
}

/**
 * 重试函数
 * @param {Function} fn - 需要重试的函数
 * @param {Object} options - 重试选项
 * @param {number} options.maxRetries - 最大重试次数
 * @param {number} options.delay - 重试延迟（毫秒）
 * @param {Function} options.shouldRetry - 判断是否应该重试的函数
 * @returns {Promise<any>}
 */
export async function retry(fn, options = {}) {
  const {
    maxRetries = 3,
    delay = 1000,
    shouldRetry = () => true
  } = options

  let lastError

  for (let i = 0; i <= maxRetries; i++) {
    try {
      return await fn()
    } catch (error) {
      lastError = error

      if (i < maxRetries && shouldRetry(error)) {
        await sleep(delay * Math.pow(2, i))  // 指数退避
      } else {
        break
      }
    }
  }

  throw lastError
}

// 导出所有工具函数
export default {
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
