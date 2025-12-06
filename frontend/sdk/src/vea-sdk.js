/**
 * Vea SDK v1.0.0
 *
 * Xray 代理管理 JavaScript SDK
 *
 * @license MIT
 * @author Vea Team
 */

// ============================================================================
// HTTP Client & API
// ============================================================================

class VeaError extends Error {
  constructor(message, statusCode, response) {
    super(message)
    this.name = 'VeaError'
    this.statusCode = statusCode
    this.response = response
  }
}

class VeaClient {
  constructor(options = {}) {
    this.baseURL = (options.baseURL || 'http://localhost:18080').replace(/\/$/, '')
    this.timeout = options.timeout || 300000 // 5分钟
    this.headers = options.headers || {}

    // 初始化资源API
    this.nodes = new NodesAPI(this)
    this.configs = new ConfigsAPI(this)
    this.geo = new GeoAPI(this)
    this.components = new ComponentsAPI(this)
    this.xray = new XrayAPI(this)
    this.traffic = new TrafficAPI(this)
    this.settings = new SettingsAPI(this)
    this.proxy = new ProxyAPI(this)
    this.proxyProfiles = new ProxyProfilesAPI(this)
  }

  async request(options) {
    const {
      method = 'GET',
      path,
      body,
      headers = {},
      timeout = this.timeout
    } = options

    const url = `${this.baseURL}${path}`
    const requestHeaders = { ...this.headers, ...headers }

    if (body && !(body instanceof FormData)) {
      requestHeaders['Content-Type'] = 'application/json'
    }

    const requestOptions = {
      method,
      headers: requestHeaders
    }

    if (body) {
      requestOptions.body = body instanceof FormData ? body : JSON.stringify(body)
    }

    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), timeout)
    requestOptions.signal = controller.signal

    try {
      const response = await fetch(url, requestOptions)
      clearTimeout(timeoutId)
      return await this._handleResponse(response)
    } catch (error) {
      clearTimeout(timeoutId)
      if (error.name === 'AbortError') {
        throw new VeaError(`Request timeout after ${timeout}ms`, 0, null)
      }
      if (error instanceof VeaError) {
        throw error
      }
      throw new VeaError(`Network error: ${error.message}`, 0, null)
    }
  }

  async _handleResponse(response) {
    if (response.status === 204) {
      return null
    }

    const contentType = response.headers.get('content-type') || ''
    let data

    if (contentType.includes('application/json')) {
      data = await response.json()
    } else {
      data = await response.text()
    }

    if (!response.ok) {
      let message = response.statusText
      if (typeof data === 'object' && data.error) {
        message = data.error
      } else if (typeof data === 'string' && data) {
        message = data
      }
      throw new VeaError(message, response.status, data)
    }

    return data
  }

  async get(path, options = {}) {
    return this.request({ method: 'GET', path, ...options })
  }

  async post(path, body, options = {}) {
    return this.request({ method: 'POST', path, body, ...options })
  }

  async put(path, body, options = {}) {
    return this.request({ method: 'PUT', path, body, ...options })
  }

  async delete(path, options = {}) {
    return this.request({ method: 'DELETE', path, ...options })
  }

  async health() {
    return this.get('/health')
  }

  async snapshot() {
    return this.get('/snapshot')
  }
}

class NodesAPI {
  constructor(client) {
    this.client = client
  }

  async list() {
    return this.client.get('/nodes')
  }

  async create(data) {
    return this.client.post('/nodes', data)
  }

  async update(id, data) {
    return this.client.put(`/nodes/${id}`, data)
  }

  async delete(id) {
    return this.client.delete(`/nodes/${id}`)
  }

  async resetTraffic(id) {
    return this.client.post(`/nodes/${id}/reset-traffic`)
  }

  async incrementTraffic(id, traffic) {
    return this.client.post(`/nodes/${id}/traffic`, traffic)
  }

  async ping(id) {
    return this.client.post(`/nodes/${id}/ping`)
  }

  async speedtest(id) {
    return this.client.post(`/nodes/${id}/speedtest`)
  }

  async select(id) {
    return this.client.post(`/nodes/${id}/select`)
  }

  async bulkPing(ids = []) {
    return this.client.post('/nodes/bulk/ping', { ids })
  }

  async resetSpeed(ids = []) {
    return this.client.post('/nodes/reset-speed', { ids })
  }
}

class ConfigsAPI {
  constructor(client) {
    this.client = client
  }

