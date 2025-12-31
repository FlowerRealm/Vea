/**
 * @vea/sdk v1.0.0
 * (c) 2025 Vea Project
 * @license MIT
 */
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
    super(message);
    this.name = 'VeaError';
    this.statusCode = statusCode;
    this.response = response;
  }
}

class VeaClient {
  constructor(options = {}) {
    this.baseURL = (options.baseURL || 'http://localhost:19080').replace(/\/$/, '');
    this.timeout = options.timeout || 300000; // 5分钟
    this.headers = options.headers || {};

    this.nodes = new NodesAPI(this);
    this.frouters = new FRoutersAPI(this);
    this.configs = new ConfigsAPI(this);
    this.geo = new GeoAPI(this);
    this.components = new ComponentsAPI(this);
    this.settings = new SettingsAPI(this);
    this.proxy = new ProxyAPI(this);
  }

  async request(options) {
    const {
      method = 'GET',
      path,
      body,
      headers = {},
      timeout = this.timeout
    } = options;

    const url = `${this.baseURL}${path}`;
    const requestHeaders = { ...this.headers, ...headers };

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

    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), timeout);
    requestOptions.signal = controller.signal;

    try {
      const response = await fetch(url, requestOptions);
      clearTimeout(timeoutId);
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

  async _handleResponse(response) {
    if (response.status === 204) {
      return null
    }

    const contentType = response.headers.get('content-type') || '';
    let data;

    if (contentType.includes('application/json')) {
      data = await response.json();
    } else {
      data = await response.text();
    }

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

class FRoutersAPI {
  constructor(client) {
    this.client = client;
  }

  async list() {
    return this.client.get('/frouters')
  }

  async create(data) {
    return this.client.post('/frouters', data)
  }

  async update(id, data) {
    return this.client.put(`/frouters/${id}`, data)
  }

  async delete(id) {
    return this.client.delete(`/frouters/${id}`)
  }

  async ping(id) {
    return this.client.post(`/frouters/${id}/ping`)
  }

  async speedtest(id) {
    return this.client.post(`/frouters/${id}/speedtest`)
  }

  async measureLatency(id) {
    return this.client.post(`/frouters/${id}/ping`)
  }

  async measureSpeed(id) {
    return this.client.post(`/frouters/${id}/speedtest`)
  }

  async bulkPing(ids = []) {
    return this.client.post('/frouters/bulk/ping', { ids })
  }

  async resetSpeed(ids = []) {
    return this.client.post('/frouters/reset-speed', { ids })
  }

  async getGraph(id) {
    return this.client.get(`/frouters/${id}/graph`)
  }

  async saveGraph(id, data) {
    return this.client.put(`/frouters/${id}/graph`, data)
  }

  async validateGraph(id, data) {
    return this.client.post(`/frouters/${id}/graph/validate`, data)
  }
}

class NodesAPI {
  constructor(client) {
    this.client = client;
  }

  async list() {
    return this.client.get('/nodes')
  }

  async bulkPing(ids = []) {
    return this.client.post('/nodes/bulk/ping', { ids })
  }

  async bulkSpeedtest(ids = []) {
    return this.client.post('/nodes/bulk/speedtest', { ids })
  }

  async ping(id) {
    return this.client.post(`/nodes/${id}/ping`)
  }

  async speedtest(id) {
    return this.client.post(`/nodes/${id}/speedtest`)
  }
}

class ConfigsAPI {
  constructor(client) {
    this.client = client;
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
}

class GeoAPI {
  constructor(client) {
    this.client = client;
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
    this.client = client;
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

class ProxyAPI {
  constructor(client) {
    this.client = client;
  }

  async getConfig() {
    return this.client.get('/proxy/config')
  }

  async updateConfig(data) {
    return this.client.put('/proxy/config', data)
  }

  async status() {
    return this.client.get('/proxy/status')
  }

  async start(data = {}) {
    return this.client.post('/proxy/start', data)
  }

  async stop() {
    return this.client.post('/proxy/stop')
  }
}

class SettingsAPI {
  constructor(client) {
    this.client = client;
  }

  async getSystemProxy() {
    return this.client.get('/settings/system-proxy')
  }

  async updateSystemProxy(data) {
    return this.client.put('/settings/system-proxy', data)
  }
}

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

    // Expose all client APIs
    nodes: client.nodes,
    frouters: client.frouters,
    configs: client.configs,
    geo: client.geo,
    components: client.components,
    settings: client.settings,
    proxy: client.proxy,

    client
  }
}

// ============================================================================
// Utility Functions
// ============================================================================

function formatTime(value) {
  if (!value) return '-'
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return String(value)
  return date.toLocaleString()
}

function formatBytes(bytes) {
  const size = Number(bytes);
  if (!size || size <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const index = Math.min(Math.floor(Math.log(size) / Math.log(1024)), units.length - 1);
  const value = size / Math.pow(1024, index);
  return `${value.toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}

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
  if (mbps >= 10) return `${mbps.toFixed(1)} Mbps`
  return `${mbps.toFixed(2)} Mbps`
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
  const num = Number(value);
  return Number.isFinite(num) ? num : 0
}

function debounce(fn, delay) {
  let timer = null;
  return function (...args) {
    if (timer) clearTimeout(timer);
    timer = setTimeout(() => fn.apply(this, args), delay);
  }
}

function throttle(fn, delay) {
  let last = 0;
  return function (...args) {
    const now = Date.now();
    if (now - last >= delay) {
      last = now;
      fn.apply(this, args);
    }
  }
}

function createPoller(fn, interval) {
  let timer = null;
  let running = false;

  return {
    start() {
      if (running) return
      running = true;
      Promise.resolve(fn()).catch(console.error);
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
        await sleep(delay * Math.pow(2, i));
      } else {
        break
      }
    }
  }

  throw lastError
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
};

export { VeaClient, VeaError, createAPI, createPoller, debounce, VeaClient as default, escapeHtml, formatBytes, formatInterval, formatLatency, formatSpeed, formatTime, parseList, parseNumber, retry, sleep, throttle, utils };
