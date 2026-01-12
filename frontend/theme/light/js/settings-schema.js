/**
 * 设置 Schema 定义
 * 使用 JSON 结构描述所有设置项，前端根据此 schema 动态渲染 UI
 *
 * 设置项类型 (type):
 * - text: 文本输入
 * - number: 数字输入
 * - toggle: 开关
 * - select: 下拉选择
 * - textarea: 多行文本
 * - tags: 标签输入（逗号分隔）
 *
 * 每个设置项的 key 对应后端 API 的字段路径
 */

const SETTINGS_SCHEMA = {
  version: "1.0",
  categories: [
    {
      id: "general",
      name: "外观",
      icon: "settings",
      settings: [
        {
          key: "theme",
          type: "select",
          label: "选择主题",
          options: [
            { value: "dark", label: "深色主题" },
            { value: "light", label: "浅色主题" }
          ],
          default: "dark"
        }
      ]
    },
    {
      id: "proxy",
      name: "系统代理",
      icon: "globe",
      settings: [
        {
          key: "proxy.port",
          type: "number",
          label: "代理端口",
          description: "本地代理监听端口（Mixed：sing-box/mihomo 同端口）",
          default: 31346,
          min: 1024,
          max: 65535
        },
        {
          key: "systemProxy.bypass",
          type: "textarea",
          label: "绕过列表",
          description: "不走代理的地址，每行一个",
          default: "localhost\n127.*\n10.*\n172.16.*\n172.17.*\n172.18.*\n172.19.*\n172.20.*\n172.21.*\n172.22.*\n172.23.*\n172.24.*\n172.25.*\n172.26.*\n172.27.*\n172.28.*\n172.29.*\n172.30.*\n172.31.*\n192.168.*\n<local>"
        }
      ]
    },
    {
      id: "tun",
      name: "TUN",
      icon: "network",
      settings: [
        {
          key: "tun.interfaceName",
          type: "text",
          label: "接口名称",
          default: "vea",
          readonly: true
        },
        {
          key: "tun.mtu",
          type: "number",
          label: "MTU",
          default: 9000,
          min: 1280,
          max: 65535
        },
        {
          key: "tun.stack",
          type: "select",
          label: "TCP/IP 协议栈",
          options: [
            { value: "system", label: "System (系统)" },
            { value: "gvisor", label: "gVisor (用户空间)" },
            { value: "mixed", label: "Mixed (混合，推荐)" }
          ],
          default: "mixed"
        },
        {
          key: "tun.autoRoute",
          type: "toggle",
          label: "自动路由",
          description: "接管系统流量",
          default: true
        },
        {
          key: "tun.strictRoute",
          type: "toggle",
          label: "严格路由",
          description: "防止流量泄漏",
          default: true
        },
        {
          key: "tun.dnsHijack",
          type: "toggle",
          label: "DNS 劫持",
          description: "劫持所有 DNS 请求",
          default: true
        },
        {
          key: "tun.autoRedirect",
          type: "toggle",
          label: "Auto Redirect",
          description: "使用 nftables 提供更高性能（需要 nftables）",
          default: false,
          platform: "linux"
        },
        {
          key: "tun.endpointIndependentNat",
          type: "toggle",
          label: "端点独立 NAT",
          description: "gVisor stack 选项（可能影响性能）",
          default: false
        },
        {
          key: "tun.udpTimeout",
          type: "number",
          label: "UDP 超时（秒）",
          default: 300,
          min: 30,
          max: 3600
        }
      ]
    },
    {
      id: "singbox",
      name: "sing-box",
      icon: "box",
      settings: [
        {
          key: "singbox.tcpFastOpen",
          type: "toggle",
          label: "TCP Fast Open",
          description: "减少 TCP 握手时间（可能有兼容性问题）",
          group: "性能优化",
          default: false
        },
        {
          key: "singbox.tcpMultiPath",
          type: "toggle",
          label: "TCP MultiPath",
          description: "启用 MPTCP（需要系统支持）",
          group: "性能优化",
          default: false
        },
        {
          key: "singbox.udpFragment",
          type: "toggle",
          label: "UDP 分片",
          description: "支持 UDP 分片传输",
          group: "性能优化",
          default: false
        },
        {
          key: "singbox.domainStrategy",
          type: "select",
          label: "域名策略",
          group: "路由配置",
          options: [
            { value: "prefer_ipv4", label: "优先 IPv4" },
            { value: "prefer_ipv6", label: "优先 IPv6" },
            { value: "ipv4_only", label: "仅 IPv4" },
            { value: "ipv6_only", label: "仅 IPv6" }
          ],
          default: "prefer_ipv4"
        },
        {
          key: "singbox.domainMatcher",
          type: "select",
          label: "域名匹配器",
          group: "路由配置",
          options: [
            { value: "hybrid", label: "Hybrid (平衡)" },
            { value: "linear", label: "Linear (低内存)" }
          ],
          default: "hybrid"
        }
      ]
    },
    {
      id: "advanced",
      name: "高级",
      icon: "sliders",
      settings: [
        // 入站配置
        {
          key: "inbound.listen",
          type: "text",
          label: "监听地址",
          group: "入站配置",
          default: "127.0.0.1"
        },
        {
          key: "inbound.port",
          type: "number",
          label: "端口",
          group: "入站配置",
          default: 38087,
          min: 1024,
          max: 65535
        },
        {
          key: "inbound.allowLan",
          type: "toggle",
          label: "允许局域网连接",
          description: "监听 0.0.0.0",
          group: "入站配置",
          default: false
        },
        {
          key: "inbound.sniff",
          type: "toggle",
          label: "域名嗅探",
          description: "自动识别目标域名",
          group: "入站配置",
          default: true
        },
        // 日志配置
        {
          key: "log.level",
          type: "select",
          label: "日志级别",
          group: "日志配置",
          options: [
            { value: "debug", label: "Debug (调试)" },
            { value: "info", label: "Info (信息)" },
            { value: "warning", label: "Warning (警告)" },
            { value: "error", label: "Error (错误)" },
            { value: "none", label: "None (关闭)" }
          ],
          default: "info"
        },
        {
          key: "log.timestamp",
          type: "toggle",
          label: "显示时间戳",
          group: "日志配置",
          default: true
        }
      ]
    }
  ]
};