  async list() {
    return this.client.get('/configs')
  }

  async import(data) {
    return this.client.post('/configs/import', data)
  }

  async update(id, data) {
    return this.client.put(`/configs/${id}`, data)
  }

  async delete(id) {
    return this.client.delete(`/configs/${id}`)
  }

  async refresh(id) {
    return this.client.post(`/configs/${id}/refresh`)
  }

  async pullNodes(id) {
    return this.client.post(`/configs/${id}/pull-nodes`)
  }

  async incrementTraffic(id, traffic) {
    return this.client.post(`/configs/${id}/traffic`, traffic)
  }
}

class GeoAPI {
  constructor(client) {
    this.client = client
  }

  async list() {
    return this.client.get('/geo')
  }

  async create(data) {
    return this.client.post('/geo', data)
  }

  async update(id, data) {
    return this.client.put(`/geo/${id}`, data)
  }

  async delete(id) {
    return this.client.delete(`/geo/${id}`)
  }

  async refresh(id) {
    return this.client.post(`/geo/${id}/refresh`)
  }
}

class ComponentsAPI {
  constructor(client) {
    this.client = client
  }

  async list() {
    return this.client.get('/components')
  }

  async create(data) {
    return this.client.post('/components', data)
  }

  async update(id, data) {
    return this.client.put(`/components/${id}`, data)
  }

  async delete(id) {
    return this.client.delete(`/components/${id}`)
  }

  async install(id) {
    return this.client.post(`/components/${id}/install`)
  }
}

class XrayAPI {
  constructor(client) {
    this.client = client
  }

  async status() {
    return this.client.get('/xray/status')
  }

  async start(options = {}) {
    return this.client.post('/xray/start', options)
  }

  async stop() {
    return this.client.post('/xray/stop')
  }
}

class ProxyAPI {
  constructor(client) {
    this.client = client
  }

  async status() {
    return this.client.get('/proxy/status')
  }

  async stop() {
    return this.client.post('/proxy/stop')
  }
}

class ProxyProfilesAPI {
  constructor(client) {
    this.client = client
  }

  async list() {
    return this.client.get('/proxy-profiles')
  }

  async create(data) {
    return this.client.post('/proxy-profiles', data)
  }

  async get(id) {
    return this.client.get(`/proxy-profiles/${id}`)
  }

  async update(id, data) {
    return this.client.put(`/proxy-profiles/${id}`, data)
  }

  async delete(id) {
    return this.client.delete(`/proxy-profiles/${id}`)
  }

  async start(id) {
    return this.client.post(`/proxy-profiles/${id}/start`)
  }
}

class TrafficAPI {
  constructor(client) {
    this.client = client
    this.rules = new TrafficRulesAPI(client)
  }

  async getProfile() {
    return this.client.get('/traffic/profile')
  }

  async updateProfile(data) {
    return this.client.put('/traffic/profile', data)
  }
}

class TrafficRulesAPI {
  constructor(client) {
    this.client = client
  }

  async list() {
    return this.client.get('/traffic/rules')
  }

  async create(data) {
    return this.client.post('/traffic/rules', data)
  }

  async update(id, data) {
    return this.client.put(`/traffic/rules/${id}`, data)
  }

  async delete(id) {
    return this.client.delete(`/traffic/rules/${id}`)
  }
}

class SettingsAPI {
  constructor(client) {
    this.client = client
  }

  async getSystemProxy() {
    return this.client.get('/settings/system-proxy')
  }

  async updateSystemProxy(data) {
    return this.client.put('/settings/system-proxy', data)
  }
}

function createAPI(baseURL = '') {
  const client = new VeaClient({ baseURL })

  return {
    async request(path, options = {}) {
      return client.request({ path, ...options })
    },

    async get(path, options = {}) {
      return client.get(path, options)
    },

    async post(path, body, options = {}) {
      return client.post(path, body, options)
    },

    async put(path, body, options = {}) {
      return client.put(path, body, options)
    },

    async delete(path, options = {}) {
      return client.delete(path, options)
    },

    // Expose all client APIs
    nodes: client.nodes,
    configs: client.configs,
    geo: client.geo,
    components: client.components,
    xray: client.xray,
    traffic: client.traffic,
    settings: client.settings,
    proxy: client.proxy,
    proxyProfiles: client.proxyProfiles,

    client
  }
}

