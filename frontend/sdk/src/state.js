/**
 * Vea SDK - State Management
 *
 * 状态管理和业务逻辑
 */

/**
 * 创建节点状态管理器
 * @param {Object} options - 配置选项
 * @param {number} options.pingCooldown - Ping冷却时间（毫秒，默认60秒）
 * @param {number} options.speedtestCooldown - 测速冷却时间（毫秒，默认60秒）
 * @returns {Object} 状态管理器
 */
export function createNodeStateManager(options = {}) {
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
    /**
     * 检查是否可以ping节点
     */
    canPing(nodeId, node = null, force = false) {
      if (force) return true

      const now = Date.now()

      // 检查全局运行状态
      if (pingState.running && now - pingState.lastTriggeredAt < pingCooldown) {
        return false
      }

      // 检查特定节点的冷却
      if (pingState.lastNodeId === nodeId && now - pingState.lastTriggeredAt < pingCooldown) {
        return false
      }

      // 检查节点最后测试时间
      if (node) {
        const lastLatencyAt = node.lastLatencyAt ? Date.parse(node.lastLatencyAt) : NaN
        if (!Number.isNaN(lastLatencyAt) && now - lastLatencyAt < pingCooldown) {
          return false
        }
      }

      return true
    },

    /**
     * 标记ping开始
     */
    startPing(nodeId) {
      pingState.running = true
      pingState.lastNodeId = nodeId
      pingState.lastTriggeredAt = Date.now()
    },

    /**
     * 标记ping结束
     */
    endPing() {
      pingState.running = false
    },

    /**
     * 检查是否可以测速
     */
    canSpeedtest(nodeId, node = null, force = false) {
      if (force) return true

      const now = Date.now()

      // 检查全局运行状态
      if (speedtestState.running && now - speedtestState.lastTriggeredAt < speedtestCooldown) {
        return false
      }

      // 检查特定节点的冷却
      if (speedtestState.lastNodeId === nodeId && now - speedtestState.lastTriggeredAt < speedtestCooldown) {
        return false
      }

      // 检查节点最后测试时间
      if (node && (!node.lastSpeedError || node.lastSpeedError.length === 0)) {
        const lastSpeedAt = node.lastSpeedAt ? Date.parse(node.lastSpeedAt) : NaN
        if (!Number.isNaN(lastSpeedAt) && now - lastSpeedAt < speedtestCooldown) {
          return false
        }
      }

      return true
    },

    /**
     * 标记测速开始
     */
    startSpeedtest(nodeId) {
      speedtestState.running = true
      speedtestState.lastNodeId = nodeId
      speedtestState.lastTriggeredAt = Date.now()
    },

    /**
     * 标记测速结束
     */
    endSpeedtest() {
      speedtestState.running = false
    }
  }
}

/**
 * 解析首选节点ID
 * @param {Object} options - 选项
 * @param {Array} options.nodes - 节点列表
 * @param {string} options.activeNodeId - 服务器激活的节点ID
 * @param {string} options.lastSelectedNodeId - 最后选择的节点ID
 * @param {string} options.savedNodeId - localStorage保存的节点ID
 * @returns {string} 首选节点ID
 */
export function resolvePreferredNode(options) {
  const { nodes, activeNodeId, lastSelectedNodeId, savedNodeId } = options

  if (!Array.isArray(nodes) || nodes.length === 0) {
    return ''
  }

  const hasNode = (id) => !!id && nodes.some((node) => node && node.id === id)

  // 优先级: localStorage > lastSelectedNodeId > activeNodeId > 第一个节点
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

/**
 * 创建localStorage节点ID管理器
 * @param {string} key - 存储key（默认'vea_selected_node_id'）
 * @returns {Object} 管理器
 */
export function createNodeIdStorage(key = 'vea_selected_node_id') {
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

/**
 * 创建主题管理器
 * @returns {Object} 主题管理器
 */
export function createThemeManager() {
  const STORAGE_KEY = 'theme'
  const THEMES = {
    DARK: 'dark',
    LIGHT: 'light'
  }

  return {
    /**
     * 获取当前主题
     */
    getCurrent() {
      try {
        return localStorage.getItem(STORAGE_KEY) || THEMES.DARK
      } catch {
        return THEMES.DARK
      }
    },

    /**
     * 切换主题
     */
    switch(theme) {
      try {
        localStorage.setItem(STORAGE_KEY, theme)
        const file = theme === THEMES.DARK ? 'dark.html' : 'light.html'
        window.location.href = file
      } catch (err) {
        console.error('Failed to switch theme:', err)
      }
    },

    /**
     * 从文件名检测主题
     */
    detectFromFilename() {
      const currentFile = window.location.pathname.split('/').pop()
      return currentFile.includes('light') ? THEMES.LIGHT : THEMES.DARK
    },

    /**
     * 自动重定向到保存的主题
     */
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

/**
 * 提取节点标签
 * @param {Array} nodes - 节点列表
 * @returns {Array<string>} 排序后的标签列表（包含"全部"）
 */
export function extractNodeTags(nodes) {
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

/**
 * 过滤节点
 * @param {Array} nodes - 节点列表
 * @param {string} tag - 标签（'全部'表示不过滤）
 * @returns {Array} 过滤后的节点列表
 */
export function filterNodesByTag(nodes, tag) {
  if (!tag || tag === '全部') {
    return nodes
  }

  return nodes.filter((node) =>
    Array.isArray(node.tags) && node.tags.includes(tag)
  )
}

export default {
  createNodeStateManager,
  resolvePreferredNode,
  createNodeIdStorage,
  createThemeManager,
  extractNodeTags,
  filterNodesByTag
}