/**
 * 设置管理器
 */
class SettingsManager {
  constructor(schema) {
    this.schema = schema;
    this.values = {};
    this.listeners = new Map();
    this.loadDefaults();
  }

  // 加载默认值
  loadDefaults() {
    for (const category of this.schema.categories) {
      for (const setting of category.settings) {
        this.values[setting.key] = setting.default;
      }
    }
  }

  // 获取设置值
  get(key) {
    return this.values[key];
  }

  // 设置值
  set(key, value) {
    const oldValue = this.values[key];
    this.values[key] = value;

    // 触发监听器
    const listeners = this.listeners.get(key) || [];
    for (const fn of listeners) {
      fn(value, oldValue, key);
    }
  }

  // 监听设置变化
  on(key, callback) {
    if (!this.listeners.has(key)) {
      this.listeners.set(key, []);
    }
    this.listeners.get(key).push(callback);
  }

  // 导出设置为 JSON
  exportJSON() {
    const exportData = {
      version: this.schema.version,
      exportedAt: new Date().toISOString(),
      settings: {}
    };

    for (const category of this.schema.categories) {
      for (const setting of category.settings) {
        // 只导出非默认值，减少文件大小
        if (this.values[setting.key] !== setting.default) {
          exportData.settings[setting.key] = this.values[setting.key];
        }
      }
    }

    return JSON.stringify(exportData, null, 2);
  }