function createNodeManager(client, interval = 1000) {
  let nodesCache = []
  let activeNodeId = ''
  let lastSelectedNodeId = ''
  let pollingTimer = null
  let listeners = []

  async function fetchNodes() {
    try {
      const result = await client.nodes.list()
      nodesCache = result.nodes || []
      activeNodeId = result.activeNodeId || ''
      lastSelectedNodeId = result.lastSelectedNodeId || ''

      listeners.forEach(fn => {
        try {
          fn({ nodes: nodesCache, activeNodeId, lastSelectedNodeId })
        } catch (err) {
          console.error('Node listener error:', err)
        }
      })

      return result
    } catch (error) {
      console.error('Failed to fetch nodes:', error)
      throw error
    }
  }

  return {
    getNodes() {
      return nodesCache
    },

    getActiveNodeId() {
      return activeNodeId
    },

    getLastSelectedNodeId() {
      return lastSelectedNodeId
    },

    async refresh() {
      return fetchNodes()
    },

    startPolling() {
      if (pollingTimer) return
      fetchNodes()
      pollingTimer = setInterval(fetchNodes, interval)
    },

    stopPolling() {
      if (pollingTimer) {
        clearInterval(pollingTimer)
        pollingTimer = null
      }
    },

    onUpdate(callback) {
      listeners.push(callback)
      return () => {
        listeners = listeners.filter(fn => fn !== callback)
      }
    }
  }
}

// ============================================================================
// Utility Functions
// ============================================================================

function formatTime(value) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return String(value)
  return date.toLocaleString()
}

