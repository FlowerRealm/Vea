import { createAPI, utils } from '../sdk/dist/vea-sdk.esm.js';
const { formatTime, formatBytes, formatInterval, escapeHtml, parseNumber, parseList, sleep } = utils;

(function () {
  const menu = document.getElementById("menu");
  const panels = document.querySelectorAll(".panel");
  const statusBar = document.getElementById("status");
  let currentPanel = "panel-home";
  let statusTimer = null;

  const api = createAPI('');

  function showStatus(message, type = "info", delay = 3200) {
    if (!message) {
      statusBar.classList.remove("visible", "success", "error", "info");
      statusBar.textContent = "";
      return;
    }
    statusBar.textContent = message;
    statusBar.classList.remove("success", "error", "info");
    statusBar.classList.add("visible", type);
    if (statusTimer) {
      clearTimeout(statusTimer);
    }
    if (delay > 0) {
      statusTimer = setTimeout(() => {
        statusBar.classList.remove("visible");
      }, delay);
    }
  }

  const SYSTEM_PROXY_DEFAULTS = ["localhost", "127.0.0.0/8", "::1"];
  let systemProxySettings = {
    enabled: false,
    ignoreHosts: [...SYSTEM_PROXY_DEFAULTS],
  };
  const HOME_PING_COOLDOWN = 60000;
  const HOME_SPEEDTEST_COOLDOWN = 60000;
  const homePingState = {
    running: false,
    lastNodeId: "",
    lastTriggeredAt: 0,
  };
  const homeSpeedtestState = {
    running: false,
    lastNodeId: "",
    lastTriggeredAt: 0,
  };
  let nodeTags = [];
  let currentNodeTab = "全部";

  let xrayStatus = {
    enabled: false,
    running: false,
    activeNodeId: "",
    binary: "",
  };
  let nodesCache = [];
  let componentsCache = [];
  let lastSelectedNodeId = "";
  let preferredNodeId = "";
  let nodesPollHandle = null;
  const NODES_POLL_INTERVAL = 1000;

  function ensureNodesPolling() {
    if (nodesPollHandle) {
      return;
    }
    nodesPollHandle = setInterval(() => {
      loadNodes();
    }, NODES_POLL_INTERVAL);
  }

  async function refreshXrayStatus({ notify = false } = {}) {
    try {
      const status = await api.get("/xray/status");
      if ((!status || !status.binary) && componentsCache.length === 0) {
        try {
          const comps = await api.get("/components");
          componentsCache = Array.isArray(comps) ? comps : [];
        } catch {
          componentsCache = [];
        }
      }
      const normalized = {
        enabled: Boolean(status && status.enabled),
        running: Boolean(status && status.running),
        activeNodeId: (status && status.activeNodeId) || "",
        binary: (status && status.binary) || "",
        config: (status && status.config) || "",
      };
      xrayStatus = normalized;
      if (normalized.activeNodeId) {
        const exists = Array.isArray(nodesCache)
          ? nodesCache.find((item) => item && item.id === normalized.activeNodeId)
          : null;
        if (exists) {
          preferredNodeId = normalized.activeNodeId;
          activeNodeId = normalized.activeNodeId;
        }
      }
      updateXrayUI(normalized);
      updateHomeNodeStatus();
      if (notify) {
        const text = normalized.enabled
          ? normalized.running
            ? "Xray 已启动"
            : "Xray 已启用"
          : "Xray 已停止";
        showStatus(text, "info");
      }
    } catch (err) {
      showStatus(`加载 Xray 状态失败：${err.message}`, "error", 6000);
    }
  }

  function updateXrayUI(status = {}) {
    if (!status || typeof status !== "object") {
      status = {};
    }
    const indicator = document.getElementById("xray-state");
    const proxyStatus = document.getElementById("proxy-status");
    const proxyDesc = document.getElementById("proxy-status-desc");
    const proxyToggle = document.getElementById("proxy-toggle");
    const xrayComponent = Array.isArray(componentsCache)
      ? componentsCache.find((component) => component && component.kind === "xray")
      : null;

    let binaryCandidate = typeof status.binary === "string" ? status.binary.trim() : "";
    if (!binaryCandidate && xrayComponent) {
      const meta = xrayComponent.meta || {};
      if (typeof meta.binary === "string" && meta.binary.trim() != "") {
        binaryCandidate = meta.binary.trim();
      } else if (typeof xrayComponent.installDir === "string" && xrayComponent.installDir.trim() != "") {
        binaryCandidate = `${xrayComponent.installDir.replace(/\/$/, "")}/xray`;
      }
    }
    if (!binaryCandidate) {
      binaryCandidate = "artifacts/core/xray/xray";
    }
    status.binary = binaryCandidate;
    const hasBinary = Boolean(binaryCandidate);

    if (indicator) {
      if (!hasBinary) {
        indicator.className = "badge warn";
        indicator.textContent = "未安装";
      } else if (status.enabled) {
        indicator.className = status.running ? "badge active" : "badge warn";
        indicator.textContent = status.running ? "运行中" : "已启用";
      } else {
        indicator.className = "badge";
        indicator.textContent = "已停止";
      }
    }

    const proxyEnabled = Boolean(systemProxySettings && systemProxySettings.enabled);
    if (!proxyStatus && !proxyToggle && !proxyDesc) {
      return;
    }

    if (!hasBinary) {
      if (proxyStatus) {
        proxyStatus.className = "badge warn";
        proxyStatus.textContent = "未安装";
      }
      if (proxyDesc) {
        proxyDesc.textContent = "尚未检测到可用核心，请先安装 Xray。";
      }
      if (proxyToggle) {
        proxyToggle.disabled = true;
        proxyToggle.dataset.mode = "";
        proxyToggle.classList.remove("active");
      }
      return;
    }

    if (proxyStatus) {
      if (status.enabled && status.running && proxyEnabled) {
        proxyStatus.className = "badge active";
        proxyStatus.textContent = "运行中";
        if (proxyDesc) {
          proxyDesc.textContent = "系统代理已指向 127.0.0.1:38087。";
        }
      } else if (status.enabled && !proxyEnabled) {
        proxyStatus.className = "badge warn";
        proxyStatus.textContent = "核心运行";
        if (proxyDesc) {
          proxyDesc.textContent = "核心已运行，系统代理尚未启用。";
        }
      } else if (status.enabled) {
        proxyStatus.className = "badge warn";
        proxyStatus.textContent = "核心待运行";
        if (proxyDesc) {
          proxyDesc.textContent = "核心正在启动，请稍候。";
        }
      } else {
        proxyStatus.className = "badge";
        proxyStatus.textContent = "已停止";
        if (proxyDesc) {
          proxyDesc.textContent = "点击启动代理即可启动核心并切换系统代理。";
        }
      }
    }

    if (proxyToggle) {
      proxyToggle.disabled = false;
      if (status.enabled && proxyEnabled) {
        proxyToggle.dataset.mode = "stop";
        proxyToggle.classList.add("active");
      } else {
        proxyToggle.dataset.mode = "start";
        proxyToggle.classList.remove("active");
      }
    }
  }

  // Panel loaders
  let activeNodeId = "";

  async function loadNodes({ notify = false } = {}) {
    try {
      const payload = await api.get("/nodes");
      const nodes = Array.isArray(payload.nodes) ? payload.nodes : [];
      const serverActiveNodeId = typeof payload.activeNodeId === "string" ? payload.activeNodeId : "";
      const recentSelectedId = typeof payload.lastSelectedNodeId === "string" ? payload.lastSelectedNodeId : "";
      nodesCache = nodes;
      lastSelectedNodeId = recentSelectedId;

      // Try to restore from localStorage first
      const savedNodeId = localStorage.getItem("vea_selected_node_id");

      const hasNode = (id) => !!id && nodes.some((node) => node && node.id === id);

      // Priority: localStorage > recentSelectedId > serverActiveNodeId > first node
      if (savedNodeId && hasNode(savedNodeId)) {
        preferredNodeId = savedNodeId;
      } else if (hasNode(recentSelectedId)) {
        preferredNodeId = recentSelectedId;
      } else if (hasNode(serverActiveNodeId)) {
        preferredNodeId = serverActiveNodeId;
      } else if (nodes.length > 0 && nodes[0] && nodes[0].id) {
        preferredNodeId = nodes[0].id;
      } else {
        preferredNodeId = "";
      }

      activeNodeId = serverActiveNodeId || preferredNodeId;

      // Save to localStorage
      if (activeNodeId) {
        localStorage.setItem("vea_selected_node_id", activeNodeId);
      }

      updateNodeTabs(nodes);
      renderNodes(nodes, activeNodeId);
      updateHomeNodeStatus();
      refreshXrayStatus();
      if (notify) {
        showStatus("节点列表已刷新", "success");
      }
      if (currentPanel === "panel-home") {
        autoMeasureCurrentNode();
      }
    } catch (err) {
      showStatus(`加载节点失败：${err.message}`, "error", 6000);
      nodesCache = [];
      updateHomeNodeStatus();
    }
  }

  function updateNodeTabs(nodes) {
    if (!nodeTabs) return;
    const tags = new Set();
    nodes.forEach((node) => {
      if (Array.isArray(node.tags)) {
        node.tags.forEach((tag) => {
          const trimmed = String(tag || "").trim();
          if (trimmed) {
            tags.add(trimmed);
          }
        });
      }
    });
    const sorted = ["全部", ...Array.from(tags).sort((a, b) => a.localeCompare(b, "zh-Hans-CN"))];
    nodeTags = sorted;
    if (!nodeTags.includes(currentNodeTab)) {
      currentNodeTab = "全部";
    }
    nodeTabs.innerHTML = nodeTags
      .map((tag) => {
        const active = tag === currentNodeTab ? "active" : "";
      })
      .join("");
  }

  let lastRenderedNodesHash = "";

  function renderNodes(nodes, currentId = "") {
    if (!nodeGrid) return;
    let filtered = nodes;
    if (currentNodeTab !== "全部") {
      filtered = nodes.filter((node) => Array.isArray(node.tags) && node.tags.includes(currentNodeTab));
    }
    if (!Array.isArray(nodes) || nodes.length === 0) {
      nodeGrid.innerHTML = '<div class="empty-card">暂无节点</div>';
      lastRenderedNodesHash = "empty";
      return;
    }

    if (!Array.isArray(filtered) || filtered.length === 0) {
      nodeGrid.innerHTML = '<div class="empty-card">当前标签下暂无节点</div>';
      lastRenderedNodesHash = "empty-filtered";
      return;
    }

    // Create a hash of the current state to detect changes
    const stateHash = JSON.stringify(filtered.map(n => ({
      id: n.id,
      name: n.name,
      address: n.address,
      port: n.port,
      protocol: n.protocol,
      lastLatencyMs: n.lastLatencyMs,
      lastSpeedMbps: n.lastSpeedMbps,
      lastSpeedError: n.lastSpeedError
    }))) + currentId + currentNodeTab;

    // Only re-render if data has changed
    if (stateHash === lastRenderedNodesHash) {
      return;
    }
    lastRenderedNodesHash = stateHash;

    nodeGrid.innerHTML = filtered
      .map((node) => {
        const rowId = escapeHtml(node.id);
        const isActive = currentId && node.id === currentId;
        const protocol = escapeHtml(node.protocol || "unknown");

        // Latency formatting
        let latencyValue = "Ping";
        let latencyClass = "";
        if (typeof node.lastLatencyMs === "number" && node.lastLatencyMs > 0) {
          latencyValue = `${node.lastLatencyMs}ms`;
          if (node.lastLatencyMs < 100) latencyClass = "good";
          else if (node.lastLatencyMs < 300) latencyClass = "fair";
          else latencyClass = "poor";
        }

        // Speed formatting
        let speedValue = "Speed";
        let speedClass = "";
        if (node.lastSpeedError) {
          speedValue = "Error";
          speedClass = "poor";
        } else if (typeof node.lastSpeedMbps === "number" && node.lastSpeedMbps > 0) {
          const fixed = node.lastSpeedMbps >= 10 ? node.lastSpeedMbps.toFixed(1) : node.lastSpeedMbps.toFixed(2);
          speedValue = `${fixed} MB/s`;
          if (node.lastSpeedMbps > 5) speedClass = "good";
          else if (node.lastSpeedMbps > 1) speedClass = "fair";
          else speedClass = "poor";
        }

        return `
          <div class="node-card ${isActive ? "active-node" : ""}" data-id="${rowId}">
            <div class="node-card-header">
              <div class="node-info">
                <div class="node-name">${escapeHtml(node.name)}</div>
                <div class="node-meta">${escapeHtml(node.address)}:${escapeHtml(node.port)}</div>
              </div>
              <div class="node-protocol-badge">${protocol}</div>
            </div>

            <div class="node-metrics-row">
              <div class="node-metric" data-action="ping-node" title="Test Latency">
                <svg viewBox="0 0 24 24" width="14" height="14" stroke="currentColor" stroke-width="2" fill="none" stroke-linecap="round" stroke-linejoin="round"><path d="M22 12h-4l-3 9L9 3l-3 9H2"></path></svg>
                <span class="node-metric-value ${latencyClass}">${latencyValue}</span>
              </div>
              <div class="node-metric" data-action="speed-node" title="Test Speed">
                <svg viewBox="0 0 24 24" width="14" height="14" stroke="currentColor" stroke-width="2" fill="none" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"></polyline></svg>
                <span class="node-metric-value ${speedClass}">${speedValue}</span>
              </div>
            </div>

            <div class="node-actions-overlay">
              <button class="node-delete-btn" data-action="delete-node" title="Delete Node">
                <svg viewBox="0 0 24 24" width="16" height="16" stroke="currentColor" stroke-width="2" fill="none" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"></polyline><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path></svg>
              </button>
            </div>
          </div>
        `;
      })
      .join("");
  }

  async function loadConfigs({ notify = false } = {}) {
    try {
      const configs = await api.get("/configs");
      renderConfigs(configs);
      if (notify) {
        showStatus("配置列表已刷新", "success");
      }
    } catch (err) {
      showStatus(`加载配置失败：${err.message}`, "error", 6000);
    }
  }

  function renderConfigs(configs) {
    const tbody = document.querySelector("#config-table tbody");
    if (!Array.isArray(configs) || configs.length === 0) {
      tbody.innerHTML = '<tr><td colspan="6" class="empty">暂无配置</td></tr>';
      return;
    }
    tbody.innerHTML = configs
      .map((cfg) => {
        const syncState = cfg.lastSyncError
          ? `<span class="badge error">失败：${escapeHtml(cfg.lastSyncError)}</span>`
          : '<span class="badge">正常</span>';
        const source = cfg.sourceUrl ? `<br /><div class="muted text-truncate" title="${escapeHtml(cfg.sourceUrl)}">${escapeHtml(cfg.sourceUrl)}</div>` : "";
        return `
          <tr data-id="${escapeHtml(cfg.id)}">
            <td>${escapeHtml(cfg.name)}${source}</td>
            <td><span class="badge">${escapeHtml(cfg.format)}</span></td>
            <td>${formatInterval(cfg.autoUpdateInterval)}</td>
            <td>${formatTime(cfg.lastSyncedAt)}</td>
            <td>${syncState}</td>
            <td>
              <button class="ghost" data-action="pull-nodes">拉取节点</button>
              <button class="ghost" data-action="refresh-config">刷新</button>
              <button class="danger" data-action="delete-config">删除</button>
            </td>
          </tr>
        `;
      })
      .join("");
  }

  function renderSystemProxy(settings) {
    if (!systemProxyIgnoreInput) return;
    const hosts = Array.isArray(settings.ignoreHosts) ? settings.ignoreHosts : [];
    systemProxyIgnoreInput.value = hosts.join("\n");
  }

  async function loadSystemProxySettings({ notify = false } = {}) {
    if (!systemProxyIgnoreInput) return;
    try {
      const payload = await api.get("/settings/system-proxy");
      const data = payload && payload.settings ? payload.settings : payload;
      systemProxySettings = {
        enabled: Boolean(data.enabled),
        ignoreHosts: Array.isArray(data.ignoreHosts) ? data.ignoreHosts : [...SYSTEM_PROXY_DEFAULTS],
      };
      renderSystemProxy(systemProxySettings);
      updateXrayUI(xrayStatus);
      if (notify) {
        const msg = payload && payload.message ? payload.message : "系统代理设置已刷新";
        showStatus(msg, payload && payload.message ? "info" : "success");
      }
    } catch (err) {
      showStatus(`加载系统代理失败：${err.message}`, "error", 6000);
    }
  }

  function collectIgnoreHosts() {
    if (!systemProxyIgnoreInput) {
      return Array.isArray(systemProxySettings.ignoreHosts)
        ? [...systemProxySettings.ignoreHosts]
        : [...SYSTEM_PROXY_DEFAULTS];
    }
    return systemProxyIgnoreInput
      .value.split(/\r?\n/)
      .map((item) => item.trim())
      .filter(Boolean);
  }

  function updateHomeNodeStatus() {
    if (!homeNodeName || !homeNodeLatency || !homeNodeSpeed) return;

    let node = null;
    if (Array.isArray(nodesCache) && nodesCache.length > 0) {
      const targetId = resolveHomeNodeId();
      if (targetId) {
        node = nodesCache.find((item) => item.id === targetId) || null;
      }
      if (!node) {
        node = nodesCache[0];
      }
    }

    if (!node) {
      homeNodeName.textContent = "Select a Node";
      if (homeNodeAddress) homeNodeAddress.textContent = "-";
      homeNodeLatency.textContent = "--";
      homeNodeSpeed.textContent = "--";
      if (homeNodeUpdated) homeNodeUpdated.textContent = "-";
      return;
    }

    homeNodeName.textContent = escapeHtml(node.name || "-");
    if (homeNodeAddress) {
      homeNodeAddress.textContent = node.address ? `${escapeHtml(node.address)}:${escapeHtml(String(node.port || ""))}` : "-";
    }

    const latency = typeof node.lastLatencyMs === "number" && node.lastLatencyMs > 0 ? `${node.lastLatencyMs} ms` : "--";
    let speed;
    if (node.lastSpeedError) {
      speed = `Error`;
    } else if (typeof node.lastSpeedMbps === "number" && node.lastSpeedMbps > 0) {
      speed = `${node.lastSpeedMbps >= 10 ? node.lastSpeedMbps.toFixed(1) : node.lastSpeedMbps.toFixed(2)} MB/s`;
    } else {
      speed = "--";
    }
    homeNodeLatency.textContent = latency;
    homeNodeSpeed.textContent = speed;

    if (homeNodeUpdated) {
      const timestamps = [node.lastLatencyAt, node.lastSpeedAt].filter(Boolean);
      const updated = timestamps.length > 0 ? timestamps.sort().slice(-1)[0] : null;
      homeNodeUpdated.textContent = updated ? formatTime(updated) : "-";
    }
  }

  function resolveHomeNodeId() {
    if (!Array.isArray(nodesCache) || nodesCache.length === 0) {
      return "";
    }
    if (xrayStatus && xrayStatus.activeNodeId) {
      const active = nodesCache.find((item) => item.id === xrayStatus.activeNodeId);
      if (active && active.id) {
        return active.id;
      }
    }
    if (preferredNodeId) {
      const preferred = nodesCache.find((item) => item.id === preferredNodeId);
      if (preferred && preferred.id) {
        return preferred.id;
      }
    }
    const first = nodesCache[0];
    return (first && first.id) || "";
  }

  async function autoPingCurrentNode(targetId, { force = false } = {}) {
    const nodeId = targetId || resolveHomeNodeId();
    if (!nodeId) {
      return false;
    }
    const now = Date.now();
    if (!force) {
      if (homePingState.running && now - homePingState.lastTriggeredAt < HOME_PING_COOLDOWN) {
        return false;
      }
      if (homePingState.lastNodeId === nodeId && now - homePingState.lastTriggeredAt < HOME_PING_COOLDOWN) {
        return false;
      }
      const node = Array.isArray(nodesCache) ? nodesCache.find((item) => item.id === nodeId) : null;
      if (node) {
        const lastLatencyAt = node.lastLatencyAt ? Date.parse(node.lastLatencyAt) : NaN;
        if (!Number.isNaN(lastLatencyAt) && now - lastLatencyAt < HOME_PING_COOLDOWN) {
          return false;
        }
      }
    }
    homePingState.running = true;
    homePingState.lastNodeId = nodeId;
    homePingState.lastTriggeredAt = now;
    try {
      await api.post(`/nodes/${nodeId}/ping`);
      return true;
    } catch (err) {
      showStatus(`延迟任务失败：${err.message}`, "error", 6000);
      return false;
    } finally {
      homePingState.running = false;
    }
  }

  async function autoSpeedtestCurrentNode(targetId, { force = false } = {}) {
    const nodeId = targetId || resolveHomeNodeId();
    if (!nodeId) {
      return false;
    }
    const now = Date.now();
    if (!force) {
      if (homeSpeedtestState.running && now - homeSpeedtestState.lastTriggeredAt < HOME_SPEEDTEST_COOLDOWN) {
        return false;
      }
      if (homeSpeedtestState.lastNodeId === nodeId && now - homeSpeedtestState.lastTriggeredAt < HOME_SPEEDTEST_COOLDOWN) {
        return false;
      }
      const node = Array.isArray(nodesCache) ? nodesCache.find((item) => item.id === nodeId) : null;
      if (node && (!node.lastSpeedError || node.lastSpeedError.length === 0)) {
        const lastSpeedAt = node.lastSpeedAt ? Date.parse(node.lastSpeedAt) : NaN;
        if (!Number.isNaN(lastSpeedAt) && now - lastSpeedAt < HOME_SPEEDTEST_COOLDOWN) {
          return false;
        }
      }
    }
    homeSpeedtestState.running = true;
    homeSpeedtestState.lastNodeId = nodeId;
    homeSpeedtestState.lastTriggeredAt = now;
    try {
      await api.post(`/nodes/${nodeId}/speedtest`);
      return true;
    } catch (err) {
      showStatus(`测速任务失败：${err.message}`, "error", 6000);
      return false;
    } finally {
      homeSpeedtestState.running = false;
    }
  }

  async function autoMeasureCurrentNode({ force = false } = {}) {
    const targetId = resolveHomeNodeId();
    if (!targetId) {
      return;
    }
    const pingTriggered = await autoPingCurrentNode(targetId, { force });
    if (pingTriggered) {
      await sleep(200);
    }
    await autoSpeedtestCurrentNode(targetId, { force });
  }

  async function loadHomePanel({ notify = false } = {}) {
    await Promise.all([
      loadNodes(),
      loadComponents(),
      refreshXrayStatus({ notify }),
      loadSystemProxySettings({ notify })
    ]);
    updateHomeNodeStatus();
    await autoMeasureCurrentNode({ force: true });
  }

  async function handleProxyToggle() {
    if (!proxyToggleButton || proxyToggleButton.disabled) return;
    const mode = proxyToggleButton.dataset.mode || "start";
    proxyToggleButton.disabled = true;
    try {
      if (mode === "stop") {
        await api.put("/settings/system-proxy", {
          enabled: false,
          ignoreHosts: collectIgnoreHosts(),
        });
        await api.post("/xray/stop");
        showStatus("代理已停止", "info");
      } else {
        const payload = {};
        if (activeNodeId) {
          payload.activeNodeId = activeNodeId;
        }
        if (Object.keys(payload).length > 0) {
          await api.post("/xray/start", payload);
        } else {
          await api.post("/xray/start");
        }
        const response = await api.put("/settings/system-proxy", {
          enabled: true,
          ignoreHosts: collectIgnoreHosts(),
        });
        const msg = response && response.message ? response.message : "代理已启动";
        showStatus(msg, response && response.message ? "info" : "success");
      }
    } catch (err) {
      showStatus(`代理操作失败：${err.message}`, "error", 6000);
    } finally {
      proxyToggleButton.disabled = false;
      await refreshXrayStatus();
      await loadSystemProxySettings();
    }
  }

  function renderComponents(components) {
    const tbody = document.querySelector("#component-table tbody");
    if (!tbody) return;
    if (!Array.isArray(components) || components.length === 0) {
      tbody.innerHTML = '<tr><td colspan="7" class="empty">暂无组件</td></tr>';
      return;
    }
    tbody.innerHTML = components
      .map((component) => {
        const id = escapeHtml(component.id || "");
        const name = escapeHtml(component.name || "-");
        const kind = escapeHtml(componentDisplayName(component.kind));
        const version = escapeHtml(component.lastVersion || "-");
        const installDir = escapeHtml(component.installDir || "-");
        const installedAt = formatTime(component.lastInstalledAt);
        let statusText;
        let actionBtn = '';

        if (component.lastSyncError) {
          statusText = `<span class="badge error">失败：${escapeHtml(component.lastSyncError)}</span>`;
          actionBtn = `<button class="primary" data-action="update-component">重试</button>`;
        } else if (component.installDir) {
          statusText = '<span class="badge">已安装</span>';
          actionBtn = `<button class="ghost" data-action="update-component">更新</button>`;
        } else {
          statusText = '<span class="badge warn">未安装</span>';
          actionBtn = `<button class="primary" data-action="update-component">安装</button>`;
        }

        const interval = formatInterval(component.autoUpdateInterval);
        return `
          <tr data-id="${id}" data-kind="${escapeHtml(component.kind)}">
            <td>${name}</td>
            <td><span class="badge">${kind}</span></td>
            <td>${version}</td>
            <td><span class="muted">${installDir}</span></td>
            <td>${installedAt}</td>
            <td>${statusText}<br /><span class="muted">自动更新：${interval}</span></td>
            <td>
              ${actionBtn}
              <button class="danger" data-action="delete-component">删除</button>
            </td>
          </tr>
        `;
      })
      .join("");
  }

  async function loadComponents({ notify = false } = {}) {
    try {
      const components = await api.get("/components");
      componentsCache = Array.isArray(components) ? components : [];
      renderComponents(components);
      await refreshXrayStatus();
      if (notify) {
        showStatus("组件列表已刷新", "success");
      }
    } catch (err) {
      showStatus(`加载组件失败：${err.message}`, "error", 6000);
      componentsCache = [];
      await refreshXrayStatus();
    }
  }

  function componentDisplayName(kind) {
    switch (kind) {
      case "xray":
        return "Xray";
      default:
        return kind || "组件";
    }
  }

  async function ensureComponent(kind) {
    const pretty = componentDisplayName(kind);
    try {
      let components = [];
      try {
        components = await api.get("/components");
      } catch {
        components = [];
      }
      componentsCache = Array.isArray(components) ? components : [];
      let target = Array.isArray(components) ? components.find((item) => item.kind === kind) : null;
      if (!target) {
        target = await api.post("/components", { kind });
      }
      await api.post(`/components/${target.id}/install`);
      showStatus(`${pretty} 安装任务已触发`, "info", 2400);
      await loadComponents();
      await refreshXrayStatus();
    } catch (err) {
      showStatus(`${pretty} 安装失败：${err.message}`, "error", 6000);
    }
  }

  async function loadTraffic({ notify = false } = {}) {
    try {
      const [profile, rules] = await Promise.all([api.get("/traffic/profile"), api.get("/traffic/rules")]);
      renderTrafficProfile(profile);
      renderTrafficRules(rules);
      if (notify) {
        showStatus("流量策略已刷新", "success");
      }
    } catch (err) {
      showStatus(`加载流量策略失败：${err.message}`, "error", 6000);
    }
  }

  function renderTrafficProfile(profile) {
    const form = document.getElementById("profile-form");
    form.defaultNodeId.value = profile.defaultNodeId || "";
    form.dnsStrategy.value = (profile.dns && profile.dns.strategy) || "";
    form.dnsServers.value = (profile.dns && profile.dns.servers ? profile.dns.servers.join("\n") : "");
  }

  function renderTrafficRules(rules) {
    const tbody = document.querySelector("#rule-table tbody");
    if (!Array.isArray(rules) || rules.length === 0) {
      tbody.innerHTML = '<tr><td colspan="6" class="empty">暂无规则</td></tr>';
      return;
    }
    tbody.innerHTML = rules
      .map((rule) => {
        const targets =
          Array.isArray(rule.targets) && rule.targets.length
            ? rule.targets.map((target) => `<span class="tag">${escapeHtml(target)}</span>`).join("")
            : '<span class="muted">-</span>';
        return `
          <tr data-id="${escapeHtml(rule.id)}">
            <td>${escapeHtml(rule.name)}</td>
            <td>${escapeHtml(rule.nodeId)}</td>
            <td><div class="tags">${targets}</div></td>
            <td>${escapeHtml(rule.priority ?? "-")}</td>
            <td>${formatTime(rule.updatedAt)}</td>
            <td>
              <button class="danger" data-action="delete-rule">删除</button>
            </td>
          </tr>
        `;
      })
      .join("");
  }

  const loaders = {
    "panel-home": () => loadHomePanel(),
    panel1: loadNodes,
    panel2: loadConfigs,
    panel3: () => loadComponents(),
    panel4: loadTraffic,
    "panel-settings": () => loadSystemProxySettings(),
  };

  menu.addEventListener("click", (event) => {
    const button = event.target.closest("button[data-target]");
    if (!button) return;
    const target = button.dataset.target;
    if (target === currentPanel) return;
    currentPanel = target;
    menu.querySelectorAll("button").forEach((btn) => btn.classList.remove("active"));
    button.classList.add("active");
    panels.forEach((panel) => {
      panel.classList.toggle("active", panel.id === target);
    });
    loaders[target]?.();
  });

  const nodeGrid = document.getElementById("node-grid");
  const nodeTabs = document.getElementById("node-tabs");
  const nodeAddButton = document.getElementById("node-add-button");
  const nodeModal = document.getElementById("node-modal");
  const nodeModalClose = document.getElementById("node-modal-close");
  const nodeModalReset = document.getElementById("node-modal-reset");
  const nodeModalBackdrop = nodeModal ? nodeModal.querySelector(".modal-backdrop") : null;
  const nodeForm = document.getElementById("node-form");
  document.getElementById("node-refresh").addEventListener("click", () => loadNodes({ notify: true }));
  document.getElementById("config-refresh").addEventListener("click", () => loadConfigs({ notify: true }));
  document.getElementById("traffic-refresh").addEventListener("click", () => loadTraffic({ notify: true }));
  document.getElementById("component-refresh").addEventListener("click", () => loadComponents({ notify: true }));
  const systemProxyIgnoreInput = document.getElementById("system-proxy-ignore");
  const systemProxySaveButton = document.getElementById("system-proxy-save");
  const systemProxyResetButton = document.getElementById("system-proxy-reset");
  const proxyToggleButton = document.getElementById("proxy-toggle");
  const homeNodeName = document.getElementById("home-node-name");
  const homeNodeAddress = document.getElementById("home-node-address");
  const homeNodeLatency = document.getElementById("home-node-latency");
  const homeNodeSpeed = document.getElementById("home-node-speed");
  const homeNodeProtocol = document.getElementById("home-node-protocol");
  const homeNodeUpdated = document.getElementById("home-node-updated");
  if (proxyToggleButton) {
    proxyToggleButton.addEventListener("click", handleProxyToggle);
  }

  function openNodeModal() {
    if (!nodeModal) return;
    nodeModal.classList.add("open");
  }

  function closeNodeModal() {
    if (!nodeModal) return;
    nodeModal.classList.remove("open");
  }

  if (nodeAddButton) {
    nodeAddButton.addEventListener("click", openNodeModal);
  }
  if (nodeModalClose) {
    nodeModalClose.addEventListener("click", closeNodeModal);
  }
  if (nodeModalBackdrop) {
    nodeModalBackdrop.addEventListener("click", closeNodeModal);
  }
  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && nodeModal?.classList.contains("open")) {
      closeNodeModal();
    }
  });
  if (nodeModalReset && nodeForm) {
    nodeModalReset.addEventListener("click", () => {
      nodeForm.reset();
    });
  }

  // Handle protocol and network-specific field visibility
  const protocolSelect = document.getElementById("protocol-select");
  const networkSelect = document.getElementById("network-select");
  const tlsSelect = nodeForm?.querySelector('select[name="tls"]');

  function updateFieldVisibility() {
    if (!protocolSelect) return;

    const protocol = protocolSelect.value;
    const network = networkSelect?.value || "tcp";
    const tls = tlsSelect?.value || "";

    // Hide all protocol-specific fields first
    const ssFields = document.getElementById("ss-fields");
    const vmessFields = document.getElementById("vmess-fields");
    const trojanFields = document.getElementById("trojan-fields");
    const proxyFields = document.getElementById("proxy-fields");

    if (ssFields) ssFields.style.display = "none";
    if (vmessFields) vmessFields.style.display = "none";
    if (trojanFields) trojanFields.style.display = "none";
    if (proxyFields) proxyFields.style.display = "none";

    // Show relevant protocol fields
    if (protocol === "shadowsocks" && ssFields) {
      ssFields.style.display = "grid";
    } else if ((protocol === "vmess" || protocol === "vless") && vmessFields) {
      vmessFields.style.display = "grid";
    } else if (protocol === "trojan" && trojanFields) {
      trojanFields.style.display = "block";
    } else if ((protocol === "http" || protocol === "socks5") && proxyFields) {
      proxyFields.style.display = "grid";
    }

    // Hide all network-specific fields first
    const wsFields = document.getElementById("ws-fields");
    const h2Fields = document.getElementById("h2-fields");
    const grpcFields = document.getElementById("grpc-fields");

    if (wsFields) wsFields.style.display = "none";
    if (h2Fields) h2Fields.style.display = "none";
    if (grpcFields) grpcFields.style.display = "none";

    // Show relevant network fields
    if (network === "ws" && wsFields) {
      wsFields.style.display = "grid";
    } else if (network === "h2" && h2Fields) {
      h2Fields.style.display = "grid";
    } else if (network === "grpc" && grpcFields) {
      grpcFields.style.display = "block";
    }

    // Show/hide TLS fields
    const tlsFields = document.getElementById("tls-fields");
    if (tlsFields) {
      tlsFields.style.display = (tls && tls !== "") ? "grid" : "none";
    }
  }

  if (protocolSelect) {
    protocolSelect.addEventListener("change", updateFieldVisibility);
  }
  if (networkSelect) {
    networkSelect.addEventListener("change", updateFieldVisibility);
  }
  if (tlsSelect) {
    tlsSelect.addEventListener("change", updateFieldVisibility);
  }

  // Handle advanced settings toggles
  const enableNetworkCheckbox = document.getElementById("enable-network");
  const enableTlsCheckbox = document.getElementById("enable-tls");
  const networkSettings = document.getElementById("network-settings");
  const tlsSettings = document.getElementById("tls-settings");

  if (enableNetworkCheckbox && networkSettings) {
    enableNetworkCheckbox.addEventListener("change", (e) => {
      networkSettings.style.display = e.target.checked ? "grid" : "none";
      if (e.target.checked) {
        updateFieldVisibility();
      }
    });
  }

  if (enableTlsCheckbox && tlsSettings) {
    enableTlsCheckbox.addEventListener("change", (e) => {
      tlsSettings.style.display = e.target.checked ? "grid" : "none";
    });
  }

  if (nodeForm) {
    nodeForm.addEventListener("submit", async (event) => {
      event.preventDefault();
      const form = event.target;

      // Check for share link first
      const shareLink = form.shareLink.value.trim();
      if (shareLink) {
        try {
          await api.post("/nodes", { shareLink });
          form.reset();
          showStatus("节点添加成功", "success");
          closeNodeModal();
          await loadNodes();
        } catch (err) {
          showStatus(`添加节点失败：${err.message}`, "error", 6000);
        }
        return;
      }

      // Build payload from form fields
      const protocol = form.protocol.value.trim();
      if (!protocol) {
        showStatus("请选择协议", "error");
        return;
      }

      const payload = {
        name: form.name.value.trim(),
        address: form.address.value.trim(),
        port: parseNumber(form.port.value),
        protocol: protocol,
        tags: parseList(form.tags.value),
        remarks: form.remarks.value.trim() || "",
      };

      if (!payload.name || !payload.address || !payload.port) {
        showStatus("请完整填写节点信息", "error");
        return;
      }

      // Protocol-specific settings
      if (protocol === "shadowsocks") {
        payload.password = form.ss_password.value.trim();
        payload.method = form.ss_method.value;
      } else if (protocol === "vmess" || protocol === "vless") {
        payload.uuid = form.vmess_uuid.value.trim();
        if (protocol === "vmess") {
          payload.alterId = parseNumber(form.vmess_alterid.value) || 0;
          payload.security = form.vmess_security.value;
        } else if (protocol === "vless") {
          payload.flow = form.vless_flow.value || "";
        }
      } else if (protocol === "trojan") {
        payload.password = form.trojan_password.value.trim();
      } else if (protocol === "http" || protocol === "socks5") {
        payload.username = form.proxy_username.value.trim() || "";
        payload.password = form.proxy_password.value.trim() || "";
      }

      // Network settings
      payload.network = form.network.value || "tcp";
      payload.tls = form.tls.value || "";

      // Network-specific settings
      if (payload.network === "ws") {
        payload.path = form.ws_path.value.trim() || "/";
        payload.host = form.ws_host.value.trim() || "";
      } else if (payload.network === "h2") {
        payload.path = form.h2_path.value.trim() || "/";
        payload.host = form.h2_host.value.trim() || "";
      } else if (payload.network === "grpc") {
        payload.serviceName = form.grpc_service.value.trim() || "";
      }

      // TLS settings
      if (payload.tls) {
        payload.sni = form.tls_sni.value.trim() || "";
        payload.fingerprint = form.tls_fingerprint.value || "";
      }

      try {
        await api.post("/nodes", payload);
        form.reset();
        updateFieldVisibility(); // Reset field visibility
        showStatus("节点添加成功", "success");
        closeNodeModal();
        await loadNodes();
      } catch (err) {
        showStatus(`添加节点失败：${err.message}`, "error", 6000);
      }
    });
  }

  function updateHomeNodeStatus() {
    if (!homeNodeName || !homeNodeLatency || !homeNodeSpeed) return;

    let node = null;
    if (Array.isArray(nodesCache) && nodesCache.length > 0) {
      const targetId = resolveHomeNodeId();
      if (targetId) {
        node = nodesCache.find((item) => item.id === targetId) || null;
      }
      if (!node) {
        node = nodesCache[0];
      }
    }

    if (!node) {
      homeNodeName.textContent = "选择节点";
      if (homeNodeAddress) homeNodeAddress.textContent = "-";
      homeNodeLatency.textContent = "--";
      homeNodeSpeed.textContent = "--";
      if (homeNodeProtocol) homeNodeProtocol.textContent = "--";
      if (homeNodeUpdated) homeNodeUpdated.textContent = "-";
      return;
    }

    homeNodeName.textContent = escapeHtml(node.name || "-");
    if (homeNodeAddress) {
      homeNodeAddress.textContent = node.address ? `${escapeHtml(node.address)}:${escapeHtml(String(node.port || ""))}` : "-";
    }

    const latency = typeof node.lastLatencyMs === "number" && node.lastLatencyMs > 0 ? `${node.lastLatencyMs} ms` : "--";
    let speed;
    if (node.lastSpeedError) {
      speed = `Error`;
    } else if (typeof node.lastSpeedMbps === "number" && node.lastSpeedMbps > 0) {
      speed = `${node.lastSpeedMbps >= 10 ? node.lastSpeedMbps.toFixed(1) : node.lastSpeedMbps.toFixed(2)} MB/s`;
    } else {
      speed = "--";
    }
    homeNodeLatency.textContent = latency;
    homeNodeSpeed.textContent = speed;

    // Display protocol
    if (homeNodeProtocol) {
      const protocol = node.protocol ? escapeHtml(node.protocol).toUpperCase() : "--";
      homeNodeProtocol.textContent = protocol;
    }

    if (homeNodeUpdated) {
      const timestamps = [node.lastLatencyAt, node.lastSpeedAt].filter(Boolean);
      const updated = timestamps.length > 0 ? timestamps.sort().slice(-1)[0] : null;
      homeNodeUpdated.textContent = updated ? formatTime(updated) : "-";
    }
  }

  if (nodeGrid) {
    nodeGrid.addEventListener("click", async (event) => {
      const card = event.target.closest(".node-card[data-id]");
      if (!card) return;
      const id = card.dataset.id;
      const actionTarget = event.target.closest("[data-action]");
      const action = actionTarget ? actionTarget.dataset.action : "select-node";
      try {
        if (action === "delete-node") {
          if (!confirm("确认删除该节点？")) return;
          await api.delete(`/nodes/${id}`);
          showStatus("节点已删除", "success");
          await loadNodes();
        } else if (action === "ping-node") {
          await api.post(`/nodes/${id}/ping`);
          showStatus("延迟任务已排队", "info");
          await loadNodes();
        } else if (action === "speed-node") {
          await api.post(`/nodes/${id}/speedtest`);
          showStatus("测速任务已排队", "info");
          await loadNodes();
        } else if (action === "select-node") {
          await api.post(`/nodes/${id}/select`);
          localStorage.setItem("vea_selected_node_id", id);
          showStatus("已切换当前节点", "success");
          await loadNodes();
        }
      } catch (err) {
        showStatus(`操作失败：${err.message}`, "error", 6000);
      }
    });
  }

  if (nodeTabs) {
    nodeTabs.addEventListener("click", (event) => {
      const button = event.target.closest(".node-tab[data-tag]");
      if (!button) return;
      const tag = button.dataset.tag;
      if (tag === currentNodeTab) return;
      currentNodeTab = tag;
      nodeTabs.querySelectorAll(".node-tab").forEach((tab) => tab.classList.remove("active"));
      button.classList.add("active");
      renderNodes(nodesCache, activeNodeId);
    });
  }

  // Traffic Tabs Logic
  const trafficTabs = document.getElementById("traffic-tabs");
  if (trafficTabs) {
    trafficTabs.addEventListener("click", (event) => {
      const button = event.target.closest(".node-tab[data-tab]");
      if (!button) return;
      const tabName = button.dataset.tab;

      // Update active tab state
      trafficTabs.querySelectorAll(".node-tab").forEach(t => t.classList.remove("active"));
      button.classList.add("active");

      // Show/Hide content
      const globalContent = document.getElementById("traffic-content-global");
      const rulesContent = document.getElementById("traffic-content-rules");

      if (tabName === "global") {
        if (globalContent) globalContent.style.display = "block";
        if (rulesContent) rulesContent.style.display = "none";
      } else {
        if (globalContent) globalContent.style.display = "none";
        if (rulesContent) rulesContent.style.display = "block";
      }
    });
  }

  const configModal = document.getElementById("config-modal");
  const configAddButton = document.getElementById("config-add-button");
  const configModalClose = document.getElementById("config-modal-close");
  const configModalBackdrop = configModal?.querySelector(".modal-backdrop");
  const configModalReset = document.getElementById("config-modal-reset");
  const configForm = document.getElementById("config-form");

  function openConfigModal() {
    if (!configModal) return;
    configModal.classList.add("open");
    if (configForm) configForm.reset();
  }

  function closeConfigModal() {
    if (!configModal) return;
    configModal.classList.remove("open");
  }

  if (configAddButton) {
    configAddButton.addEventListener("click", openConfigModal);
  }
  if (configModalClose) {
    configModalClose.addEventListener("click", closeConfigModal);
  }
  if (configModalBackdrop) {
    configModalBackdrop.addEventListener("click", closeConfigModal);
  }
  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && configModal?.classList.contains("open")) {
      closeConfigModal();
    }
  });
  if (configModalReset && configForm) {
    configModalReset.addEventListener("click", () => {
      configForm.reset();
    });
  }

  if (configForm) {
    configForm.addEventListener("submit", async (event) => {
      event.preventDefault();
      const form = event.target;
      let inferredFormat = "";
      const typedPayload = (form.payload.value || "").trim();
      const sourceUrl = form.sourceUrl.value.trim();
      const detectSample = typedPayload || sourceUrl;
      if (detectSample) {
        const lower = detectSample.toLowerCase();
        if (/\"outbounds\"/.test(detectSample) || /\"inbounds\"/.test(detectSample) || /vmess|trojan|vless|ss:/.test(lower)) {
          inferredFormat = "xray-json";
        }
      }
      const payload = {
        name: form.name.value.trim(),
        format: inferredFormat || "xray-json",
        sourceUrl,
        payload: typedPayload,
        autoUpdateIntervalMinutes: parseNumber(form.autoUpdateInterval.value),
      };
      if (!payload.sourceUrl) {
        showStatus("请填写源/订阅链接", "error", 5000);
        return;
      }
      try {
        await api.post("/configs/import", payload);
        form.reset();
        showStatus("配置添加成功", "success");
        closeConfigModal();
        await Promise.all([loadConfigs(), loadNodes()]);
      } catch (err) {
        showStatus(`添加配置失败：${err.message}`, "error", 6000);
      }
    });
  }

  function getSelectedNodeIds() {
    if (!nodeGrid) return [];
    return Array.from(nodeGrid.querySelectorAll(".node-card[data-id]"))
      .map((card) => card.dataset.id)
      .filter(Boolean);
  }

  document.getElementById("node-bulk-ping").addEventListener("click", async () => {
    const ids = getSelectedNodeIds();
    if (ids.length === 0) {
      showStatus("请先勾选节点", "error");
      return;
    }
    try {
      await api.post("/nodes/bulk/ping", { ids });
      showStatus("批量延迟任务已排队", "info");
      await loadNodes();
    } catch (err) {
      showStatus(`批量延迟失败：${err.message}`, "error", 6000);
    }
  });

  document.getElementById("node-bulk-speed").addEventListener("click", async () => {
    const ids = getSelectedNodeIds();
    if (ids.length === 0) {
      showStatus("请先勾选节点", "error");
      return;
    }
    try {
      await api.post("/nodes/reset-speed", { ids });
      await loadNodes();
      for (const id of ids) {
        await api.post(`/nodes/${id}/speedtest`);
      }
      showStatus("批量测速任务已排队", "info");
      await loadNodes();
    } catch (err) {
      showStatus(`批量测速失败：${err.message}`, "error", 6000);
    }
  });

  document.getElementById("config-table").addEventListener("click", async (event) => {
    const button = event.target.closest("button[data-action]");
    if (!button) return;
    const tr = button.closest("tr[data-id]");
    if (!tr) return;
    const id = tr.dataset.id;
    const action = button.dataset.action;
    try {
      if (action === "refresh-config") {
        await api.post(`/configs/${id}/refresh`);
        showStatus("配置刷新完成", "success");
        await Promise.all([loadConfigs(), loadNodes()]);
      } else if (action === "pull-nodes") {
        const nodes = await api.post(`/configs/${id}/pull-nodes`);
        renderNodes(nodes);
        showStatus(`订阅节点已同步（${nodes.length}）`, "success");
        await Promise.all([loadConfigs(), loadNodes()]);
      } else if (action === "delete-config") {
        if (!confirm("确认删除该配置？")) return;
        await api.delete(`/configs/${id}`);
        showStatus("配置已删除", "success");
        await Promise.all([loadConfigs(), loadNodes()]);
      }
    } catch (err) {
      showStatus(`操作失败：${err.message}`, "error", 6000);
    }
  });

  if (systemProxySaveButton) {
    systemProxySaveButton.addEventListener("click", async () => {
      try {
        systemProxySaveButton.disabled = true;
        const payload = {
          enabled: Boolean(systemProxySettings && systemProxySettings.enabled),
          ignoreHosts: collectIgnoreHosts(),
        };
        const response = await api.put("/settings/system-proxy", payload);
        const data = response && response.settings ? response.settings : response;
        systemProxySettings = {
          enabled: Boolean(data.enabled),
          ignoreHosts: Array.isArray(data.ignoreHosts) ? data.ignoreHosts : [...SYSTEM_PROXY_DEFAULTS],
        };
        renderSystemProxy(systemProxySettings);
        updateXrayUI(xrayStatus);
        const msg = response && response.message ? response.message : "系统代理设置已保存";
        showStatus(msg, response && response.message ? "info" : "success");
      } catch (err) {
        showStatus(`保存系统代理失败：${err.message}`, "error", 6000);
      } finally {
        systemProxySaveButton.disabled = false;
      }
    });
  }

  if (systemProxyResetButton) {
    systemProxyResetButton.addEventListener("click", () => {
      systemProxySettings = {
        ...systemProxySettings,
        ignoreHosts: [...SYSTEM_PROXY_DEFAULTS],
      };
      renderSystemProxy(systemProxySettings);
      updateXrayUI(xrayStatus);
    });
  }

  renderSystemProxy(systemProxySettings);

  const componentTable = document.getElementById("component-table");
  if (componentTable) {
    componentTable.addEventListener("click", async (event) => {
      const button = event.target.closest("button[data-action]");
      if (!button) return;
      const tr = button.closest("tr[data-id]");
      if (!tr) return;
      const id = tr.dataset.id;
      const kind = tr.dataset.kind;
      const action = button.dataset.action;
      try {
        if (action === "delete-component") {
          if (!confirm("确认删除该组件？")) return;
          await api.delete(`/components/${id}`);
          showStatus("组件已删除", "success");
          await loadComponents();
          await refreshXrayStatus();
        } else if (action === "update-component") {
          if (kind) {
            await ensureComponent(kind);
          } else {
            showStatus("无法识别组件类型", "error");
          }
        }
      } catch (err) {
        showStatus(`组件操作失败：${err.message}`, "error", 6000);
      }
    });
  }

  document.querySelectorAll(".ensure-component").forEach((button) => {
    button.addEventListener("click", async () => {
      const kind = button.dataset.kind;
      if (!kind) return;
      button.disabled = true;
      try {
        await ensureComponent(kind);
      } finally {
        button.disabled = false;
      }
    });
  });

  document.getElementById("profile-form").addEventListener("submit", async (event) => {
    event.preventDefault();
    const form = event.target;
    const payload = {
      defaultNodeId: form.defaultNodeId.value.trim(),
      dns: {
        strategy: form.dnsStrategy.value.trim(),
        servers: parseList(form.dnsServers.value),
      },
    };
    try {
      const updated = await api.put("/traffic/profile", payload);
      renderTrafficProfile(updated);
      showStatus("默认出口与 DNS 已更新", "success");
    } catch (err) {
      showStatus(`更新失败：${err.message}`, "error", 6000);
    }
  });

  const ruleModal = document.getElementById("rule-modal");
  const ruleAddButton = document.getElementById("rule-add-button");
  const ruleModalClose = document.getElementById("rule-modal-close");
  const ruleModalBackdrop = ruleModal?.querySelector(".modal-backdrop");
  const ruleModalReset = document.getElementById("rule-modal-reset");
  const ruleForm = document.getElementById("rule-form");

  function openRuleModal() {
    if (!ruleModal) return;
    ruleModal.classList.add("open");
    if (ruleForm) ruleForm.reset();
  }

  function closeRuleModal() {
    if (!ruleModal) return;
    ruleModal.classList.remove("open");
  }

  if (ruleAddButton) {
    ruleAddButton.addEventListener("click", openRuleModal);
  }
  if (ruleModalClose) {
    ruleModalClose.addEventListener("click", closeRuleModal);
  }
  if (ruleModalBackdrop) {
    ruleModalBackdrop.addEventListener("click", closeRuleModal);
  }
  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && ruleModal?.classList.contains("open")) {
      closeRuleModal();
    }
  });
  if (ruleModalReset && ruleForm) {
    ruleModalReset.addEventListener("click", () => {
      ruleForm.reset();
    });
  }

  if (ruleForm) {
    ruleForm.addEventListener("submit", async (event) => {
      event.preventDefault();
      const form = event.target;
      const payload = {
        name: form.name.value.trim(),
        priority: parseNumber(form.priority.value),
        targets: parseList(form.targets.value),
        nodeId: form.nodeId.value.trim(),
      };
      if (!payload.name || !payload.targets.length || !payload.nodeId) {
        showStatus("请完整填写规则信息", "error");
        return;
      }
      try {
        await api.post("/traffic/rules", payload);
        form.reset();
        showStatus("路由规则添加成功", "success");
        closeRuleModal();
        await loadTrafficRules();
      } catch (err) {
        showStatus(`添加规则失败：${err.message}`, "error", 6000);
      }
    });
  }

  document.getElementById("rule-table").addEventListener("click", async (event) => {
    const button = event.target.closest("button[data-action]");
    if (!button) return;
    const tr = button.closest("tr[data-id]");
    if (!tr) return;
    const id = tr.dataset.id;
    const action = button.dataset.action;
    if (action !== "delete-rule") return;
    try {
      if (!confirm("确认删除该分流规则？")) return;
      await api.delete(`/traffic/rules/${id}`);
      showStatus("规则已删除", "success");
      await loadTraffic();
    } catch (err) {
      showStatus(`删除失败：${err.message}`, "error", 6000);
    }
  });

  // Initial load
  loadNodes();
  loaders[currentPanel]?.();
  ensureNodesPolling();
})();

