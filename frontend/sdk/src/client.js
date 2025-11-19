/**
 * Vea SDK - HTTP Client
 *
 * 核心 HTTP 客户端，处理所有 API 请求和响应
 * 支持浏览器和 Node.js 环境
 */

class VeaError extends Error {
  constructor(message, statusCode, response) {
    super(message)
    this.name = 'VeaError'
    this.statusCode = statusCode
    this.response = response
  }
}

class VeaClient {
  /**
   * 创建 Vea 客户端实例
   * @param {Object} options - 配置选项
   * @param {string} options.baseURL - API 基础 URL（默认：http://localhost:8080）
   * @param {number} options.timeout - 请求超时时间（毫秒，默认：300000 = 5分钟）
   * @param {Object} options.headers - 自定义 HTTP 头
   */
  constructor(options = {}) {
    this.baseURL = (options.baseURL || 'http://localhost:8080').replace(/\/$/, '')
    this.timeout = options.timeout || 300000 // 5 分钟，适合大文件下载（如 Xray）
    this.headers = options.headers || {}

    // 检测环境
    this.isBrowser = typeof window !== 'undefined' && typeof window.document !== 'undefined'
    this.isNode = typeof process !== 'undefined' && process.versions && process.versions.node

    // 初始化资源 API
    this.nodes = new NodesAPI(this)
    this.configs = new ConfigsAPI(this)
    this.geo = new GeoAPI(this)
    this.components = new ComponentsAPI(this)
    this.xray = new XrayAPI(this)
    this.traffic = new TrafficAPI(this)
    this.settings = new SettingsAPI(this)
  }