function formatBytes(bytes) {
  const size = Number(bytes)
  if (!size || size <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const index = Math.min(Math.floor(Math.log(size) / Math.log(1024)), units.length - 1)
  const value = size / Math.pow(1024, index)
  return `${value.toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}

function formatInterval(duration) {
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

function escapeHtml(value) {
  return String(value ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

function formatLatency(ms) {
  if (!ms || ms <= 0) return '-'
  if (ms < 1000) return `${Math.round(ms)} ms`
  return `${(ms / 1000).toFixed(2)} s`
}

function formatSpeed(mbps) {
  if (!mbps || mbps <= 0) return '-'
  if (mbps >= 10) return `${mbps.toFixed(1)} MB/s`
  return `${mbps.toFixed(2)} MB/s`
}

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms))
}

function parseList(input) {
  if (!input) return []
  return input
    .split(/[\s,;\n]+/)
    .map(item => item.trim())
    .filter(Boolean)
}

function parseNumber(value) {
  const num = Number(value)
  return Number.isFinite(num) ? num : 0
}

function debounce(fn, delay) {
  let timer = null
  return function (...args) {
    if (timer) clearTimeout(timer)
    timer = setTimeout(() => fn.apply(this, args), delay)
  }
}

function throttle(fn, delay) {
  let last = 0
  return function (...args) {
    const now = Date.now()
    if (now - last >= delay) {
      last = now
      fn.apply(this, args)
    }
  }
}

function createPoller(fn, interval) {
  let timer = null
  let running = false

  return {
    start() {
      if (running) return
      running = true
      Promise.resolve(fn()).catch(console.error)
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

async function retry(fn, options = {}) {
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
        await sleep(delay * Math.pow(2, i))
      } else {
        break
      }
    }
  }

  throw lastError
}

// ============================================================================
// State Management
// ============================================================================

function createNodeStateManager(options = {}) {
  const {
    pingCooldown = 60000,
    speedtestCooldown = 60000
  } = options

  const pingState = {
    running: false,
    lastNodeId: '',
    lastTriggeredAt: 0
  }

  const speedtestState = {
    running: false,
    lastNodeId: '',
    lastTriggeredAt: 0
  }

  return {
    canPing(nodeId, node = null, force = false) {
      if (force) return true

      const now = Date.now()

      if (pingState.running && now - pingState.lastTriggeredAt < pingCooldown) {
        return false
      }

      if (pingState.lastNodeId === nodeId && now - pingState.lastTriggeredAt < pingCooldown) {
        return false
      }

      if (node) {
        const lastLatencyAt = node.lastLatencyAt ? Date.parse(node.lastLatencyAt) : NaN
        if (!Number.isNaN(lastLatencyAt) && now - lastLatencyAt < pingCooldown) {
          return false
        }
      }

      return true
    },

    startPing(nodeId) {
      pingState.running = true
      pingState.lastNodeId = nodeId
      pingState.lastTriggeredAt = Date.now()
    },

    endPing() {
      pingState.running = false
    },

    canSpeedtest(nodeId, node = null, force = false) {
      if (force) return true

      const now = Date.now()

      if (speedtestState.running && now - speedtestState.lastTriggeredAt < speedtestCooldown) {
        return false
      }

      if (speedtestState.lastNodeId === nodeId && now - speedtestState.lastTriggeredAt < speedtestCooldown) {
        return false
      }

      if (node && (!node.lastSpeedError || node.lastSpeedError.length === 0)) {
        const lastSpeedAt = node.lastSpeedAt ? Date.parse(node.lastSpeedAt) : NaN
        if (!Number.isNaN(lastSpeedAt) && now - lastSpeedAt < speedtestCooldown) {
          return false
        }
      }

      return true
    },

    startSpeedtest(nodeId) {
      speedtestState.running = true
      speedtestState.lastNodeId = nodeId
      speedtestState.lastTriggeredAt = Date.now()
    },

    endSpeedtest() {
      speedtestState.running = false
    }
  }
}

function resolvePreferredNode(options) {
  const { nodes, activeNodeId, lastSelectedNodeId, savedNodeId } = options

  if (!Array.isArray(nodes) || nodes.length === 0) {
    return ''
  }

  const hasNode = (id) => !!id && nodes.some((node) => node && node.id === id)

  if (savedNodeId && hasNode(savedNodeId)) {
    return savedNodeId
  }
  if (hasNode(lastSelectedNodeId)) {
    return lastSelectedNodeId
  }
  if (hasNode(activeNodeId)) {
    return activeNodeId
  }
  if (nodes.length > 0 && nodes[0] && nodes[0].id) {
    return nodes[0].id
  }

  return ''
}

function createNodeIdStorage(key = 'vea_selected_node_id') {
  return {
    get() {
      try {
        return localStorage.getItem(key) || ''
      } catch {
        return ''
      }
    },

    set(nodeId) {
      try {
        if (nodeId) {
          localStorage.setItem(key, nodeId)
        } else {
          localStorage.removeItem(key)
        }
      } catch (err) {
        console.error('Failed to save node ID:', err)
      }
    },

    remove() {
      try {
        localStorage.removeItem(key)
      } catch (err) {
        console.error('Failed to remove node ID:', err)
      }
    }
  }
}

function createThemeManager() {
  const STORAGE_KEY = 'theme'
  const THEMES = {
    DARK: 'dark',
    LIGHT: 'light'
  }

  return {
    getCurrent() {
      try {
        return localStorage.getItem(STORAGE_KEY) || THEMES.DARK
      } catch {
        return THEMES.DARK
      }
    },

    switch(theme) {
      try {
        localStorage.setItem(STORAGE_KEY, theme)
        const file = theme === THEMES.DARK ? 'dark.html' : 'light.html'
        window.location.href = file
      } catch (err) {
        console.error('Failed to switch theme:', err)
      }
    },

    detectFromFilename() {
      const currentFile = window.location.pathname.split('/').pop()
      return currentFile.includes('light') ? THEMES.LIGHT : THEMES.DARK
    },

    autoRedirect() {
      const savedTheme = this.getCurrent()
      const currentTheme = this.detectFromFilename()

      if (savedTheme !== currentTheme) {
        this.switch(savedTheme)
      }
    },

    THEMES
  }
}

function extractNodeTags(nodes) {
  const tags = new Set()

  if (Array.isArray(nodes)) {
    nodes.forEach((node) => {
      if (Array.isArray(node.tags)) {
        node.tags.forEach((tag) => {
          const trimmed = String(tag || '').trim()
          if (trimmed) {
            tags.add(trimmed)
          }
        })
      }
    })
  }

  return ['全部', ...Array.from(tags).sort((a, b) => a.localeCompare(b, 'zh-Hans-CN'))]
}

function filterNodesByTag(nodes, tag) {
  if (!tag || tag === '全部') {
    return nodes
  }

  return nodes.filter((node) =>
    Array.isArray(node.tags) && node.tags.includes(tag)
  )
}

// ============================================================================
// Exports
// ============================================================================

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

export {
  VeaClient,
  VeaError,
  createAPI,
  createNodeManager,
  utils,
  state,
  // Utils
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
  // State
  createNodeStateManager,
  resolvePreferredNode,
  createNodeIdStorage,
  createThemeManager,
  extractNodeTags,
  filterNodesByTag
}

export default VeaClient