  // 导入设置
  importJSON(jsonString) {
    try {
      const data = JSON.parse(jsonString);

      if (!data.settings) {
        throw new Error("Invalid settings file format");
      }

      // 先重置为默认值
      this.loadDefaults();

      // 应用导入的值
      for (const [key, value] of Object.entries(data.settings)) {
        if (this.values.hasOwnProperty(key)) {
          this.set(key, value);
        }
      }

      return { success: true, imported: Object.keys(data.settings).length };
    } catch (e) {
      return { success: false, error: e.message };
    }
  }

  // 从后端 API 加载设置
  async loadFromAPI(apiBaseUrl = '') {
    try {
      const response = await fetch(`${apiBaseUrl}/settings/frontend`);
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
      }
      const settings = await response.json();

      // 应用加载的值
      for (const [key, value] of Object.entries(settings)) {
        if (this.values.hasOwnProperty(key)) {
          this.values[key] = value; // 直接设置，不触发监听器
        }
      }

      return { success: true, loaded: Object.keys(settings).length };
    } catch (e) {
      console.warn('[Settings] 从 API 加载设置失败:', e.message);
      return { success: false, error: e.message };
    }
  }

  // 保存设置到后端 API
  async saveToAPI(apiBaseUrl = '') {
    try {
      // 只保存非默认值
      const toSave = {};
      for (const category of this.schema.categories) {
        for (const setting of category.settings) {
          if (this.values[setting.key] !== setting.default) {
            toSave[setting.key] = this.values[setting.key];
          }
        }
      }

      const response = await fetch(`${apiBaseUrl}/settings/frontend`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(toSave)
      });

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
      }

      return { success: true };
    } catch (e) {
      console.error('[Settings] 保存设置到 API 失败:', e.message);
      return { success: false, error: e.message };
    }
  }

  // 重置为默认值
  resetToDefaults() {
    for (const category of this.schema.categories) {
      for (const setting of category.settings) {
        this.set(setting.key, setting.default);
      }
    }
  }

  // 获取某个分类的设置
  getCategorySettings(categoryId) {
    const category = this.schema.categories.find(c => c.id === categoryId);
    return category ? category.settings : [];
  }

  // 按 group 分组设置项
  getGroupedSettings(categoryId) {
    const settings = this.getCategorySettings(categoryId);
    const groups = new Map();

    for (const setting of settings) {
      const groupName = setting.group || "默认";
      if (!groups.has(groupName)) {
        groups.set(groupName, []);
      }
      groups.get(groupName).push(setting);
    }

    return groups;
  }
}

/**
 * 设置渲染器
 */
class SettingsRenderer {
  constructor(manager) {
    this.manager = manager;
  }

  // 渲染单个设置项
  renderSetting(setting) {
    const value = this.manager.get(setting.key);
    const id = `setting-${setting.key.replace(/\./g, '-')}`;

    switch (setting.type) {
      case 'toggle':
        return this.renderToggle(setting, id, value);
      case 'select':
        return this.renderSelect(setting, id, value);
      case 'number':
        return this.renderNumber(setting, id, value);
      case 'text':
        return this.renderText(setting, id, value);
      case 'textarea':
        return this.renderTextarea(setting, id, value);
      case 'tags':
        return this.renderTags(setting, id, value);
      default:
        return '';
    }
  }

  renderToggle(setting, id, value) {
    return `
      <div style="grid-column:1/-1; display:flex; justify-content:space-between; align-items:center; padding:8px 0;">
        <div>
          <div style="font-size:13px; font-weight:500;">${setting.label}</div>
          ${setting.description ? `<div style="font-size:12px; color:var(--text-secondary); margin-top:2px;">${setting.description}</div>` : ''}
        </div>
        <label class="switch">
          <input type="checkbox" id="${id}" data-key="${setting.key}" ${value ? 'checked' : ''}>
          <span class="slider"></span>
        </label>
      </div>
    `;
  }

