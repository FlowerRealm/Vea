/**
 * @vea/sdk v1.0.0
 * (c) 2025 Vea Project
 * @license MIT
 */
(function (global, factory) {
  typeof exports === 'object' && typeof module !== 'undefined' ? factory(exports) :
  typeof define === 'function' && define.amd ? define(['exports'], factory) :
  (global = typeof globalThis !== 'undefined' ? globalThis : global || self, factory(global.VeaSDK = {}));
})(this, (function (exports) { 'use strict';

  /**
   * Vea SDK - HTTP Client
   *
   * 核心 HTTP 客户端，处理所有 API 请求和响应
   * 支持浏览器和 Node.js 环境
   */

  class VeaError extends Error {
    constructor(message, statusCode, response) {
      super(message);
      this.name = 'VeaError';
      this.statusCode = statusCode;
      this.response = response;
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
      this.baseURL = (options.baseURL || 'http://localhost:8080').replace(/\/$/, '');
      this.timeout = options.timeout || 300000; // 5 分钟，适合大文件下载（如 Xray）
      this.headers = options.headers || {};

      // 检测环境
      this.isBrowser = typeof window !== 'undefined' && typeof window.document !== 'undefined';
      this.isNode = typeof process !== 'undefined' && process.versions && process.versions.node;

      // 初始化资源 API
      this.nodes = new NodesAPI(this);
      this.configs = new ConfigsAPI(this);
      this.geo = new GeoAPI(this);
      this.components = new ComponentsAPI(this);
      this.xray = new XrayAPI(this);
      this.traffic = new TrafficAPI(this);
      this.settings = new SettingsAPI(this);
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
      } = options;

      const url = `${this.baseURL}${path}`;
      const requestHeaders = {
        ...this.headers,
        ...headers
      };

      // 如果有 body，自动设置 Content-Type
      if (body && !(body instanceof FormData)) {
        requestHeaders['Content-Type'] = 'application/json';
      }

      const requestOptions = {
        method,
        headers: requestHeaders
      };

      if (body) {
        requestOptions.body = body instanceof FormData ? body : JSON.stringify(body);
      }

      // 创建 AbortController 实现超时
      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), timeout);
      requestOptions.signal = controller.signal;

      try {
        const response = await fetch(url, requestOptions);
        clearTimeout(timeoutId);

        // 处理响应
        return await this._handleResponse(response)
      } catch (error) {
        clearTimeout(timeoutId);

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

      const contentType = response.headers.get('content-type') || '';
      let data;

      // 解析响应体
      if (contentType.includes('application/json')) {
        data = await response.json();
      } else {
        data = await response.text();
      }

      // 检查错误
      if (!response.ok) {
        let message = response.statusText;

        if (typeof data === 'object' && data.error) {
          message = data.error;
        } else if (typeof data === 'string' && data) {
          message = data;
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
      this.client = client;
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
      this.client = client;
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
      this.client = client;
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
      this.client = client;
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
      this.client = client;
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
      this.client = client;
      this.rules = new TrafficRulesAPI(client);
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
      this.client = client;
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
      this.client = client;
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
    const client = new VeaClient({ baseURL });

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
    let nodesCache = [];
    let activeNodeId = '';
    let lastSelectedNodeId = '';
    let pollingTimer = null;
    let listeners = [];

    async function fetchNodes() {
      try {
        const result = await client.nodes.list();
        nodesCache = result.nodes || [];
        activeNodeId = result.activeNodeId || '';
        lastSelectedNodeId = result.lastSelectedNodeId || '';

        // 触发监听器
        listeners.forEach(fn => {
          try {
            fn({ nodes: nodesCache, activeNodeId, lastSelectedNodeId });
          } catch (err) {
            console.error('Node listener error:', err);
          }
        });

        return result
      } catch (error) {
        console.error('Failed to fetch nodes:', error);
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
        fetchNodes();  // 立即执行一次
        pollingTimer = setInterval(fetchNodes, interval);
      },

      // 停止轮询
      stopPolling() {
        if (pollingTimer) {
          clearInterval(pollingTimer);
          pollingTimer = null;
        }
      },

      // 监听节点变化
      onUpdate(callback) {
        listeners.push(callback);
        return () => {
          listeners = listeners.filter(fn => fn !== callback);
        }
      }
    }
  }

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
  function formatTime(value) {
    if (!value) return '-'
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return String(value)
    return date.toLocaleString()
  }

  /**
   * 格式化字节数为可读格式
   * @param {number} bytes - 字节数
   * @returns {string} 格式化后的字符串（如 "1.5 MB"）
   */
  function formatBytes(bytes) {
    const size = Number(bytes);
    if (!size || size <= 0) return '0 B'
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const index = Math.min(Math.floor(Math.log(size) / Math.log(1024)), units.length - 1);
    const value = size / Math.pow(1024, index);
    return `${value.toFixed(index === 0 ? 0 : 1)} ${units[index]}`
  }

  /**
   * 格式化时间间隔（Go duration）
   * @param {number|string} duration - 纳秒（数字）或 Go duration 字符串
   * @returns {string} 格式化后的字符串
   */
  function formatInterval(duration) {
    if (!duration) return '未设置'

    if (typeof duration === 'number') {
      if (duration === 0) return '未设置'
      const minutes = Math.round(duration / 60000000000);
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
  function escapeHtml(value) {
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
  function formatLatency(ms) {
    if (!ms || ms <= 0) return '-'
    if (ms < 1000) return `${Math.round(ms)} ms`
    return `${(ms / 1000).toFixed(2)} s`
  }

  /**
   * 格式化速度（MB/s）
   * @param {number} mbps - MB/s
   * @returns {string} 格式化后的字符串
   */
  function formatSpeed(mbps) {
    if (!mbps || mbps <= 0) return '-'
    if (mbps >= 10) return `${mbps.toFixed(1)} MB/s`
    return `${mbps.toFixed(2)} MB/s`
  }

  /**
   * 休眠指定时间
   * @param {number} ms - 毫秒数
   * @returns {Promise<void>}
   */
  function sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms))
  }

  /**
   * 解析列表字符串（逗号、分号、换行分隔）
   * @param {string} input - 输入字符串
   * @returns {string[]} 解析后的数组
   */
  function parseList(input) {
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
  function parseNumber(value) {
    const num = Number(value);
    return Number.isFinite(num) ? num : 0
  }

  /**
   * 防抖函数
   * @param {Function} fn - 需要防抖的函数
   * @param {number} delay - 延迟时间（毫秒）
   * @returns {Function} 防抖后的函数
   */
  function debounce(fn, delay) {
    let timer = null;
    return function(...args) {
      if (timer) clearTimeout(timer);
      timer = setTimeout(() => fn.apply(this, args), delay);
    }
  }

  /**
   * 节流函数
   * @param {Function} fn - 需要节流的函数
   * @param {number} delay - 延迟时间（毫秒）
   * @returns {Function} 节流后的函数
   */
  function throttle(fn, delay) {
    let last = 0;
    return function(...args) {
      const now = Date.now();
      if (now - last >= delay) {
        last = now;
        fn.apply(this, args);
      }
    }
  }

  /**
   * 创建轮询器
   * @param {Function} fn - 轮询执行的函数
   * @param {number} interval - 轮询间隔（毫秒）
   * @returns {Object} 包含 start/stop 方法的控制器
   */
  function createPoller(fn, interval) {
    let timer = null;
    let running = false;

    return {
      start() {
        if (running) return
        running = true;

        // 立即执行一次
        Promise.resolve(fn()).catch(console.error);

        // 启动定时器
        timer = setInterval(() => {
          Promise.resolve(fn()).catch(console.error);
        }, interval);
      },

      stop() {
        if (!running) return
        running = false;
        if (timer) {
          clearInterval(timer);
          timer = null;
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
  async function retry(fn, options = {}) {
    const {
      maxRetries = 3,
      delay = 1000,
      shouldRetry = () => true
    } = options;

    let lastError;

    for (let i = 0; i <= maxRetries; i++) {
      try {
        return await fn()
      } catch (error) {
        lastError = error;

        if (i < maxRetries && shouldRetry(error)) {
          await sleep(delay * Math.pow(2, i));  // 指数退避
        } else {
          break
        }
      }
    }

    throw lastError
  }

  /**
   * Vea SDK - 统一入口文件
   */


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
  };

  exports.VeaClient = VeaClient;
  exports.VeaError = VeaError;
  exports.createAPI = createAPI;
  exports.createNodeManager = createNodeManager;
  exports.createPoller = createPoller;
  exports.debounce = debounce;
  exports.default = VeaClient;
  exports.escapeHtml = escapeHtml;
  exports.formatBytes = formatBytes;
  exports.formatInterval = formatInterval;
  exports.formatLatency = formatLatency;
  exports.formatSpeed = formatSpeed;
  exports.formatTime = formatTime;
  exports.parseList = parseList;
  exports.parseNumber = parseNumber;
  exports.retry = retry;
  exports.sleep = sleep;
  exports.throttle = throttle;
  exports.utils = utils;

  Object.defineProperty(exports, '__esModule', { value: true });

}));