  /**
   * 执行 HTTP 请求
   * @param {Object} options - 请求选项
   * @param {string} options.method - HTTP 方法
   * @param {string} options.path - 请求路径
   * @param {Object} options.body - 请求体
   * @param {Object} options.headers - 自定义请求头
   * @param {number} options.timeout - 超时时间（覆盖默认值）
   * @returns {Promise<any>} 响应数据
   */
  async request(options) {
    const {
      method = 'GET',
      path,
      body,
      headers = {},
      timeout = this.timeout
    } = options

    const url = `${this.baseURL}${path}`
    const requestHeaders = {
      ...this.headers,
      ...headers
    }

    // 如果有 body，自动设置 Content-Type
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

    // 创建 AbortController 实现超时
    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), timeout)
    requestOptions.signal = controller.signal

    try {
      const response = await fetch(url, requestOptions)
      clearTimeout(timeoutId)

      // 处理响应
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

  /**
   * 处理 HTTP 响应
   * @private
   * @param {Response} response - Fetch API Response 对象
   * @returns {Promise<any>} 解析后的响应数据
   */
  async _handleResponse(response) {
    // 204 No Content
    if (response.status === 204) {
      return null
    }

    const contentType = response.headers.get('content-type') || ''
    let data

    // 解析响应体
    if (contentType.includes('application/json')) {
      data = await response.json()
    } else {
      data = await response.text()
    }

    // 检查错误
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

  /**
   * GET 请求快捷方法
   * @param {string} path - 请求路径
   * @param {Object} options - 额外选项
   * @returns {Promise<any>}
   */
  async get(path, options = {}) {
    return this.request({ method: 'GET', path, ...options })
  }

  /**
   * POST 请求快捷方法
   * @param {string} path - 请求路径
   * @param {Object} body - 请求体
   * @param {Object} options - 额外选项
   * @returns {Promise<any>}
   */
  async post(path, body, options = {}) {
    return this.request({ method: 'POST', path, body, ...options })
  }

  /**
   * PUT 请求快捷方法
   * @param {string} path - 请求路径
   * @param {Object} body - 请求体
   * @param {Object} options - 额外选项
   * @returns {Promise<any>}
   */
  async put(path, body, options = {}) {
    return this.request({ method: 'PUT', path, body, ...options })
  }

  /**
   * DELETE 请求快捷方法
   * @param {string} path - 请求路径
   * @param {Object} options - 额外选项
   * @returns {Promise<any>}
   */
  async delete(path, options = {}) {
    return this.request({ method: 'DELETE', path, ...options })
  }

  /**
   * 健康检查
   * @returns {Promise<{status: string, timestamp: string}>}
   */
  async health() {
    return this.get('/health')
  }

  /**
   * 获取完整状态快照
   * @returns {Promise<Object>} 包含所有节点、配置、组件等的快照
   */
  async snapshot() {
    return this.get('/snapshot')
  }
}

/**
 * 节点管理 API
 */
class NodesAPI {
  constructor(client) {
    this.client = client
  }

  /**
   * 列出所有节点
   * @returns {Promise<{nodes: Array, activeNodeId: string, lastSelectedNodeId: string}>}
   */
  async list() {
    return this.client.get('/nodes')
  }

  /**
   * 创建节点
   * @param {Object} data - 节点数据
   * @param {string} data.shareLink - 分享链接（vmess/vless/trojan/ss）
   * @param {string} data.name - 节点名称（手动创建时必需）
   * @param {string} data.address - 服务器地址（手动创建时必需）
   * @param {number} data.port - 端口（手动创建时必需）
   * @param {string} data.protocol - 协议类型（手动创建时必需）
   * @param {Array<string>} data.tags - 标签列表
   * @returns {Promise<Object>} 创建的节点
   */
  async create(data) {
    return this.client.post('/nodes', data)
  }

  /**
   * 更新节点
   * @param {string} id - 节点 ID
   * @param {Object} data - 更新数据
   * @returns {Promise<Object>} 更新后的节点
   */
  async update(id, data) {
    return this.client.put(`/nodes/${id}`, data)
  }

  /**
   * 删除节点
   * @param {string} id - 节点 ID
   * @returns {Promise<null>}
   */
  async delete(id) {
    return this.client.delete(`/nodes/${id}`)
  }

  /**
   * 重置节点流量
   * @param {string} id - 节点 ID
   * @returns {Promise<Object>} 更新后的节点
   */
  async resetTraffic(id) {
    return this.client.post(`/nodes/${id}/reset-traffic`)
  }

  /**
   * 增加节点流量
   * @param {string} id - 节点 ID
   * @param {Object} traffic - 流量数据
   * @param {number} traffic.uploadBytes - 上传字节数
   * @param {number} traffic.downloadBytes - 下载字节数
   * @returns {Promise<Object>} 更新后的节点
   */
  async incrementTraffic(id, traffic) {
    return this.client.post(`/nodes/${id}/traffic`, traffic)
  }

  /**
   * 测试节点延迟（异步）
   * @param {string} id - 节点 ID
   * @returns {Promise<null>}
   */
  async ping(id) {
    return this.client.post(`/nodes/${id}/ping`)
  }

  /**
   * 测试节点速度（异步）
   * @param {string} id - 节点 ID
   * @returns {Promise<null>}
   */
  async speedtest(id) {
    return this.client.post(`/nodes/${id}/speedtest`)
  }

  /**
   * 选择节点（切换 Xray 使用的节点）
   * @param {string} id - 节点 ID
   * @returns {Promise<null>}
   */
  async select(id) {
    return this.client.post(`/nodes/${id}/select`)
  }

  /**
   * 批量测试节点延迟
   * @param {Array<string>} ids - 节点 ID 列表（为空则测试所有节点）
   * @returns {Promise<null>}
   */
  async bulkPing(ids = []) {
    return this.client.post('/nodes/bulk/ping', { ids })
  }

  /**
   * 重置节点速度测试结果
   * @param {Array<string>} ids - 节点 ID 列表
   * @returns {Promise<null>}
   */
  async resetSpeed(ids = []) {
    return this.client.post('/nodes/reset-speed', { ids })
  }
}

/**
 * 配置管理 API
 */
class ConfigsAPI {
  constructor(client) {
    this.client = client
  }

  /**
   * 列出所有配置
   * @returns {Promise<Array>}
   */
  async list() {
    return this.client.get('/configs')
  }

  /**
   * 导入配置
   * @param {Object} data - 配置数据
   * @param {string} data.name - 配置名称
   * @param {string} data.format - 配置格式（默认 xray-json）
   * @param {string} data.sourceUrl - 配置源地址或订阅链接
   * @param {string} data.payload - 配置内容（可选）
   * @param {number} data.autoUpdateIntervalMinutes - 自动更新间隔（分钟）
   * @param {string} data.expireAt - 过期时间（ISO 8601 格式）
   * @returns {Promise<Object>} 创建的配置
   */
  async import(data) {
    return this.client.post('/configs/import', data)
  }

  /**
   * 更新配置
   * @param {string} id - 配置 ID
   * @param {Object} data - 更新数据
   * @returns {Promise<Object>} 更新后的配置
   */
  async update(id, data) {
    return this.client.put(`/configs/${id}`, data)
  }

  /**
   * 删除配置
   * @param {string} id - 配置 ID
   * @returns {Promise<null>}
   */
  async delete(id) {
    return this.client.delete(`/configs/${id}`)
  }

  /**
   * 刷新配置（从 sourceUrl 重新拉取）
   * @param {string} id - 配置 ID
   * @returns {Promise<Object>} 刷新后的配置
   */
  async refresh(id) {
    return this.client.post(`/configs/${id}/refresh`)
  }

  /**
   * 同步配置节点（从配置中提取节点）
   * @param {string} id - 配置 ID
   * @returns {Promise<Array>} 提取的节点列表
   */
  async pullNodes(id) {
    return this.client.post(`/configs/${id}/pull-nodes`)
  }

  /**
   * 增加配置流量
   * @param {string} id - 配置 ID
   * @param {Object} traffic - 流量数据
   * @returns {Promise<Object>} 更新后的配置
   */
  async incrementTraffic(id, traffic) {
    return this.client.post(`/configs/${id}/traffic`, traffic)
  }
}

/**
 * Geo 资源管理 API
 */
class GeoAPI {
  constructor(client) {
    this.client = client
  }

  /**
   * 列出所有 Geo 资源
   * @returns {Promise<Array>}
   */
  async list() {
    return this.client.get('/geo')
  }

  /**
   * 创建 Geo 资源
   * @param {Object} data - Geo 资源数据
   * @returns {Promise<Object>}
   */
  async create(data) {
    return this.client.post('/geo', data)
  }

  /**
   * 更新 Geo 资源（upsert）
   * @param {string} id - Geo 资源 ID
   * @param {Object} data - 更新数据
   * @returns {Promise<Object>}
   */
  async update(id, data) {
    return this.client.put(`/geo/${id}`, data)
  }

  /**
   * 删除 Geo 资源
   * @param {string} id - Geo 资源 ID
   * @returns {Promise<null>}
   */
  async delete(id) {
    return this.client.delete(`/geo/${id}`)
  }

  /**
   * 刷新 Geo 资源（从 sourceUrl 重新下载）
   * @param {string} id - Geo 资源 ID
   * @returns {Promise<Object>}
   */
  async refresh(id) {
    return this.client.post(`/geo/${id}/refresh`)
  }
}

/**
 * 核心组件管理 API
 */
class ComponentsAPI {
  constructor(client) {
    this.client = client
  }

  /**
   * 列出所有核心组件
   * @returns {Promise<Array>}
   */
  async list() {
    return this.client.get('/components')
  }

  /**
   * 创建核心组件记录
   * @param {Object} data - 组件数据
   * @param {string} data.kind - 组件类型（xray/geo/generic）
   * @returns {Promise<Object>}
   */
  async create(data) {
    return this.client.post('/components', data)
  }

  /**
   * 更新核心组件
   * @param {string} id - 组件 ID
   * @param {Object} data - 更新数据
   * @returns {Promise<Object>}
   */
  async update(id, data) {
    return this.client.put(`/components/${id}`, data)
  }

  /**
   * 删除核心组件
   * @param {string} id - 组件 ID
   * @returns {Promise<null>}
   */
  async delete(id) {
    return this.client.delete(`/components/${id}`)
  }

  /**
   * 安装核心组件
   * @param {string} id - 组件 ID
   * @returns {Promise<Object>}
   */
  async install(id) {
    return this.client.post(`/components/${id}/install`)
  }
}

/**
 * Xray 控制 API
 */
class XrayAPI {
  constructor(client) {
    this.client = client
  }

  /**
   * 获取 Xray 状态
   * @returns {Promise<Object>}
   */
  async status() {
    return this.client.get('/xray/status')
  }

  /**
   * 启动 Xray
   * @param {Object} options - 启动选项
   * @param {string} options.activeNodeId - 使用的节点 ID
   * @returns {Promise<null>}
   */
  async start(options = {}) {
    return this.client.post('/xray/start', options)
  }

  /**
   * 停止 Xray
   * @returns {Promise<null>}
   */
  async stop() {
    return this.client.post('/xray/stop')
  }
}

/**
 * 流量策略 API
 */
class TrafficAPI {
  constructor(client) {
    this.client = client
    this.rules = new TrafficRulesAPI(client)
  }

  /**
   * 获取流量策略
   * @returns {Promise<Object>}
   */
  async getProfile() {
    return this.client.get('/traffic/profile')
  }

  /**
   * 更新流量策略
   * @param {Object} data - 策略数据
   * @param {string} data.defaultNodeId - 默认出口节点 ID
   * @param {Object} data.dns - DNS 设置
   * @returns {Promise<Object>}
   */
  async updateProfile(data) {
    return this.client.put('/traffic/profile', data)
  }
}

/**
 * 分流规则 API
 */
class TrafficRulesAPI {
  constructor(client) {
    this.client = client
  }

  /**
   * 列出所有分流规则
   * @returns {Promise<Array>}
   */
  async list() {
    return this.client.get('/traffic/rules')
  }

  /**
   * 创建分流规则
   * @param {Object} data - 规则数据
   * @returns {Promise<Object>}
   */
  async create(data) {
    return this.client.post('/traffic/rules', data)
  }

  /**
   * 更新分流规则
   * @param {string} id - 规则 ID
   * @param {Object} data - 更新数据
   * @returns {Promise<Object>}
   */
  async update(id, data) {
    return this.client.put(`/traffic/rules/${id}`, data)
  }

  /**
   * 删除分流规则
   * @param {string} id - 规则 ID
   * @returns {Promise<null>}
   */
  async delete(id) {
    return this.client.delete(`/traffic/rules/${id}`)
  }
}

/**
 * 系统设置 API
 */
class SettingsAPI {
  constructor(client) {
    this.client = client
  }

  /**
   * 获取系统代理设置
   * @returns {Promise<{settings: Object, message: string}>}
   */
  async getSystemProxy() {
    return this.client.get('/settings/system-proxy')
  }

  /**
   * 更新系统代理设置
   * @param {Object} data - 代理设置
   * @param {boolean} data.enabled - 是否启用系统代理
   * @param {Array<string>} data.ignoreHosts - 忽略代理的主机列表
   * @returns {Promise<{settings: Object, message: string}>}
   */
  async updateSystemProxy(data) {
    return this.client.put('/settings/system-proxy', data)
  }
}

/**
 * 简化版 API 对象（兼容现有前端代码）
 *
 * 使用方式:
 * const api = createAPI('http://localhost:8080')
 * await api.get('/nodes')
 * await api.post('/nodes', { name: 'test' })
 */
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

    // 暴露底层客户端
    client
  }
}

/**
 * 创建带轮询的节点管理器
 * @param {VeaClient} client - Vea 客户端实例
 * @param {number} interval - 轮询间隔（毫秒，默认 1000）
 * @returns {Object} 节点管理器
 */
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

      // 触发监听器
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
    // 获取当前缓存的节点
    getNodes() {
      return nodesCache
    },

    getActiveNodeId() {
      return activeNodeId
    },

    getLastSelectedNodeId() {
      return lastSelectedNodeId
    },

    // 刷新节点（手动）
    async refresh() {
      return fetchNodes()
    },

    // 启动轮询
    startPolling() {
      if (pollingTimer) return
      fetchNodes()  // 立即执行一次
      pollingTimer = setInterval(fetchNodes, interval)
    },

    // 停止轮询
    stopPolling() {
      if (pollingTimer) {
        clearInterval(pollingTimer)
        pollingTimer = null
      }
    },

    // 监听节点变化
    onUpdate(callback) {
      listeners.push(callback)
      return () => {
        listeners = listeners.filter(fn => fn !== callback)
      }
    }
  }
}

// ES Module 导出（rollup 打包时使用）
export {
  VeaClient,
  VeaError,
  createAPI,
  createNodeManager
}

// 默认导出
export default VeaClient