// 窗口控制按钮事件
if (window.electronAPI) {
  document.getElementById('minimize-btn').addEventListener('click', () => {
    window.electronAPI.minimizeWindow();
  });

  document.getElementById('maximize-btn').addEventListener('click', () => {
    window.electronAPI.maximizeWindow();
  });

  document.getElementById('close-btn').addEventListener('click', () => {
    window.electronAPI.closeWindow();
  });
}

// Theme settings - Switch between HTML files
// Theme selector
const themeSelector = document.getElementById("theme-selector");

function switchTheme(theme) {
  localStorage.setItem("theme", theme);
  const file = theme === "dark" ? "dark.html" : "light.html";
  window.location.href = file;
}

// Get current theme from filename
const currentFile = window.location.pathname.split('/').pop();
const currentTheme = currentFile.includes('light') ? 'light' : 'dark';

// Check if saved theme is different from current loaded theme
const savedTheme = localStorage.getItem("theme") || "dark";
if (savedTheme !== currentTheme) {
  // Auto-redirect to saved theme
  switchTheme(savedTheme);
}

// Set selector value to current theme
if (themeSelector) {
  themeSelector.value = currentTheme;

  // Listen for theme changes
  themeSelector.addEventListener("change", (e) => {
    switchTheme(e.target.value);
  });
}