  renderSelect(setting, id, value) {
    const options = setting.options.map(opt =>
      `<option value="${opt.value}" ${value === opt.value ? 'selected' : ''}>${opt.label}</option>`
    ).join('');

    return `
      <label style="grid-column:${setting.gridColumn || '1/2'};">
        <span>${setting.label}</span>
        <select id="${id}" data-key="${setting.key}">
          ${options}
        </select>
        ${setting.description ? `<div style="font-size:11px; color:var(--text-tertiary); margin-top:4px;">${setting.description}</div>` : ''}
      </label>
    `;
  }

  renderNumber(setting, id, value) {
    const attrs = [];
    if (setting.min !== undefined) attrs.push(`min="${setting.min}"`);
    if (setting.max !== undefined) attrs.push(`max="${setting.max}"`);
    if (setting.readonly) attrs.push('readonly style="background:var(--bg-tertiary);"');

    return `
      <label style="grid-column:${setting.gridColumn || '1/2'};">
        <span>${setting.label}</span>
        <input type="number" id="${id}" data-key="${setting.key}" value="${value}" ${attrs.join(' ')}>
        ${setting.description ? `<div style="font-size:11px; color:var(--text-tertiary); margin-top:4px;">${setting.description}</div>` : ''}
      </label>
    `;
  }

  renderText(setting, id, value) {
    const attrs = [];
    if (setting.readonly) attrs.push('readonly style="background:var(--bg-tertiary);"');
    if (setting.placeholder) attrs.push(`placeholder="${setting.placeholder}"`);

    return `
      <label style="grid-column:${setting.gridColumn || '1/2'};">
        <span>${setting.label}</span>
        <input type="text" id="${id}" data-key="${setting.key}" value="${value || ''}" ${attrs.join(' ')}>
        ${setting.description ? `<div style="font-size:11px; color:var(--text-tertiary); margin-top:4px;">${setting.description}</div>` : ''}
      </label>
    `;
  }

  renderTextarea(setting, id, value) {
    return `
      <label style="grid-column:1/-1;">
        <span>${setting.label}</span>
        <textarea id="${id}" data-key="${setting.key}" style="height:120px;">${value || ''}</textarea>
        ${setting.description ? `<div style="font-size:11px; color:var(--text-tertiary); margin-top:4px;">${setting.description}</div>` : ''}
      </label>
    `;
  }

  renderTags(setting, id, value) {
    return `
      <label style="grid-column:1/-1;">
        <span>${setting.label}</span>
        <input type="text" id="${id}" data-key="${setting.key}" value="${value || ''}" placeholder="用逗号分隔">
        ${setting.description ? `<div style="font-size:11px; color:var(--text-tertiary); margin-top:4px;">${setting.description}</div>` : ''}
      </label>
    `;
  }

  // 渲染分组的设置
  renderGroup(groupName, settings) {
    const settingsHtml = settings.map(s => this.renderSetting(s)).join('');
    return `
      <div class="card form-grid">
        <h3 style="grid-column:1/-1; font-size:14px; color:var(--text-primary);">${groupName}</h3>
        ${settingsHtml}
      </div>
    `;
  }

  // 渲染整个分类页面
  renderCategory(categoryId) {
    const groups = this.manager.getGroupedSettings(categoryId);
    let html = '';

    for (const [groupName, settings] of groups) {
      html += this.renderGroup(groupName, settings);
    }

    return html;
  }

  // 绑定事件
  bindEvents(container) {
    // 监听所有设置项变化
    container.querySelectorAll('[data-key]').forEach(el => {
      const key = el.dataset.key;

      el.addEventListener('change', () => {
        let value;
        if (el.type === 'checkbox') {
          value = el.checked;
        } else if (el.type === 'number') {
          value = parseInt(el.value, 10);
        } else {
          value = el.value;
        }
        this.manager.set(key, value);
      });
    });
  }
}

// 导出
if (typeof module !== 'undefined' && module.exports) {
  module.exports = { SETTINGS_SCHEMA, SettingsManager, SettingsRenderer };
}
