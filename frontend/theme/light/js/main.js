import { createAPI, utils } from './vea-sdk.esm.js';
    const { formatTime, formatBytes, formatInterval, escapeHtml, parseNumber, parseList, sleep } = utils;

    (function () {
      const menu = document.getElementById("menu");
      const panels = document.querySelectorAll(".panel");
      const statusBar = document.getElementById("status");
      let currentPanel = "panel-home";
      let statusTimer = null;

      const api = createAPI('http://127.0.0.1:19080');

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
      let frouterTags = [];
      let currentFRouterTab = "全部";

      let coreStatus = {
        enabled: false,
        running: false,
        binary: "",
      };
	      let froutersCache = [];
	      let nodesCache = [];
	      let configsCache = [];
	      let currentFRouterId = "";
	      let componentsCache = [];
	      let froutersPollHandle = null;
	      let nodesPollHandle = null;
	      let kernelLogsPollHandle = null;
	      let kernelLogOffset = 0;
	      let kernelLogSession = 0;
	      let kernelLogStartedAt = "";
	      let appLogsPollHandle = null;
	      let appLogOffset = 0;
	      let appLogStartedAt = "";
	      const ANSI_ESC = "\x1b";
	      const ANSI_ESC_SYMBOL = "␛";
	      let kernelLogAnsiState = createAnsiState();
	      let kernelLogAnsiCarry = "";
	      let appLogAnsiState = createAnsiState();
	      let appLogAnsiCarry = "";
	      let currentLogsTab = "kernel";
	      let kernelLogAutoScroll = true;
	      let kernelLogsPanelInitialized = false;
	      let appLogAutoScroll = true;
	      let appLogsPanelInitialized = false;
	      let logsTabsInitialized = false;
	      let nodesLoadInFlight = false;
		      let homeAutoTestInFlight = false;
		      let applyFRouterInFlight = false;
		      let applyFRouterPendingId = "";
		      const FROUTERS_POLL_INTERVAL = 1000;
		      const NODES_POLL_INTERVAL = 100; // 0.1s
		      const KERNEL_LOGS_POLL_INTERVAL = 800;
		      const FROUTER_STORAGE_KEY = "vea_selected_frouter_id";

	      function createAnsiState() {
	        return {
	          fg: null,
	          bg: null,
	          bold: false,
	          italic: false,
	          underline: false,
	        };
	      }

	      function resetKernelLogRender(pre) {
	        kernelLogAnsiState = createAnsiState();
	        kernelLogAnsiCarry = "";
	        if (pre) pre.innerHTML = "";
	      }

	      function resetAppLogRender(pre) {
	        appLogAnsiState = createAnsiState();
	        appLogAnsiCarry = "";
	        if (pre) pre.innerHTML = "";
	      }

	      function clampByte(value) {
	        const n = Number(value);
	        if (!Number.isFinite(n)) return 0;
	        if (n < 0) return 0;
	        if (n > 255) return 255;
	        return Math.floor(n);
	      }

	      function xterm256ToCss(n) {
	        const code = Number(n);
	        if (!Number.isFinite(code)) return null;
	        const idx = Math.max(0, Math.min(255, Math.floor(code)));

	        const base16 = [
	          [0, 0, 0],
	          [205, 0, 0],
	          [0, 205, 0],
	          [205, 205, 0],
	          [0, 0, 238],
	          [205, 0, 205],
	          [0, 205, 205],
	          [229, 229, 229],
	          [127, 127, 127],
	          [255, 0, 0],
	          [0, 255, 0],
	          [255, 255, 0],
	          [92, 92, 255],
	          [255, 0, 255],
	          [0, 255, 255],
	          [255, 255, 255],
	        ];

	        let rgb = null;
	        if (idx < 16) {
	          rgb = base16[idx];
	        } else if (idx <= 231) {
	          const c = idx - 16;
	          const r = Math.floor(c / 36);
	          const g = Math.floor((c % 36) / 6);
	          const b = c % 6;
	          const steps = [0, 95, 135, 175, 215, 255];
	          rgb = [steps[r], steps[g], steps[b]];
	        } else {
	          const v = 8 + (idx - 232) * 10;
	          rgb = [v, v, v];
	        }
	        return `rgb(${rgb[0]},${rgb[1]},${rgb[2]})`;
	      }

	      function ansiStateToStyle(state) {
	        if (!state) return "";
	        let style = "";
	        if (state.fg) style += `color:${state.fg};`;
	        if (state.bg) style += `background-color:${state.bg};`;
	        if (state.bold) style += "font-weight:600;";
	        if (state.italic) style += "font-style:italic;";
	        if (state.underline) style += "text-decoration:underline;";
	        return style;
	      }

	      function wrapAnsiHtml(escapedText, state) {
	        if (!escapedText) return "";
	        const style = ansiStateToStyle(state);
	        if (!style) return escapedText;
	        return `<span style="${style}">${escapedText}</span>`;
	      }

	      function applySgrSequence(params, state) {
	        const parts = params ? params.split(";") : ["0"];
	        let i = 0;
	        while (i < parts.length) {
	          const raw = parts[i];
	          const code = raw === "" ? 0 : Number(raw);
	          if (!Number.isFinite(code)) {
	            i += 1;
	            continue;
	          }

	          if (code === 0) {
	            state.fg = null;
	            state.bg = null;
	            state.bold = false;
	            state.italic = false;
	            state.underline = false;
	            i += 1;
	            continue;
	          }

	          if (code === 1) {
	            state.bold = true;
	            i += 1;
	            continue;
	          }
	          if (code === 22) {
	            state.bold = false;
	            i += 1;
	            continue;
	          }
	          if (code === 3) {
	            state.italic = true;
	            i += 1;
	            continue;
	          }
	          if (code === 23) {
	            state.italic = false;
	            i += 1;
	            continue;
	          }
	          if (code === 4) {
	            state.underline = true;
	            i += 1;
	            continue;
	          }
	          if (code === 24) {
	            state.underline = false;
	            i += 1;
	            continue;
	          }
	          if (code === 39) {
	            state.fg = null;
	            i += 1;
	            continue;
	          }
	          if (code === 49) {
	            state.bg = null;
	            i += 1;
	            continue;
	          }

	          if (code >= 30 && code <= 37) {
	            state.fg = xterm256ToCss(code - 30);
	            i += 1;
	            continue;
	          }
	          if (code >= 90 && code <= 97) {
	            state.fg = xterm256ToCss(8 + (code - 90));
	            i += 1;
	            continue;
	          }
	          if (code >= 40 && code <= 47) {
	            state.bg = xterm256ToCss(code - 40);
	            i += 1;
	            continue;
	          }
	          if (code >= 100 && code <= 107) {
	            state.bg = xterm256ToCss(8 + (code - 100));
	            i += 1;
	            continue;
	          }

	          if (code === 38 || code === 48) {
	            const isFg = code === 38;
	            const mode = Number(parts[i + 1]);
	            if (mode === 5) {
	              const css = xterm256ToCss(parts[i + 2]);
	              if (css) {
	                if (isFg) state.fg = css;
	                else state.bg = css;
	              }
	              i += 3;
	              continue;
	            }
	            if (mode === 2) {
	              const r = clampByte(parts[i + 2]);
	              const g = clampByte(parts[i + 3]);
	              const b = clampByte(parts[i + 4]);
	              const css = `rgb(${r},${g},${b})`;
	              if (isFg) state.fg = css;
	              else state.bg = css;
	              i += 5;
	              continue;
	            }
	          }

	          i += 1;
	        }
	      }

	      function ansiToHtmlChunk(text, state) {
	        const src = String(text || "");
	        let out = "";
	        let segStart = 0;
	        let i = 0;

	        while (i < src.length) {
	          const ch = src[i];
	          if (ch !== ANSI_ESC && ch !== ANSI_ESC_SYMBOL) {
	            i += 1;
	            continue;
	          }

	          if (i > segStart) {
	            out += wrapAnsiHtml(escapeHtml(src.slice(segStart, i)), state);
	          }

	          if (i + 1 >= src.length) {
	            return { html: out, carry: src.slice(i) };
	          }
	          if (src[i + 1] !== "[") {
	            i += 1;
	            segStart = i;
	            continue;
	          }

	          let j = i + 2;
	          while (j < src.length) {
	            const c = src[j];
	            if (c >= "@" && c <= "~") break;
	            j += 1;
	          }
	          if (j >= src.length) {
	            return { html: out, carry: src.slice(i) };
	          }

	          const final = src[j];
	          const params = src.slice(i + 2, j);
	          if (final === "m") {
	            applySgrSequence(params, state);
	          }
	          i = j + 1;
	          segStart = i;
	        }

	        if (segStart < src.length) {
	          out += wrapAnsiHtml(escapeHtml(src.slice(segStart)), state);
	        }
	        return { html: out, carry: "" };
	      }

	      function appendKernelLogChunk(pre, chunk) {
	        if (!pre || !chunk) return;
	        const { html, carry } = ansiToHtmlChunk(kernelLogAnsiCarry + chunk, kernelLogAnsiState);
	        kernelLogAnsiCarry = carry || "";
	        if (html) {
	          pre.insertAdjacentHTML("beforeend", html);
	        }
	      }

	      function appendAppLogChunk(pre, chunk) {
	        if (!pre || !chunk) return;
	        const { html, carry } = ansiToHtmlChunk(appLogAnsiCarry + chunk, appLogAnsiState);
	        appLogAnsiCarry = carry || "";
	        if (html) {
	          pre.insertAdjacentHTML("beforeend", html);
	        }
	      }

	      function ensureFRoutersPolling() {
	        if (froutersPollHandle) {
	          return;
	        }
	        froutersPollHandle = setInterval(() => {
	          loadFRouters();
	        }, FROUTERS_POLL_INTERVAL);
	      }

	      function startNodesPolling() {
	        if (nodesPollHandle) {
	          return;
	        }
	        nodesPollHandle = setInterval(() => {
	          if (currentPanel === "panel-nodes") {
	            loadNodes();
	          }
	        }, NODES_POLL_INTERVAL);
	      }

	      function stopNodesPolling() {
	        if (!nodesPollHandle) {
	          return;
	        }
	        clearInterval(nodesPollHandle);
	        nodesPollHandle = null;
	      }

	      function ensureKernelLogsPanel() {
	        if (kernelLogsPanelInitialized) {
	          return;
	        }
	        kernelLogsPanelInitialized = true;

	        const refreshBtn = document.getElementById("kernel-log-refresh");
	        const copyBtn = document.getElementById("kernel-log-copy");
	        const autoScrollToggle = document.getElementById("kernel-log-autoscroll");
	        const logContainer = document.getElementById("kernel-log-container");

	        if (autoScrollToggle) {
	          kernelLogAutoScroll = Boolean(autoScrollToggle.checked);
	          autoScrollToggle.addEventListener("change", (e) => {
	            kernelLogAutoScroll = Boolean(e.target.checked);
	            if (kernelLogAutoScroll && logContainer) {
	              logContainer.scrollTop = logContainer.scrollHeight;
	            }
	          });
	        }

	        if (refreshBtn) {
	          refreshBtn.addEventListener("click", async () => {
	            kernelLogOffset = 0;
	            kernelLogSession = 0;
	            kernelLogStartedAt = "";
	            const pre = document.getElementById("kernel-log-content");
	            resetKernelLogRender(pre);
	            await loadKernelLogs();
	          });
	        }

	        if (copyBtn) {
	          copyBtn.addEventListener("click", async () => {
	            const pre = document.getElementById("kernel-log-content");
	            const text = pre ? pre.textContent : "";
	            if (!text) {
	              showStatus("暂无可复制内容", "info", 1500);
	              return;
	            }
	            try {
	              await navigator.clipboard.writeText(text);
	              showStatus("日志已复制到剪贴板", "success", 1600);
	            } catch (err) {
	              showStatus(`复制失败：${err.message}`, "error", 3000);
	            }
	          });
	        }
	      }

	      function ensureAppLogsPanel() {
	        if (appLogsPanelInitialized) {
	          return;
	        }
	        appLogsPanelInitialized = true;

	        const refreshBtn = document.getElementById("app-log-refresh");
	        const copyBtn = document.getElementById("app-log-copy");
	        const autoScrollToggle = document.getElementById("app-log-autoscroll");
	        const logContainer = document.getElementById("app-log-container");

	        if (autoScrollToggle) {
	          appLogAutoScroll = Boolean(autoScrollToggle.checked);
	          autoScrollToggle.addEventListener("change", (e) => {
	            appLogAutoScroll = Boolean(e.target.checked);
	            if (appLogAutoScroll && logContainer) {
	              logContainer.scrollTop = logContainer.scrollHeight;
	            }
	          });
	        }

	        if (refreshBtn) {
	          refreshBtn.addEventListener("click", async () => {
	            appLogOffset = 0;
	            appLogStartedAt = "";
	            const pre = document.getElementById("app-log-content");
	            resetAppLogRender(pre);
	            await loadAppLogs();
	          });
	        }

	        if (copyBtn) {
	          copyBtn.addEventListener("click", async () => {
	            const pre = document.getElementById("app-log-content");
	            const text = pre ? pre.textContent : "";
	            if (!text) {
	              showStatus("暂无可复制内容", "info", 1500);
	              return;
	            }
	            try {
	              await navigator.clipboard.writeText(text);
	              showStatus("日志已复制到剪贴板", "success", 1600);
	            } catch (err) {
	              showStatus(`复制失败：${err.message}`, "error", 3000);
	            }
	          });
	        }
	      }

	      function syncLogsTabUI() {
	        const tabs = document.getElementById("log-tabs");
	        const kernelCard = document.getElementById("kernel-log-card");
	        const appCard = document.getElementById("app-log-card");

	        if (kernelCard) {
	          kernelCard.style.display = currentLogsTab === "kernel" ? "" : "none";
	        }
	        if (appCard) {
	          appCard.style.display = currentLogsTab === "app" ? "" : "none";
	        }

	        if (tabs) {
	          tabs.querySelectorAll(".node-tab[data-log-tab]").forEach((tab) => {
	            tab.classList.toggle("active", tab.dataset.logTab === currentLogsTab);
	          });
	        }
	      }

	      function setLogsTab(tab) {
	        const next = tab === "app" ? "app" : "kernel";
	        if (next === currentLogsTab) {
	          return;
	        }
	        currentLogsTab = next;
	        syncLogsTabUI();
	        if (currentPanel === "panel-logs") {
	          if (currentLogsTab === "app") {
	            loadAppLogs();
	          } else {
	            loadKernelLogs();
	          }
	        }
	      }

	      function ensureLogsTabs() {
	        if (logsTabsInitialized) {
	          return;
	        }
	        logsTabsInitialized = true;
	        const tabs = document.getElementById("log-tabs");
	        if (!tabs) {
	          return;
	        }
	        tabs.addEventListener("click", (event) => {
	          const button = event.target.closest(".node-tab[data-log-tab]");
	          if (!button) {
	            return;
	          }
	          setLogsTab(button.dataset.logTab);
	        });
	        syncLogsTabUI();
	      }

	      function startKernelLogsPolling() {
	        if (kernelLogsPollHandle) {
	          return;
	        }
	        kernelLogsPollHandle = setInterval(() => {
	          if (currentPanel === "panel-logs" && currentLogsTab === "kernel") {
	            loadKernelLogs();
	          }
	        }, KERNEL_LOGS_POLL_INTERVAL);
	      }

	      function startAppLogsPolling() {
	        if (appLogsPollHandle) {
	          return;
	        }
	        appLogsPollHandle = setInterval(() => {
	          if (currentPanel === "panel-logs" && currentLogsTab === "app") {
	            loadAppLogs();
	          }
	        }, KERNEL_LOGS_POLL_INTERVAL);
	      }

	      function stopKernelLogsPolling() {
	        if (!kernelLogsPollHandle) {
	          return;
	        }
	        clearInterval(kernelLogsPollHandle);
	        kernelLogsPollHandle = null;
	      }

	      function stopAppLogsPolling() {
	        if (!appLogsPollHandle) {
	          return;
	        }
	        clearInterval(appLogsPollHandle);
	        appLogsPollHandle = null;
	      }

	      async function loadKernelLogs(options = {}) {
	        const metaEl = document.getElementById("kernel-log-meta");
	        const pre = document.getElementById("kernel-log-content");
	        const logContainer = document.getElementById("kernel-log-container");
	        if (!pre) return;

	        try {
	          const forceFromStart = Boolean(options && options.forceFromStart);
	          const since = forceFromStart ? 0 : kernelLogOffset;
	          const payload = await api.get(`/proxy/kernel/logs?since=${since}`);
	          const data = payload || {};
	          const session = Number(data.session || 0);
	          const startedAt = data.startedAt || "";
	          const lost = Boolean(data.lost);
	          const chunk = data.text || "";

	          const sessionChanged = session && session !== kernelLogSession;
	          const startedAtChanged = startedAt && startedAt !== kernelLogStartedAt;

	          // 新 session 不应从旧 offset 续读：否则会丢掉“kernel start ...”等关键起始信息。
	          if (!forceFromStart && (sessionChanged || startedAtChanged) && kernelLogOffset !== 0) {
	            kernelLogOffset = 0;
	            await loadKernelLogs({ forceFromStart: true });
	            return;
	          }

	          const shouldReset = lost || sessionChanged || startedAtChanged || kernelLogOffset === 0;
	          if (shouldReset) {
	            resetKernelLogRender(pre);
	          }
	          appendKernelLogChunk(pre, chunk);

	          kernelLogSession = session || kernelLogSession;
	          kernelLogStartedAt = startedAt || kernelLogStartedAt;
	          if (typeof data.to === "number") {
	            kernelLogOffset = data.to;
	          }

	          if (metaEl) {
	            const parts = [];
	            if (data.engine) {
	              parts.push(componentDisplayName(String(data.engine)));
	            } else {
	              parts.push("内核");
	            }
	            if (data.running === false) parts.push("已停止");
	            if (data.pid) parts.push(`pid ${data.pid}`);
	            if (startedAt) parts.push(`启动于 ${formatTime(startedAt)}`);
	            if (data.error) parts.push(`错误：${data.error}`);
	            metaEl.textContent = parts.length ? parts.join(" · ") : "-";
	          }

	          if (kernelLogAutoScroll && logContainer) {
	            logContainer.scrollTop = logContainer.scrollHeight;
	          }
	        } catch (err) {
	          if (metaEl) {
	            metaEl.textContent = `加载失败：${err.message}`;
	          }
	        }
	      }

	      async function loadAppLogs(options = {}) {
	        const metaEl = document.getElementById("app-log-meta");
	        const pre = document.getElementById("app-log-content");
	        const logContainer = document.getElementById("app-log-container");
	        if (!pre) return;

	        try {
	          const forceFromStart = Boolean(options && options.forceFromStart);
	          const since = forceFromStart ? 0 : appLogOffset;
	          const payload = await api.get(`/app/logs?since=${since}`);
	          const data = payload || {};
	          const startedAt = data.startedAt || "";
	          const lost = Boolean(data.lost);
	          const chunk = data.text || "";

	          const startedAtChanged = startedAt && startedAt !== appLogStartedAt;

	          if (!forceFromStart && startedAtChanged && appLogOffset !== 0) {
	            appLogOffset = 0;
	            await loadAppLogs({ forceFromStart: true });
	            return;
	          }

	          const shouldReset = lost || startedAtChanged || appLogOffset === 0;
	          if (shouldReset) {
	            resetAppLogRender(pre);
	          }
	          appendAppLogChunk(pre, chunk);

	          appLogStartedAt = startedAt || appLogStartedAt;
	          if (typeof data.to === "number") {
	            appLogOffset = data.to;
	          }

	          if (metaEl) {
	            const parts = ["应用"];
	            if (data.running === false) parts.push("已停止");
	            if (data.pid) parts.push(`pid ${data.pid}`);
	            if (startedAt) parts.push(`启动于 ${formatTime(startedAt)}`);
	            if (data.error) parts.push(`错误：${data.error}`);
	            metaEl.textContent = parts.length ? parts.join(" · ") : "-";
	          }

	          if (appLogAutoScroll && logContainer) {
	            logContainer.scrollTop = logContainer.scrollHeight;
	          }
	        } catch (err) {
	          if (metaEl) {
	            metaEl.textContent = `加载失败：${err.message}`;
	          }
	        }
	      }

      function getSavedFRouterId() {
        try {
          return localStorage.getItem(FROUTER_STORAGE_KEY) || "";
        } catch {
          return "";
        }
      }

      function persistFRouterId(frouterId) {
        try {
          localStorage.setItem(FROUTER_STORAGE_KEY, frouterId || "");
        } catch (err) {
          console.warn("Failed to persist FRouter selection:", err);
        }
      }

      function getCurrentFRouter() {
        if (!Array.isArray(froutersCache) || froutersCache.length === 0) {
          return null;
        }
        if (currentFRouterId) {
          const found = froutersCache.find((frouter) => frouter.id === currentFRouterId);
          if (found) return found;
        }
        return froutersCache[0] || null;
      }

      function updateFRouterSelectors() {
        const current = getCurrentFRouter();
        const selectedId = current ? current.id : "";
        if (selectedId && selectedId !== currentFRouterId) {
          currentFRouterId = selectedId;
          persistFRouterId(selectedId);
        }
        const options = Array.isArray(froutersCache)
          ? froutersCache
              .map((frouter) => `<option value="${escapeHtml(frouter.id)}">${escapeHtml(frouter.name || frouter.id)}</option>`)
              .join("")
          : "";
        ["chain-route-select"].forEach((id) => {
          const select = document.getElementById(id);
          if (!select) return;
          select.innerHTML = options || '<option value="">暂无 FRouter</option>';
          if (selectedId) {
            select.value = selectedId;
          }
        });
      }

      function setCurrentFRouter(frouterId, { notify = false, reloadGraph = true } = {}) {
        if (!Array.isArray(froutersCache) || froutersCache.length === 0) {
          currentFRouterId = "";
          updateFRouterSelectors();
          updateHomeFRouterMetrics();
          return;
        }
        const next = froutersCache.find((frouter) => frouter.id === frouterId) || froutersCache[0];
        currentFRouterId = next ? next.id : "";
        persistFRouterId(currentFRouterId);
        updateFRouterSelectors();
        renderFRouters(froutersCache, currentFRouterId);
        updateHomeFRouterMetrics();
        if (chainEditorInitialized && chainListEditor) {
          chainListEditor.setFRouterId(currentFRouterId);
          if (reloadGraph) {
            chainListEditor.loadGraph();
          }
        }
        if (notify && next) {
          showStatus("已切换", "info", 2000);
        }
      }

      async function applyFRouterSelection(frouterId, { notify = false } = {}) {
        const nextId = String(frouterId || "").trim();
        if (!nextId) return;

        applyFRouterPendingId = nextId;
        if (applyFRouterInFlight) {
          return;
        }

        applyFRouterInFlight = true;
        try {
          while (applyFRouterPendingId) {
            const targetId = applyFRouterPendingId;
            applyFRouterPendingId = "";
            await applyFRouterSelectionOnce(targetId, { notify });
          }
        } finally {
          applyFRouterInFlight = false;
        }
      }

      async function applyFRouterSelectionOnce(frouterId, { notify = false } = {}) {
        let status = null;
        try {
          status = await api.proxy.status();
        } catch (err) {
          if (notify) {
            showStatus(`获取内核状态失败：${err.message}`, "error", 6000);
          }
          return;
        }

        coreStatus = status || {};
        updateCoreUI(coreStatus);

        const coreRunning = Boolean(status && status.running);
        if (!coreRunning) {
          if (notify) {
            showStatus("内核未运行：已切换 FRouter（下次启动生效）", "info", 2500);
          }
          await loadIPGeo();
          return;
        }

        const currentRunningId = status && typeof status.frouterId === "string" ? status.frouterId : "";
        if (currentRunningId && currentRunningId === frouterId) {
          if (notify) {
            showStatus("已是当前 FRouter", "info", 1500);
          }
          await loadIPGeo();
          return;
        }

        const wasSystemProxyEnabled = Boolean(systemProxySettings && systemProxySettings.enabled);
        const ignoreHosts = collectIgnoreHosts();

        if (notify) {
          showStatus("正在切换 FRouter…", "info", 2000);
        }

        // 系统代理开启时，重启内核前必须先关闭系统代理，避免“指向黑洞”断网。
        if (wasSystemProxyEnabled) {
          try {
            await api.put("/settings/system-proxy", {
              enabled: false,
              ignoreHosts,
            });
          } catch (err) {
            showStatus(`关闭系统代理失败：${err.message}`, "error", 6000);
            return;
          }
          await loadSystemProxySettings();
        }

        try {
          await api.post("/proxy/start", { frouterId });
        } catch (err) {
          showStatus(`切换失败：${err.message}`, "error", 6000);
          await refreshCoreStatus();
          await loadSystemProxySettings();
          await loadIPGeo();
          return;
        }

        await sleep(500);
        await refreshCoreStatus();

        if (wasSystemProxyEnabled) {
          try {
            await api.put("/settings/system-proxy", {
              enabled: true,
              ignoreHosts,
            });
          } catch (err) {
            showStatus(`系统代理未能恢复：${err.message}`, "warn", 6000);
          }
          await loadSystemProxySettings();
        }

        await loadIPGeo();
        if (notify) {
          showStatus("FRouter 已切换", "success", 2000);
        }
      }

      async function refreshCoreStatus({ notify = false } = {}) {
        try {
          const status = await api.proxy.status();
          coreStatus = status || {};
          updateCoreUI(coreStatus);

          if (notify && coreStatus.running) {
            showStatus("内核已启动", "success");
          }
        } catch (err) {
          console.error("Failed to refresh status:", err);
          if (notify) {
            showStatus(`加载状态失败：${err.message}`, "error");
          }
        }
      }

      function updateCoreUI(status = {}) {
        if (!status || typeof status !== "object") {
          status = {};
        }
        const indicator = document.getElementById("core-state");
        const proxyToggle = document.getElementById("proxy-toggle");

        const coreRunning = Boolean(status.running);
        const systemProxyEnabled = Boolean(systemProxySettings && systemProxySettings.enabled);

        // Update core status indicator
        if (indicator) {
          indicator.className = coreRunning ? "badge active" : "badge";
          const engineLabel = coreRunning && status.engine ? componentDisplayName(status.engine) : "";
          indicator.textContent = coreRunning
            ? `核心：运行中${engineLabel ? `（${engineLabel}）` : ""}`
            : "核心：已停止";
        }

        // Update proxy toggle button
        if (!proxyToggle) return;

        if (systemProxyEnabled) {
          proxyToggle.dataset.mode = "stop";
          proxyToggle.classList.add("active");
          proxyToggle.title = "关闭系统代理";
        } else {
          proxyToggle.dataset.mode = "start";
          proxyToggle.classList.remove("active");
          proxyToggle.title = coreRunning ? "启用系统代理（内核已在后台运行）" : "启用系统代理";
        }
      }

      // TUN status management
      let tunStatusCache = null;

      async function checkTUNStatus({ notify = false } = {}) {
        try {
          const status = await api.get("/tun/check");
          tunStatusCache = status || {};
          updateTUNUI(tunStatusCache);
          if (notify) {
            const text = status.configured
              ? `TUN 模式已配置 (${status.platform})`
              : `TUN 模式未配置 (${status.platform})`;
            showStatus(text, status.configured ? "success" : "info");
          }
        } catch (err) {
          tunStatusCache = { configured: false, error: err.message };
          updateTUNUI(tunStatusCache);
          if (notify) {
            showStatus(`检查 TUN 状态失败：${err.message}`, "error", 6000);
          }
        }
      }

      // TUN Settings state
      let tunSettingsCache = null;

      async function loadTUNSettings() {
        try {
          const cfg = await api.get("/proxy/config");
          tunSettingsCache = { enabled: cfg && cfg.inboundMode === "tun" };
          updateTUNToggle(tunSettingsCache);
        } catch (err) {
          console.error("Failed to load TUN settings:", err);
          tunSettingsCache = { enabled: false };
          updateTUNToggle(tunSettingsCache);
        }
      }

      // Engine Settings state
      let engineSettingsCache = null;

      function isCoreComponentInstalled(kind) {
        const list = Array.isArray(componentsCache) ? componentsCache : [];
        const target = list.find((item) => item && item.kind === kind);
        return Boolean(target && target.installDir);
      }

      function updateEngineSelectOptions() {
        const select = document.getElementById("engine-select");
        if (!select) return;

        Array.from(select.options).forEach((opt) => {
          const value = opt.value || "";
          const baseLabel = opt.dataset.label || opt.textContent || value || "-";
          if (!opt.dataset.label) {
            opt.dataset.label = baseLabel;
          }

          if (value === "auto") {
            opt.disabled = false;
            opt.textContent = baseLabel;
            return;
          }

          const installed = isCoreComponentInstalled(value);
          opt.disabled = !installed;
          opt.textContent = installed ? baseLabel : `${baseLabel}（未安装）`;
        });
      }

      async function loadEngineSettings() {
        const select = document.getElementById("engine-select");
        if (!select) return;
        try {
          const cfg = await api.get("/proxy/config");
          const preferred = cfg && cfg.preferredEngine ? cfg.preferredEngine : "auto";
          engineSettingsCache = { preferredEngine: preferred };
          select.value = preferred || "auto";
        } catch (err) {
          console.error("Failed to load engine settings:", err);
          engineSettingsCache = { preferredEngine: "auto" };
          select.value = "auto";
        } finally {
          updateEngineSelectOptions();
        }
      }

      async function updateEngineSetting(preferredEngine) {
        const select = document.getElementById("engine-select");
        if (!select) return;

        const normalized = ["auto", "singbox", "clash"].includes(preferredEngine)
          ? preferredEngine
          : "auto";

        select.disabled = true;

        let prevEngine = "auto";
        try {
          if (engineSettingsCache && engineSettingsCache.preferredEngine) {
            prevEngine = engineSettingsCache.preferredEngine;
          } else {
            const cfg = await api.get("/proxy/config");
            prevEngine = cfg && cfg.preferredEngine ? cfg.preferredEngine : "auto";
          }
        } catch {
          prevEngine = "auto";
        }

        if (normalized === prevEngine) {
          select.value = normalized;
          select.disabled = false;
          updateEngineSelectOptions();
          return;
        }

        const ignoreHosts = collectIgnoreHosts();
        let wasRunning = false;
        try {
          const status = await api.proxy.status();
          coreStatus = status || coreStatus;
          wasRunning = Boolean(status && status.running);
        } catch {
          wasRunning = false;
        }

        const wasSystemProxyEnabled = Boolean(systemProxySettings && systemProxySettings.enabled);
        let systemProxyDisabled = false;
        let configSaved = false;

        async function refreshEngineSwitchUI({ alwaysLoadSystemProxy = false, restoreSystemProxy = null } = {}) {
          await refreshCoreStatus();

          if (typeof restoreSystemProxy === "function") {
            await restoreSystemProxy();
          }

          if (alwaysLoadSystemProxy || (wasSystemProxyEnabled && systemProxyDisabled)) {
            await loadSystemProxySettings();
          }

          await loadIPGeo();

          if (currentPanel === "panel-logs") {
            kernelLogOffset = 0;
            kernelLogSession = 0;
            kernelLogStartedAt = "";
            const pre = document.getElementById("kernel-log-content");
            resetKernelLogRender(pre);
            await loadKernelLogs();
          }
        }

        try {
          const updated = await api.put("/proxy/config", { preferredEngine: normalized });
          configSaved = true;
          engineSettingsCache = { preferredEngine: updated && updated.preferredEngine ? updated.preferredEngine : normalized };
          select.value = engineSettingsCache.preferredEngine || normalized;

          const selectedFRouter = getCurrentFRouter();
          const frouterId = selectedFRouter && selectedFRouter.id
            ? selectedFRouter.id
            : (updated && updated.frouterId ? updated.frouterId : (coreStatus && coreStatus.frouterId ? coreStatus.frouterId : ""));
          if (!frouterId) {
            showStatus("内核偏好已保存，但未选择 FRouter，无法重启内核", "warn", 4000);
            return;
          }

          if (wasSystemProxyEnabled) {
            try {
              await api.put("/settings/system-proxy", { enabled: false, ignoreHosts });
              systemProxyDisabled = true;
              await loadSystemProxySettings();
            } catch (err) {
              showStatus(`关闭系统代理失败：${err.message}`, "error", 6000);
              return;
            }
          }

          showStatus(wasRunning ? "正在重启内核..." : "正在启动内核...", "info", 2000);
          await api.post("/proxy/start", { frouterId });
          await sleep(500);

          await refreshEngineSwitchUI({
            restoreSystemProxy: async () => {
              if (!wasSystemProxyEnabled || !systemProxyDisabled) return;
              try {
                await api.put("/settings/system-proxy", { enabled: true, ignoreHosts });
              } catch (err) {
                showStatus(`系统代理未能恢复：${err.message}`, "warn", 6000);
              }
            },
          });

          const actualEngine = coreStatus && coreStatus.engine ? String(coreStatus.engine) : "";
          const action = wasRunning ? "重启" : "启动";
          if (normalized === "auto") {
            const label = actualEngine ? componentDisplayName(actualEngine) : "";
            showStatus(label ? `内核已${action}（自动：${label}）` : `内核已${action}`, "success", 2000);
          } else if (actualEngine && actualEngine !== normalized) {
            showStatus(`内核已${action}，但实际运行：${componentDisplayName(actualEngine)}`, "warn", 5000);
          } else {
            const label = actualEngine ? componentDisplayName(actualEngine) : componentDisplayName(normalized);
            showStatus(label ? `内核已${action}（${label}）` : `内核已${action}`, "success", 2000);
          }
        } catch (err) {
          showStatus(`切换内核失败：${err.message}`, "error", 6000);
          if (configSaved && wasRunning) {
            select.value = prevEngine || "auto";
            engineSettingsCache = { preferredEngine: prevEngine || "auto" };

            try {
              await api.put("/proxy/config", { preferredEngine: prevEngine || "auto" });
            } catch {
              // ignore
            }

            const selectedFRouter = getCurrentFRouter();
            const frouterId = selectedFRouter && selectedFRouter.id
              ? selectedFRouter.id
              : (coreStatus && coreStatus.frouterId ? coreStatus.frouterId : "");
            if (frouterId) {
              try {
                await api.post("/proxy/start", { frouterId });
                await sleep(500);
              } catch {
                // ignore
              }
            }
          } else if (!configSaved) {
            select.value = prevEngine || "auto";
            engineSettingsCache = { preferredEngine: prevEngine || "auto" };
          } else {
            select.value = normalized;
            engineSettingsCache = { preferredEngine: normalized };
          }

          await refreshEngineSwitchUI({
            alwaysLoadSystemProxy: true,
            restoreSystemProxy: async () => {
              if (!wasSystemProxyEnabled || !systemProxyDisabled) return;
              if (!(coreStatus && coreStatus.running)) return;
              try {
                await api.put("/settings/system-proxy", { enabled: true, ignoreHosts });
              } catch {
                // ignore
              }
            },
          });
        } finally {
          select.disabled = false;
          updateEngineSelectOptions();
        }
      }

      // IP Geo info
      async function loadIPGeo() {
        const ipEl = document.getElementById("ip-geo-ip");
        const locEl = document.getElementById("ip-geo-location");
        if (!ipEl || !locEl) return;

        ipEl.textContent = "...";
        locEl.textContent = "正在获取";

        try {
          const data = await api.get("/ip/geo");
          if (data.error) {
            ipEl.textContent = "--";
            locEl.textContent = data.error;
            return;
          }
          ipEl.textContent = data.ip || "--";
          const loc = [data.location, data.isp].filter(Boolean).join(" | ");
          locEl.textContent = loc || "--";
        } catch (err) {
          ipEl.textContent = "--";
          locEl.textContent = "获取失败";
        }
      }

      // IP Geo refresh button
      const ipGeoRefreshBtn = document.getElementById("ip-geo-refresh");
      if (ipGeoRefreshBtn) {
        ipGeoRefreshBtn.addEventListener("click", () => {
          loadIPGeo();
        });
      }

      async function updateTUNSetting(enabled) {
        const prevEnabled = Boolean(tunSettingsCache && tunSettingsCache.enabled);

        try {
          const updated = await api.put("/proxy/config", { inboundMode: enabled ? "tun" : "mixed" });
          tunSettingsCache = { enabled: updated && updated.inboundMode === "tun" };
          updateTUNToggle(tunSettingsCache || { enabled: false });

          // 启用 TUN 时系统代理应该关闭（否则系统会指向一个不存在的本地端口）。
          if (enabled) {
            try {
              const response = await api.put("/settings/system-proxy", {
                enabled: false,
                ignoreHosts: collectIgnoreHosts(),
              });
              const data = response && response.settings ? response.settings : response;
              systemProxySettings = {
                enabled: Boolean(data.enabled),
                ignoreHosts: Array.isArray(data.ignoreHosts) ? data.ignoreHosts : systemProxySettings.ignoreHosts,
              };
              renderSystemProxy(systemProxySettings);
              updateCoreUI(coreStatus);
            } catch (err) {
              console.warn("Failed to disable system proxy when enabling TUN:", err);
            }
          }

          // 变更入站模式需要重启/启动内核，否则不会创建/释放 TUN 网卡。
          let status = null;
          try {
            status = await api.proxy.status();
            coreStatus = status || coreStatus;
          } catch {
            status = null;
          }

          const shouldStart = enabled || (status && status.running);
          if (shouldStart) {
            const selectedFRouter = getCurrentFRouter();
            const frouterId = selectedFRouter && selectedFRouter.id
              ? selectedFRouter.id
              : (updated && updated.frouterId ? updated.frouterId : "");
            if (!frouterId) {
              throw new Error("请先选择一个 FRouter");
            }
            showStatus(enabled ? "正在启动 TUN 内核..." : "正在重启内核...", "info", 2000);
            await api.post("/proxy/start", { frouterId });
          }

          await refreshCoreStatus();
          await checkTUNStatus();
          await loadSystemProxySettings();
          await loadIPGeo();
          showStatus(`TUN 模式已${enabled ? '启用' : '禁用'}`, "success", 2000);
        } catch (err) {
          showStatus(`更新 TUN 设置失败：${err.message}`, "error", 6000);

          // 恢复开关状态（并回滚配置，避免 UI/后端不一致）
          tunSettingsCache = { enabled: prevEnabled };
          updateTUNToggle(tunSettingsCache);
          try {
            await api.put("/proxy/config", { inboundMode: prevEnabled ? "tun" : "mixed" });
          } catch {
            // ignore
          }
        }
      }

      function updateTUNToggle(settings) {
        const tunToggle = document.getElementById("tun-toggle");
        if (tunToggle) {
          tunToggle.checked = Boolean(settings.enabled);
          // 不再禁用开关，让用户点击时自动检测
        }
      }

      function updateTUNUI(status = {}) {
        const tunCard = document.getElementById("tun-status-card");
        const tunValue = document.getElementById("tun-status-value");

        if (!tunCard || !tunValue) return;

        const configured = Boolean(status.configured);
        const platform = status.platform || "unknown";

        if (status.error) {
          tunValue.textContent = "检查失败";
          tunValue.style.color = "var(--error)";
          tunCard.title = `错误：${status.error}`;
        } else if (configured) {
          tunValue.textContent = "已配置";
          tunValue.style.color = "var(--success)";
          tunCard.title = `TUN 模式已在 ${platform} 上配置`;
        } else {
          tunValue.textContent = "未配置";
          tunValue.style.color = "var(--warning)";
          const setupCmd = status.setupCommand || "查看文档";
          tunCard.title = `点击查看配置方法：${setupCmd}`;
        }

        // 更新设置面板中的 TUN 状态
        const tunConfigStatus = document.getElementById("tun-config-status");
        const tunSetupBtn = document.getElementById("tun-setup-btn");

        if (tunConfigStatus) {
          if (status.error) {
            tunConfigStatus.textContent = `检查失败: ${status.error}`;
            tunConfigStatus.style.color = "var(--error)";
          } else if (configured) {
            tunConfigStatus.textContent = `已配置 (${platform})`;
            tunConfigStatus.style.color = "var(--success)";
          } else {
            tunConfigStatus.textContent = `未配置 (${platform})`;
            tunConfigStatus.style.color = "var(--warning)";
          }
        }

        if (tunSetupBtn) {
          tunSetupBtn.style.display = (!configured && platform === "linux") ? "block" : "none";
        }
      }

      async function showTUNStatusDialog() {
        showStatus("正在检查 TUN 状态...", "info");
        await checkTUNStatus({ notify: true });
      }

      // 配置 TUN - 直接调用 API
      async function setupTUN() {
        try {
          showStatus("正在配置 TUN 模式...", "info");
          await api.post("/tun/setup", {});
          showStatus("TUN 配置成功！", "success", 2000);
          await checkTUNStatus();
          // 配置成功后自动启用 TUN
          await updateTUNSetting(true);
        } catch (err) {
          showStatus(`TUN 配置失败：${err.message}`, "error", 5000);
        }
      }

      // Panel loaders

		      async function loadFRouters({ notify = false } = {}) {
		        try {
		          const payload = await api.get("/frouters");
	          const frouters = Array.isArray(payload.frouters) ? payload.frouters : [];
	          froutersCache = frouters;

	          if (!currentFRouterId) {
	            currentFRouterId = getSavedFRouterId();
	          }
	          if (currentFRouterId && !froutersCache.find((frouter) => frouter.id === currentFRouterId)) {
	            currentFRouterId = "";
	          }

          const current = getCurrentFRouter();
          if (current && current.id) {
            currentFRouterId = current.id;
            persistFRouterId(currentFRouterId);
          }

	          updateFRouterTabs(frouters);
	          renderFRouters(frouters, currentFRouterId);
	          updateFRouterSelectors();
	          updateHomeFRouterMetrics();
	          refreshCoreStatus();
	          if (notify) {
	            showStatus("FRouter 列表已刷新", "success");
	          }
	        } catch (err) {
	          showStatus(`加载 FRouter 失败：${err.message}`, "error", 6000);
	          froutersCache = [];
	          currentFRouterId = "";
	          updateFRouterSelectors();
	          updateHomeFRouterMetrics();
		        }
		      }

          async function loadNodes() {
            if (nodesLoadInFlight) {
              return;
            }
            nodesLoadInFlight = true;
            try {
              const payload = await api.get("/nodes");
              nodesCache = Array.isArray(payload.nodes) ? payload.nodes : [];
              if (currentPanel === "panel-nodes") {
                renderOrUpdateNodesPanel(nodesCache);
              }
            } catch (err) {
              console.error("加载节点失败:", err);
              if (currentPanel === "panel-nodes") {
                showStatus(`加载节点失败：${err.message}`, "error", 6000);
              }
            } finally {
              nodesLoadInFlight = false;
            }
          }
		
		      async function autoTestHomePanel({ notify = false } = {}) {
		        const frouter = getCurrentFRouter();
		        const id = frouter && frouter.id ? frouter.id : "";
		        if (!id) return;

	        if (homeAutoTestInFlight) {
	          return;
	        }
	        homeAutoTestInFlight = true;

	        let queued = false;
	        try {
	          await api.post(`/frouters/${id}/ping`);
	          queued = true;
	        } catch (err) {
	          console.error("主页自动测延迟失败:", err);
	          if (notify) {
	            showStatus(`自动测延迟失败：${err.message}`, "error", 4000);
	          }
	        }
	        try {
	          await api.post(`/frouters/${id}/speedtest`);
	          queued = true;
	        } catch (err) {
	          console.error("主页自动测速失败:", err);
	          if (notify) {
	            showStatus(`自动测速失败：${err.message}`, "error", 4000);
	          }
	        }
	        if (notify && queued) {
	          showStatus("已自动开始测速/测延迟", "info", 2000);
	        }
	        homeAutoTestInFlight = false;
	      }

      function updateFRouterTabs(frouters) {
        if (!nodeTabs) return;
        const tags = new Set();
        frouters.forEach((frouter) => {
          if (Array.isArray(frouter.tags)) {
            frouter.tags.forEach((tag) => {
              const trimmed = String(tag || "").trim();
              if (trimmed) {
                tags.add(trimmed);
              }
            });
          }
        });
        const sorted = ["全部", ...Array.from(tags).sort((a, b) => a.localeCompare(b, "zh-Hans-CN"))];
        frouterTags = sorted;
        if (!frouterTags.includes(currentFRouterTab)) {
          currentFRouterTab = "全部";
        }
        nodeTabs.innerHTML = frouterTags
          .map((tag) => {
            const active = tag === currentFRouterTab ? "active" : "";
            return `<div class=\"node-tab ${active}\" data-tag=\"${escapeHtml(tag)}\">${escapeHtml(tag)}</div>`;
          })
          .join("");
	      }

	      function getConfigNameById(configId) {
	        if (!configId) return "";
	        const list = Array.isArray(configsCache) ? configsCache : [];
	        const cfg = list.find((item) => item && item.id === configId);
	        return cfg?.name || "";
	      }

	      function formatSubscriptionLabel(configId) {
	        if (!configId) return "未归属订阅";
	        const name = getConfigNameById(configId);
	        return name || String(configId);
	      }

	      function groupNodesBySubscription(nodes) {
	        const all = Array.isArray(nodes) ? nodes : [];
	        const groupsMap = new Map();
	        for (const n of all) {
	          const key = n?.sourceConfigId ? String(n.sourceConfigId) : "";
	          if (!groupsMap.has(key)) {
	            groupsMap.set(key, { key, label: formatSubscriptionLabel(key), nodes: [] });
	          }
	          groupsMap.get(key).nodes.push(n);
	        }

	        for (const group of groupsMap.values()) {
	          group.nodes.sort((a, b) => {
	            const an = (a?.name || a?.id || "").toString();
	            const bn = (b?.name || b?.id || "").toString();
	            return an.localeCompare(bn, "zh-Hans-CN");
	          });
	        }

	        const order = new Map();
	        if (Array.isArray(configsCache)) {
	          configsCache.forEach((cfg, idx) => {
	            if (cfg?.id) order.set(String(cfg.id), idx);
	          });
	        }

	        return Array.from(groupsMap.values()).sort((a, b) => {
	          const aEmpty = !a.key;
	          const bEmpty = !b.key;
	          if (aEmpty && bEmpty) return 0;
	          if (aEmpty) return 1;
	          if (bEmpty) return -1;
	          const ao = order.has(a.key) ? order.get(a.key) : Number.MAX_SAFE_INTEGER;
	          const bo = order.has(b.key) ? order.get(b.key) : Number.MAX_SAFE_INTEGER;
	          if (ao !== bo) return ao - bo;
	          return a.label.localeCompare(b.label, "zh-Hans-CN");
	        });
	      }

	      let lastRenderedFRoutersHash = "";
	      let lastRenderedNodesStructureHash = "";

	      function nodesStructureHash(nodes) {
	        const all = Array.isArray(nodes) ? nodes : [];
	        if (all.length === 0) return "";
	        return all
	          .map((n) => {
	            const id = (n && n.id) || "";
	            const name = (n && n.name) || "";
	            const proto = (n && n.protocol) || "";
	            const addr = (n && n.address) || "";
	            const port = (n && n.port) || "";
	            const source = (n && n.sourceConfigId) || "";
	            const sourceName = source ? getConfigNameById(source) : "";
	            return `${id}|${name}|${proto}|${addr}|${port}|${source}|${sourceName}`;
	          })
	          .join(";");
	      }

	      function getNodeLatencyDisplay(n) {
	        let value = "--";
	        let cls = "";
	        let title = "";
	        if (n && n.lastLatencyError) {
	          value = "错误";
	          cls = "poor";
	          title = n.lastLatencyError;
	        } else if (n && typeof n.lastLatencyMs === "number" && n.lastLatencyMs > 0) {
	          value = `${n.lastLatencyMs}ms`;
	          if (n.lastLatencyMs < 100) cls = "good";
	          else if (n.lastLatencyMs < 300) cls = "fair";
	          else cls = "poor";
	        }
	        return { value, cls, title };
	      }

	      function getNodeSpeedDisplay(n) {
	        let value = "--";
	        let cls = "";
	        let title = "";
	        if (n && n.lastSpeedError) {
	          value = "错误";
	          cls = "poor";
	          title = n.lastSpeedError;
	        } else if (n && typeof n.lastSpeedMbps === "number" && n.lastSpeedMbps > 0) {
	          const fixed = n.lastSpeedMbps >= 10 ? n.lastSpeedMbps.toFixed(1) : n.lastSpeedMbps.toFixed(2);
	          value = `${fixed} Mbps`;
	          if (n.lastSpeedMbps > 5) cls = "good";
	          else if (n.lastSpeedMbps > 1) cls = "fair";
	          else cls = "poor";
	        }
	        return { value, cls, title };
	      }

	      function renderOrUpdateNodesPanel(nodes) {
	        const all = Array.isArray(nodes) ? nodes : [];
	        if (all.length === 0) {
	          lastRenderedNodesStructureHash = "";
	          renderNodesPanel(all);
	          return;
	        }

	        const nextHash = nodesStructureHash(all);
	        if (nextHash !== lastRenderedNodesStructureHash) {
	          lastRenderedNodesStructureHash = nextHash;
	          renderNodesPanel(all);
	          return;
	        }

	        updateNodesMetrics(all);
	      }

	      function updateNodesMetrics(nodes) {
	        const tbody = document.querySelector("#nodes-table tbody");
	        if (!tbody) return;

	        const rowMap = new Map();
	        tbody.querySelectorAll('tr[data-id]').forEach((row) => {
	          const id = row.dataset.id || "";
	          if (id) rowMap.set(id, row);
	        });
	        if (rowMap.size !== nodes.length) {
	          renderNodesPanel(nodes);
	          return;
	        }

	        for (const n of nodes) {
	          const id = (n && n.id) || "";
	          if (!id) continue;
	          const row = rowMap.get(id);
	          if (!row) {
	            renderNodesPanel(nodes);
	            return;
	          }

	          const latencyEl = row.querySelector('[data-metric="latency"]');
	          const speedEl = row.querySelector('[data-metric="speed"]');
	          if (!latencyEl || !speedEl) {
	            renderNodesPanel(nodes);
	            return;
	          }

	          const latency = getNodeLatencyDisplay(n);
	          latencyEl.textContent = latency.value;
	          latencyEl.className = `node-metric-value${latency.cls ? " " + latency.cls : ""}`;
	          latencyEl.title = latency.title || "";

	          const speed = getNodeSpeedDisplay(n);
	          speedEl.textContent = speed.value;
	          speedEl.className = `node-metric-value${speed.cls ? " " + speed.cls : ""}`;
	          speedEl.title = speed.title || "";
	        }
	      }
	      function renderNodesPanel(nodes) {
	        const tbody = document.querySelector("#nodes-table tbody");
	        if (!tbody) return;

	        const all = Array.isArray(nodes) ? nodes : [];
	        if (all.length === 0) {
	          tbody.innerHTML = `
	            <tr>
	              <td colspan="6" style="text-align:center; color:var(--text-secondary); padding:24px;">暂无节点</td>
	            </tr>
	          `;
	          return;
	        }

	        const groups = groupNodesBySubscription(all);
	        tbody.innerHTML = groups
	          .map((group) => {
	            const pillText = group.key ? "订阅" : "未归属";
	            const header = `
	              <tr class="node-group-row" data-group="${escapeHtml(group.key || "unbound")}">
	                <td colspan="6">
	                  <div class="node-group-label">
	                    <span class="node-group-pill">${escapeHtml(pillText)}</span>
	                    <span class="node-group-name">${escapeHtml(group.label)}</span>
	                    <span class="node-group-count">${group.nodes.length} 个</span>
	                  </div>
	                </td>
	              </tr>
	            `;

		            const rows = group.nodes
		              .map((n) => {
		                const id = n.id || "";
		                const name = n.name || n.id || "未命名节点";
		                const proto = (n.protocol || "").toUpperCase() || "UNKNOWN";
		                const addr = n.address ? `${n.address}:${n.port || ""}` : "-";

		                const latency = getNodeLatencyDisplay(n);
		                const speed = getNodeSpeedDisplay(n);

		                const editButton = `<button class="mini-btn" data-action="edit-node">编辑</button>`;
		                return `
		                  <tr class="node-card-row" data-id="${escapeHtml(id)}">
		                    <td colspan="6">
		                      <div class="node-card">
		                        <div class="node-card-top">
		                          <div class="node-card-title">
		                            <span class="node-card-name" title="${escapeHtml(name)}">${escapeHtml(name)}</span>
		                            <span class="node-card-protocol">${escapeHtml(proto)}</span>
		                          </div>
		                          <div class="node-card-actions">
		                            <button class="mini-btn" data-action="ping-node">测延迟</button>
		                            <button class="mini-btn" data-action="speedtest-node">测速</button>
		                            ${editButton}
		                          </div>
		                        </div>
		                        <div class="node-card-meta">
		                          <span class="node-card-addr" title="${escapeHtml(addr)}">${escapeHtml(addr)}</span>
		                          <span class="node-card-id" title="${escapeHtml(id)}">${escapeHtml(id)}</span>
		                        </div>
		                        <div class="node-card-stats">
		                          <div class="node-card-stat">
		                            <span>延迟</span>
		                            <span class="node-metric-value ${latency.cls}" data-metric="latency" title="${escapeHtml(latency.title)}">${escapeHtml(latency.value)}</span>
		                          </div>
		                          <div class="node-card-stat">
		                            <span>速度</span>
		                            <span class="node-metric-value ${speed.cls}" data-metric="speed" title="${escapeHtml(speed.title)}">${escapeHtml(speed.value)}</span>
		                          </div>
		                        </div>
		                      </div>
		                    </td>
		                  </tr>
		                `;
		              })
		              .join("");

	            return `${header}${rows}`;
	          })
	          .join("");
	      }

	      function renderFRouters(frouters, currentId = "") {
	        if (!nodeGrid) return;
	        let filtered = frouters;
        if (currentFRouterTab !== "全部") {
          filtered = frouters.filter((frouter) => Array.isArray(frouter.tags) && frouter.tags.includes(currentFRouterTab));
        }
        if (!Array.isArray(frouters) || frouters.length === 0) {
          nodeGrid.innerHTML = '<div class="empty-card">暂无 FRouter</div>';
          lastRenderedFRoutersHash = "empty";
          return;
        }

        if (!Array.isArray(filtered) || filtered.length === 0) {
          nodeGrid.innerHTML = '<div class="empty-card">当前标签下暂无 FRouter</div>';
          lastRenderedFRoutersHash = "empty-filtered";
          return;
        }

        const stateHash = JSON.stringify(filtered.map(r => ({
          id: r.id,
          name: r.name,
          tags: r.tags,
          edges: r.chainProxy && Array.isArray(r.chainProxy.edges) ? r.chainProxy.edges.length : 0,
          slots: r.chainProxy && Array.isArray(r.chainProxy.slots) ? r.chainProxy.slots.length : 0,
          lastLatencyMs: r.lastLatencyMs,
          lastSpeedMbps: r.lastSpeedMbps,
          lastSpeedError: r.lastSpeedError
        }))) + currentId + currentFRouterTab;

        if (stateHash === lastRenderedFRoutersHash) {
          return;
        }
        lastRenderedFRoutersHash = stateHash;

        nodeGrid.innerHTML = filtered
          .map((frouter) => {
            const rowId = escapeHtml(frouter.id);
            const edgeCount = frouter.chainProxy && Array.isArray(frouter.chainProxy.edges) ? frouter.chainProxy.edges.length : 0;
            const slotCount = frouter.chainProxy && Array.isArray(frouter.chainProxy.slots) ? frouter.chainProxy.slots.length : 0;
            const tagText = Array.isArray(frouter.tags) && frouter.tags.length
              ? frouter.tags.join(" · ")
              : "无标签";

	            let latencyValue = "延迟";
	            let latencyClass = "";
	            const latencyTitle = frouter.lastLatencyError
	              ? `延迟失败：${frouter.lastLatencyError}`
	              : "测延迟";
	            if (typeof frouter.lastLatencyMs === "number" && frouter.lastLatencyMs > 0) {
	              latencyValue = `${frouter.lastLatencyMs}ms`;
	              if (frouter.lastLatencyMs < 100) latencyClass = "good";
	              else if (frouter.lastLatencyMs < 300) latencyClass = "fair";
	              else latencyClass = "poor";
	            }

	            let speedValue = "速度";
	            let speedClass = "";
	            const speedTitle = frouter.lastSpeedError
	              ? `测速失败：${frouter.lastSpeedError}`
	              : "测速";
	            if (frouter.lastSpeedError) {
	              speedValue = "错误";
	              speedClass = "poor";
	            } else if (typeof frouter.lastSpeedMbps === "number" && frouter.lastSpeedMbps > 0) {
              const fixed = frouter.lastSpeedMbps >= 10 ? frouter.lastSpeedMbps.toFixed(1) : frouter.lastSpeedMbps.toFixed(2);
              speedValue = `${fixed} Mbps`;
              if (frouter.lastSpeedMbps > 5) speedClass = "good";
              else if (frouter.lastSpeedMbps > 1) speedClass = "fair";
              else speedClass = "poor";
            }

            const selectedClass = frouter.id === currentId ? "active" : "";

            return `
          <div class="node-row ${selectedClass}" data-id="${rowId}">
            <div class="node-row-main">
              <div class="node-row-title">
		                <div class="node-row-name">${escapeHtml(frouter.name || "未命名 FRouter")}</div>
                <div class="node-row-meta">边: ${edgeCount} · 槽: ${slotCount} · ${escapeHtml(tagText)}</div>
              </div>
              <div class="node-row-protocol">路由</div>
            </div>

	            <div class="node-row-metrics">
	              <div class="node-metric" data-action="ping-route" title="${escapeHtml(latencyTitle)}">
	                <svg viewBox="0 0 24 24" width="14" height="14" stroke="currentColor" stroke-width="2" fill="none" stroke-linecap="round" stroke-linejoin="round"><path d="M22 12h-4l-3 9L9 3l-3 9H2"></path></svg>
	                <span class="node-metric-value ${latencyClass}">${latencyValue}</span>
	              </div>
	              <div class="node-metric" data-action="speed-route" title="${escapeHtml(speedTitle)}">
	                <svg viewBox="0 0 24 24" width="14" height="14" stroke="currentColor" stroke-width="2" fill="none" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"></polyline></svg>
	                <span class="node-metric-value ${speedClass}">${speedValue}</span>
	              </div>
	            </div>
          </div>
        `;
          })
          .join("");
      }

      async function loadConfigs({ notify = false } = {}) {
        try {
          const configs = await api.get("/configs");
          configsCache = Array.isArray(configs) ? configs : [];
          renderConfigs(configsCache);
          renderOrUpdateNodesPanel(nodesCache);
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
            const syncErr = cfg.lastSyncError || "";
            const syncState = syncErr
              ? `<span class="badge error text-truncate" title="${escapeHtml(syncErr)}">失败：${escapeHtml(syncErr)}</span>`
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
	              <button class="ghost" data-action="edit-config">编辑</button>
	              <button class="ghost" data-action="pull-nodes">拉取节点</button>
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
        try {
          const payload = await api.get("/settings/system-proxy");
          const data = payload && payload.settings ? payload.settings : payload;
          systemProxySettings = {
            enabled: Boolean(data.enabled),
            ignoreHosts: Array.isArray(data.ignoreHosts) ? data.ignoreHosts : [...SYSTEM_PROXY_DEFAULTS],
          };
          renderSystemProxy(systemProxySettings);
          updateCoreUI(coreStatus);
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

      function summarizeHomeFRouterMetrics() {
        const route = getCurrentFRouter();
        if (!route) {
          return { latencyMs: null, speedMbps: null, speedError: "" };
        }
        const latencyMs = typeof route.lastLatencyMs === "number" && route.lastLatencyMs > 0
          ? route.lastLatencyMs
          : null;
        const speedError = route.lastSpeedError || "";
        const speedMbps = !speedError && typeof route.lastSpeedMbps === "number" && route.lastSpeedMbps > 0
          ? route.lastSpeedMbps
          : null;
        return { latencyMs, speedMbps, speedError };
      }

      function updateHomeFRouterMetrics() {
        const latencyEl = document.getElementById("home-node-latency");
        const speedEl = document.getElementById("home-node-speed");
        if (!latencyEl || !speedEl) return;

        const { latencyMs, speedMbps, speedError } = summarizeHomeFRouterMetrics();

        latencyEl.textContent = typeof latencyMs === "number" ? `${latencyMs}ms` : "--";

        if (speedError) {
          speedEl.textContent = "错误";
          speedEl.title = speedError;
          return;
        }

        speedEl.title = "";
        if (typeof speedMbps === "number") {
          const fixed = speedMbps >= 10 ? speedMbps.toFixed(1) : speedMbps.toFixed(2);
          speedEl.textContent = `${fixed} Mbps`;
        } else {
          speedEl.textContent = "--";
        }
      }

	      async function loadHomePanel({ notify = false } = {}) {
	        await Promise.all([
	          loadFRouters(),
          loadNodes(),
          loadComponents(),
          loadIPGeo(),
          refreshCoreStatus({ notify }),
          loadSystemProxySettings({ notify }),
          checkTUNStatus({ notify }),
          loadTUNSettings(),
          loadEngineSettings()
	        ]);
	        updateHomeFRouterMetrics();
          updateEngineSelectOptions();
	        autoTestHomePanel({ notify: true });
	      }

      async function handleProxyToggle() {
        const button = document.getElementById("proxy-toggle");
        if (!button || button.disabled) return;
        const mode = button.dataset.mode || "start";
        button.disabled = true;
        try {
          if (mode === "stop") {
            const response = await api.put("/settings/system-proxy", {
              enabled: false,
              ignoreHosts: collectIgnoreHosts(),
            });
            const data = response && response.settings ? response.settings : response;
            systemProxySettings = {
              enabled: Boolean(data.enabled),
              ignoreHosts: Array.isArray(data.ignoreHosts) ? data.ignoreHosts : systemProxySettings.ignoreHosts,
            };
            if (response && response.message) {
              showStatus(`系统代理已关闭，但提示: ${response.message}`, "warn", 4000);
            } else {
              showStatus("系统代理已关闭", "info");
            }
        } else {
            let status = null;
            try {
              status = await api.proxy.status();
              coreStatus = status || coreStatus;
            } catch {
              status = null;
            }

            if (!status || !status.running) {
              const selectedFRouter = getCurrentFRouter();
              const frouterId = selectedFRouter && selectedFRouter.id ? selectedFRouter.id : "";
              if (!frouterId) {
                throw new Error("请先选择一个 FRouter");
              }
              await api.post("/proxy/start", { frouterId });
              // 等待核心完全启动后再启用系统代理
              await new Promise((r) => setTimeout(r, 500));
            }
            const response = await api.put("/settings/system-proxy", {
              enabled: true,
              ignoreHosts: collectIgnoreHosts(),
            });
            if (response && response.message) {
              showStatus(`系统代理已启用，但提示: ${response.message}`, "info", 5000);
            } else {
              showStatus("系统代理已启用", "success");
            }
          }
          // Immediately refresh status after toggle
          await refreshCoreStatus();
          await loadSystemProxySettings();
          await loadIPGeo();
        } catch (err) {
          showStatus(`操作失败: ${err.message}`, "error");
        } finally {
          button.disabled = false;
        }
      }

      // 组件安装轮询定时器
      let componentPollTimer = null;

      function normalizeComponents(components) {
        const list = Array.isArray(components) ? components : [];

        const singbox = list.find((item) => item && item.kind === "singbox") || {
          id: "",
          name: "sing-box",
          kind: "singbox",
          lastInstalledAt: "",
          installDir: "",
          lastVersion: "",
          checksum: "",
          lastSyncError: "",
          meta: { repo: "SagerNet/sing-box" },
        };

        const clash = list.find((item) => item && item.kind === "clash") || {
          id: "",
          name: "clash",
          kind: "clash",
          lastInstalledAt: "",
          installDir: "",
          lastVersion: "",
          checksum: "",
          lastSyncError: "",
          meta: { repo: "MetaCubeX/mihomo" },
        };

        const rest = list.filter((item) => {
          if (!item || typeof item !== "object") return false;
          if (item.kind === "singbox") return item !== singbox;
          if (item.kind === "clash") return item !== clash;
          return true;
        });

        return [singbox, clash, ...rest];
      }

      function renderComponents(components) {
        const tbody = document.querySelector("#component-table tbody");
        if (!tbody) return;
        const normalized = normalizeComponents(components);

        // 检查是否有组件正在安装
        const installing = normalized.some(c => c.installStatus === 'downloading' || c.installStatus === 'extracting');
        if (installing && !componentPollTimer) {
          componentPollTimer = setInterval(() => loadComponents(), 1000);
        } else if (!installing && componentPollTimer) {
          clearInterval(componentPollTimer);
          componentPollTimer = null;
        }

        tbody.innerHTML = normalized
          .map((component) => {
            const id = escapeHtml(component.id || "");
            const name = escapeHtml(component.name || "-");
            const kind = escapeHtml(componentDisplayName(component.kind));
            const version = escapeHtml(component.lastVersion || "-");
            const installDir = escapeHtml(component.installDir || "-");
            const installedAt = formatTime(component.lastInstalledAt);
            let statusText;
            let actionBtn = '';

            // 检查安装状态
            const installStatus = component.installStatus || '';
            const installProgress = component.installProgress || 0;
            const installMessage = component.installMessage || '';

            if (installStatus === 'downloading' || installStatus === 'extracting') {
              // 正在安装中，显示进度条
              statusText = `
            <div class="install-progress">
              <div class="progress-bar">
                <div class="progress-fill" style="width: ${installProgress}%"></div>
              </div>
              <span class="progress-text">${escapeHtml(installMessage)} (${installProgress}%)</span>
            </div>
          `;
              actionBtn = `<button class="ghost" disabled>安装中...</button>`;
            } else if (installStatus === 'error' || component.lastSyncError) {
              const errMsg = installMessage || component.lastSyncError || '未知错误';
              statusText = `<span class="badge error">失败：${escapeHtml(errMsg)}</span>`;
              actionBtn = `<button class="primary" data-action="update-component">重试</button>`;
            } else if (component.installDir) {
              statusText = '<span class="badge">已安装</span>';
              actionBtn = `<button class="ghost" data-action="update-component">更新</button>`;
            } else {
              statusText = '<span class="badge warn">未安装</span>';
              actionBtn = `<button class="primary" data-action="update-component">安装</button>`;
            }

            const isCoreComponent = component.kind === "singbox" || component.kind === "clash";
            const uninstallBtn = (isCoreComponent && component.installDir && installStatus !== 'downloading' && installStatus !== 'extracting')
              ? '<button class="danger" data-action="uninstall-component">卸载</button>'
              : '';
            const deleteBtn = isCoreComponent ? '' : '<button class="danger" data-action="delete-component">删除</button>';

            return `
          <tr data-id="${id}" data-kind="${escapeHtml(component.kind)}">
            <td>${name}</td>
            <td><span class="badge">${kind}</span></td>
            <td>${version}</td>
            <td><span class="muted">${installDir}</span></td>
            <td>${installedAt}</td>
            <td>${statusText}</td>
            <td>
              ${actionBtn}
              ${uninstallBtn}
              ${deleteBtn}
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
          renderComponents(componentsCache);
          updateEngineSelectOptions();
          await refreshCoreStatus();
          if (notify) {
            showStatus("组件列表已刷新", "success");
          }
        } catch (err) {
          showStatus(`加载组件失败：${err.message}`, "error", 6000);
          componentsCache = [];
          renderComponents([]);
          updateEngineSelectOptions();
          await refreshCoreStatus();
        }
      }

      function componentDisplayName(kind) {
        switch (kind) {
          case "singbox":
            return "sing-box";
          case "clash":
            return "Clash";
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
          await refreshCoreStatus();
        } catch (err) {
          showStatus(`${pretty} 安装失败：${err.message}`, "error", 6000);
        }
      }

      // ===== Chain List Editor (路由规则列表) =====
      let chainListEditor = null;
      let chainEditorInitialized = false;

      class ChainListEditor {
        constructor(container, apiClient) {
          this.container = container;
          this.api = apiClient;
          this.frouterId = '';
          this.edges = [];
          this.detourEdges = [];
          this.slots = [];
          this.nodes = [];
          this.dirty = false;
          this.draggedItem = null;
          this.editingEdgeId = null;
        }

        setFRouterId(id) {
          this.frouterId = id;
        }

        async init() {
          await this.loadGraph();
          this.bindEvents();
          this.initRuleTemplates();
        }

        async loadNodes() {
          try {
            const payload = await this.api.get('/nodes');
            this.nodes = Array.isArray(payload.nodes) ? payload.nodes : [];
          } catch (err) {
            console.error('加载节点失败:', err);
            this.nodes = [];
          }
        }

        async loadGraph() {
          try {
            await this.loadNodes();
            const path = this.frouterId
              ? `/frouters/${encodeURIComponent(this.frouterId)}/graph`
              : '/graph';
            const data = await this.api.get(path);
            const allEdges = Array.isArray(data.edges) ? data.edges : [];
            this.edges = allEdges.filter(e => (e?.from || '') === 'local');
            this.detourEdges = allEdges.filter(e => (e?.from || '') !== 'local');
            this.slots = Array.isArray(data.slots) ? data.slots : [];
            this.sortEdgesByPriority();
            this.render();
            this.dirty = false;
            this.updateStatus();
          } catch (err) {
            console.error('加载图数据失败:', err);
            this.edges = [];
            this.detourEdges = [];
            this.slots = [];
            this.render();
          }
        }

        sortEdgesByPriority() {
          this.edges.sort((a, b) => (b.priority || 0) - (a.priority || 0));
        }

        render() {
          const list = this.container.querySelector('#chain-rules-list');
          if (!list) return;

          if (this.edges.length === 0) {
            list.innerHTML = '<div class="chain-rules-empty">暂无路由规则，点击"添加规则"创建</div>';
            return;
          }

          list.innerHTML = this.edges.map((edge, index) => this.renderRuleItem(edge, index)).join('');
        }

        renderRuleItem(edge, index) {
          const target = this.getChainDisplay(edge);
          const targetClass = edge.to === 'direct' ? 'direct' : (edge.to === 'block' ? 'block' : '');
          const title = this.getRuleTitle(edge);
          const ruleInfo = `去向: ${target}`;
          const disabledClass = edge.enabled === false ? 'disabled' : '';

          return `
            <div class="chain-rule-item ${disabledClass}" data-edge-id="${escapeHtml(edge.id)}" data-index="${index}" draggable="true">
              <div class="chain-rule-drag">⋮⋮</div>
              <div class="chain-rule-main">
                <div class="chain-rule-target ${targetClass}">${escapeHtml(title)}</div>
                <div class="chain-rule-info">${escapeHtml(ruleInfo)}</div>
              </div>
              <div class="chain-rule-meta">
                <label class="chain-toggle">
                  <input type="checkbox" ${edge.enabled !== false ? 'checked' : ''} data-action="toggle">
                  <span class="chain-toggle-slider"></span>
                </label>
              </div>
              <div class="chain-rule-actions">
                <button data-action="edit" title="编辑">✎</button>
                <button class="delete" data-action="delete" title="删除">×</button>
              </div>
            </div>
          `;
        }

        getTargetDisplay(to) {
          if (to === 'direct') return '不代理（直连）';
          if (to === 'block') return '阻断';
          if (to && to.startsWith('slot-')) {
            const slot = this.slots.find(s => s.id === to);
            return slot?.name || to;
          }
          const node = this.nodes.find(n => n.id === to);
          return node?.name || to || '未知';
        }

        getChainDisplay(edge) {
          const to = (edge?.to && String(edge.to)) || '';
          if (to === 'direct' || to === 'block') {
            return this.getTargetDisplay(to);
          }
          const via = Array.isArray(edge?.via) ? edge.via : [];
          const hops = [to, ...via]
            .map(v => (v && String(v).trim()) || '')
            .filter(Boolean);
          if (hops.length === 0) return '未知';
          return hops.map(id => this.getTargetDisplay(id)).join(' → ');
        }

        fillRuleToSelect(selectEl, currentValue) {
          if (!selectEl) return;

          const value = (currentValue && String(currentValue)) || 'direct';
          const nodes = Array.isArray(this.nodes) ? [...this.nodes] : [];
          const slots = Array.isArray(this.slots) ? [...this.slots] : [];
          const seen = new Set();
          const optionHtml = (val, label) => `<option value="${escapeHtml(val)}">${escapeHtml(label)}</option>`;
          const hasValue = (v) => {
            if (v === 'direct' || v === 'block') return true;
            return nodes.some(n => n?.id === v) || slots.some(s => s?.id === v);
          };

          let html = '';
          if (value && !hasValue(value)) {
            html += optionHtml(value, `未知: ${value}`);
            seen.add(value);
          }

          const groups = groupNodesBySubscription(nodes);
          for (const group of groups) {
            const groupOptions = [];
            for (const n of group.nodes) {
              const id = n?.id;
              if (!id || seen.has(id)) continue;
              groupOptions.push(optionHtml(id, n?.name || id));
              seen.add(id);
            }
            if (groupOptions.length > 0) {
              html += `<optgroup label="${escapeHtml(group.label)}">${groupOptions.join('')}</optgroup>`;
            }
          }

          slots.sort((a, b) => {
            const an = (a?.name || a?.id || '').toString();
            const bn = (b?.name || b?.id || '').toString();
            return an.localeCompare(bn, "zh-Hans-CN");
          });

          const slotOptions = [];
          for (const s of slots) {
            const id = s?.id;
            if (!id || seen.has(id)) continue;
            const bound = s?.boundNodeId;
            let suffix = '';
            if (bound) {
              const bn = nodes.find(n => n?.id === bound);
              suffix = bn ? `（已绑定: ${bn?.name || bound}）` : `（已绑定: ${bound}）`;
            } else {
              suffix = '（未绑定）';
            }
            slotOptions.push(optionHtml(id, `${s?.name || id}${suffix}`));
            seen.add(id);
          }
          if (slotOptions.length > 0) {
            html += `<optgroup label="槽位">${slotOptions.join('')}</optgroup>`;
          }

          const builtinOptions = [];
          if (!seen.has('direct')) {
            builtinOptions.push(optionHtml('direct', '不代理（直连）'));
            seen.add('direct');
          }
          if (!seen.has('block')) {
            builtinOptions.push(optionHtml('block', '阻断'));
            seen.add('block');
          }
          if (builtinOptions.length > 0) {
            html += `<optgroup label="内置">${builtinOptions.join('')}</optgroup>`;
          }

          selectEl.innerHTML = html;
          selectEl.value = value;
        }

        fillChainHopSelect(selectEl, currentValue) {
          if (!selectEl) return;

          const value = (currentValue && String(currentValue)) || '';
          const nodes = Array.isArray(this.nodes) ? [...this.nodes] : [];
          const slots = Array.isArray(this.slots) ? [...this.slots] : [];
          const seen = new Set();
          const optionHtml = (val, label) => `<option value="${escapeHtml(val)}">${escapeHtml(label)}</option>`;
          const hasValue = (v) => {
            return nodes.some(n => n?.id === v) || slots.some(s => s?.id === v);
          };

          let html = '';
          if (value && !hasValue(value)) {
            html += optionHtml(value, `未知: ${value}`);
            seen.add(value);
          }

          const groups = groupNodesBySubscription(nodes);
          for (const group of groups) {
            const groupOptions = [];
            for (const n of group.nodes) {
              const id = n?.id;
              if (!id || seen.has(id)) continue;
              groupOptions.push(optionHtml(id, n?.name || id));
              seen.add(id);
            }
            if (groupOptions.length > 0) {
              html += `<optgroup label="${escapeHtml(group.label)}">${groupOptions.join('')}</optgroup>`;
            }
          }

          slots.sort((a, b) => {
            const an = (a?.name || a?.id || '').toString();
            const bn = (b?.name || b?.id || '').toString();
            return an.localeCompare(bn, "zh-Hans-CN");
          });

          const slotOptions = [];
          for (const s of slots) {
            const id = s?.id;
            if (!id || seen.has(id)) continue;
            const bound = s?.boundNodeId;
            let suffix = '';
            if (bound) {
              const bn = nodes.find(n => n?.id === bound);
              suffix = bn ? `（已绑定: ${bn?.name || bound}）` : `（已绑定: ${bound}）`;
            } else {
              suffix = '（未绑定）';
            }
            slotOptions.push(optionHtml(id, `${s?.name || id}${suffix}`));
            seen.add(id);
          }
          if (slotOptions.length > 0) {
            html += `<optgroup label="槽位">${slotOptions.join('')}</optgroup>`;
          }

          selectEl.innerHTML = html;

          if (value && seen.has(value)) {
            selectEl.value = value;
            return;
          }
          const firstOption = selectEl.querySelector('option');
          if (firstOption) {
            selectEl.value = firstOption.value;
          }
        }

        appendChainHopRow(listEl, currentValue) {
          if (!listEl) return;
          const row = document.createElement('div');
          row.className = 'rule-chain-item';
          const select = document.createElement('select');
          select.className = 'rule-chain-hop';
          this.fillChainHopSelect(select, currentValue);
          const delBtn = document.createElement('button');
          delBtn.type = 'button';
          delBtn.className = 'danger';
          delBtn.textContent = '×';
          delBtn.dataset.action = 'chain-remove';
          row.appendChild(select);
          row.appendChild(delBtn);
          listEl.appendChild(row);
        }

        renderChainEditor(edge) {
          const group = this.container.querySelector('#rule-chain-group');
          const list = this.container.querySelector('#rule-chain-list');
          const toSelect = this.container.querySelector('#rule-to');
          if (!group || !list || !toSelect) return;

          const to = toSelect.value || edge?.to || 'direct';
          const show = to !== 'direct' && to !== 'block';
          group.style.display = show ? 'block' : 'none';

          list.innerHTML = '';
          const via = Array.isArray(edge?.via) ? edge.via : [];
          for (const hop of via) {
            this.appendChainHopRow(list, hop);
          }
        }

        getRuleTitle(edge) {
          const ruleType = edge?.ruleType != null ? String(edge.ruleType) : '';
          const desc = edge?.description != null ? String(edge.description).trim() : '';

          if (!ruleType) {
            return desc || '默认（匹配所有流量）';
          }

          if (ruleType === 'route') {
            if (desc && desc.startsWith('模板:')) {
              return desc;
            }

            const tmpl = this.findMatchingTemplate(edge);
            if (tmpl?.name) {
              return `模板: ${tmpl.name}`;
            }

            const domains = Array.isArray(edge?.routeRule?.domains)
              ? edge.routeRule.domains.map(s => String(s).trim()).filter(Boolean)
              : [];
            const ips = Array.isArray(edge?.routeRule?.ips)
              ? edge.routeRule.ips.map(s => String(s).trim()).filter(Boolean)
              : [];
            const total = domains.length + ips.length;
            if (total > 0) {
              const first = domains.length > 0 ? domains[0] : ips[0];
              const kind = domains.length > 0 ? '域名' : 'IP';
              return `${kind}: ${first}${total > 1 ? ` +${total - 1}` : ''}`;
            }

            return desc || '自定义';
          }

          return desc || '自定义';
        }

        findMatchingTemplate(edge) {
          const rr = edge?.routeRule;
          if (!rr) return null;

          const normalize = (arr) => {
            if (!Array.isArray(arr)) return [];
            return arr
              .map(v => (v == null ? '' : String(v).trim()))
              .filter(Boolean)
              .sort();
          };

          const domains = normalize(rr.domains);
          const ips = normalize(rr.ips);
          const templates = (typeof RULE_TEMPLATES !== 'undefined' && RULE_TEMPLATES && Array.isArray(RULE_TEMPLATES.templates))
            ? RULE_TEMPLATES.templates
            : [];

          const equal = (a, b) => {
            if (a.length !== b.length) return false;
            for (let i = 0; i < a.length; i++) {
              if (a[i] !== b[i]) return false;
            }
            return true;
          };

          for (const tmpl of templates) {
            const tr = tmpl?.rule || {};
            if (equal(domains, normalize(tr.domains)) && equal(ips, normalize(tr.ips))) {
              return tmpl;
            }
          }

          return null;
        }

        bindEvents() {
          const list = this.container.querySelector('#chain-rules-list');
          if (list) {
            list.addEventListener('dragstart', (e) => this.onDragStart(e));
            list.addEventListener('dragover', (e) => this.onDragOver(e));
            list.addEventListener('dragend', (e) => this.onDragEnd(e));
            list.addEventListener('drop', (e) => this.onDrop(e));
            list.addEventListener('click', (e) => this.onItemClick(e));
            list.addEventListener('change', (e) => this.onToggleChange(e));
          }

          // 弹窗事件
          const dialog = this.container.querySelector('#rule-edit-dialog');
          if (dialog) {
            dialog.addEventListener('click', (e) => {
              if (e.target === dialog) this.closeEditDialog();
            });
          }
          this.container.querySelector('#rule-dialog-close')?.addEventListener('click', () => this.closeEditDialog());
          this.container.querySelector('#rule-cancel')?.addEventListener('click', () => this.closeEditDialog());
          this.container.querySelector('#rule-save')?.addEventListener('click', () => this.saveEditDialog());
          this.container.querySelector('#rule-delete')?.addEventListener('click', () => this.deleteCurrentRule());

          // 规则类型切换
          this.container.querySelector('#rule-type')?.addEventListener('change', (e) => {
            const section = this.container.querySelector('#rule-route-section');
            if (section) section.style.display = e.target.value === 'route' ? 'block' : 'none';
          });

          // 去向切换：直连/阻断时隐藏链编辑
          this.container.querySelector('#rule-to')?.addEventListener('change', (e) => {
            const group = this.container.querySelector('#rule-chain-group');
            if (!group) return;
            const v = e.target.value;
            const show = v !== 'direct' && v !== 'block';
            group.style.display = show ? 'block' : 'none';
          });

          // 链式代理：添加/删除节点
          this.container.querySelector('#rule-chain-add')?.addEventListener('click', () => {
            const list = this.container.querySelector('#rule-chain-list');
            if (!list) return;
            const selects = list.querySelectorAll('.rule-chain-hop');
            const last = selects.length > 0 ? selects[selects.length - 1].value : '';
            this.appendChainHopRow(list, last);
          });

          this.container.querySelector('#rule-chain-list')?.addEventListener('click', (e) => {
            if (e.target?.dataset?.action !== 'chain-remove') return;
            e.target.closest('.rule-chain-item')?.remove();
          });
        }

        // 拖拽排序
        onDragStart(e) {
          const item = e.target.closest('.chain-rule-item');
          if (!item) return;
          this.draggedItem = item;
          item.classList.add('dragging');
          e.dataTransfer.effectAllowed = 'move';
        }

        onDragOver(e) {
          e.preventDefault();
          e.dataTransfer.dropEffect = 'move';
          const item = e.target.closest('.chain-rule-item');
          if (!item || item === this.draggedItem) return;
          const list = this.container.querySelector('#chain-rules-list');
          list.querySelectorAll('.chain-rule-item').forEach(i => i.classList.remove('drag-over'));
          item.classList.add('drag-over');
        }

        onDragEnd(e) {
          this.container.querySelectorAll('.chain-rule-item').forEach(i => {
            i.classList.remove('dragging', 'drag-over');
          });
          this.draggedItem = null;
        }

        onDrop(e) {
          e.preventDefault();
          const dropTarget = e.target.closest('.chain-rule-item');
          if (!dropTarget || !this.draggedItem || dropTarget === this.draggedItem) return;

          const fromIndex = parseInt(this.draggedItem.dataset.index);
          const toIndex = parseInt(dropTarget.dataset.index);

          const [moved] = this.edges.splice(fromIndex, 1);
          this.edges.splice(toIndex, 0, moved);

          this.recalculatePriorities();
          this.render();
          this.markDirty();
        }

        recalculatePriorities() {
          const base = 100;
          this.edges.forEach((edge, index) => {
            edge.priority = base - index;
          });
        }

        // 操作事件
        onItemClick(e) {
          const action = e.target.closest('[data-action]')?.dataset.action;
          const item = e.target.closest('.chain-rule-item');
          if (!item || !action) return;
          if (action === 'toggle') return; // handled by onToggleChange

          const edgeId = item.dataset.edgeId;
          if (action === 'edit') {
            this.openEditDialog(edgeId);
          } else if (action === 'delete') {
            this.deleteRule(edgeId);
          }
        }

        onToggleChange(e) {
          if (e.target.dataset?.action !== 'toggle' && !e.target.closest('[data-action="toggle"]')) {
            if (e.target.type !== 'checkbox') return;
          }
          const item = e.target.closest('.chain-rule-item');
          if (!item) return;

          const edgeId = item.dataset.edgeId;
          const edge = this.edges.find(ed => ed.id === edgeId);
          if (edge) {
            edge.enabled = e.target.checked;
            this.markDirty();
            this.render();
          }
        }

        // CRUD
        addRule() {
          const newEdge = {
            id: `rule-${crypto.randomUUID()}`,
            from: 'local',
            to: 'direct',
            via: [],
            priority: this.edges.length > 0 ? Math.max(...this.edges.map(e => e.priority || 0)) + 10 : 100,
            enabled: true,
            ruleType: '',
            routeRule: null,
            description: ''
          };
          this.edges.unshift(newEdge);
          this.recalculatePriorities();
          this.render();
          this.markDirty();
          this.openEditDialog(newEdge.id);
        }

        deleteRule(edgeId) {
          if (!confirm('确定要删除此规则？')) return;
          this.edges = this.edges.filter(e => e.id !== edgeId);
          this.recalculatePriorities();
          this.render();
          this.markDirty();
        }

        openEditDialog(edgeId) {
          const edge = this.edges.find(e => e.id === edgeId);
          if (!edge) return;

          this.editingEdgeId = edgeId;
          const dialog = this.container.querySelector('#rule-edit-dialog');
          const title = this.container.querySelector('#rule-dialog-title');
          const deleteBtn = this.container.querySelector('#rule-delete');

          if (title) title.textContent = edge.id.startsWith('rule-') && this.edges.indexOf(edge) === 0 ? '添加规则' : '编辑规则';
          if (deleteBtn) deleteBtn.style.display = 'block';

          // 填充表单
          const toSelect = this.container.querySelector('#rule-to');
          const typeSelect = this.container.querySelector('#rule-type');
          const enabledCheck = this.container.querySelector('#rule-enabled');
          const descInput = this.container.querySelector('#rule-description');
          const domainsInput = this.container.querySelector('#rule-domains');
          const ipsInput = this.container.querySelector('#rule-ips');
          const routeSection = this.container.querySelector('#rule-route-section');

          if (toSelect) this.fillRuleToSelect(toSelect, edge.to || 'direct');
          if (typeSelect) typeSelect.value = edge.ruleType || '';
          if (enabledCheck) enabledCheck.checked = edge.enabled !== false;
          if (descInput) descInput.value = edge.description || '';
          if (domainsInput) domainsInput.value = (edge.routeRule?.domains || []).join('\n');
          if (ipsInput) ipsInput.value = (edge.routeRule?.ips || []).join('\n');
          if (routeSection) routeSection.style.display = edge.ruleType === 'route' ? 'block' : 'none';
          this.renderChainEditor(edge);

          dialog?.classList.add('open');
        }

        closeEditDialog() {
          this.editingEdgeId = null;
          const dialog = this.container.querySelector('#rule-edit-dialog');
          dialog?.classList.remove('open');
        }

        saveEditDialog() {
          const edge = this.edges.find(e => e.id === this.editingEdgeId);
          if (!edge) return;

          const toSelect = this.container.querySelector('#rule-to');
          const typeSelect = this.container.querySelector('#rule-type');
          const enabledCheck = this.container.querySelector('#rule-enabled');
          const descInput = this.container.querySelector('#rule-description');
          const domainsInput = this.container.querySelector('#rule-domains');
          const ipsInput = this.container.querySelector('#rule-ips');

          edge.to = toSelect?.value || 'direct';
          edge.ruleType = typeSelect?.value || '';
          edge.enabled = enabledCheck?.checked !== false;
          edge.description = descInput?.value || '';
          if (edge.to === 'direct' || edge.to === 'block') {
            edge.via = [];
          } else {
            const hopSelects = Array.from(this.container.querySelectorAll('#rule-chain-list .rule-chain-hop'));
            edge.via = hopSelects
              .map(sel => (sel?.value || '').trim())
              .filter(v => v && v !== 'direct' && v !== 'block' && v !== 'local');
          }

          if (edge.ruleType === 'route') {
            const domains = (domainsInput?.value || '').split('\n').map(s => s.trim()).filter(Boolean);
            const ips = (ipsInput?.value || '').split('\n').map(s => s.trim()).filter(Boolean);
            edge.routeRule = { domains, ips };
          } else {
            edge.routeRule = null;
          }

          this.render();
          this.markDirty();
          this.closeEditDialog();
        }

        deleteCurrentRule() {
          if (this.editingEdgeId) {
            this.closeEditDialog();
            this.deleteRule(this.editingEdgeId);
          }
        }

        // 保存
        async saveGraph() {
          try {
            const path = this.frouterId
              ? `/frouters/${encodeURIComponent(this.frouterId)}/graph`
              : '/graph';

            const data = {
              edges: [...this.edges, ...this.detourEdges],
              slots: this.slots,
              positions: {}
            };

            await this.api.put(path, data);
            this.dirty = false;
            this.updateStatus();
            return { success: true };
          } catch (err) {
            console.error('保存失败:', err);
            return { success: false, error: err.message };
          }
        }

        markDirty() {
          this.dirty = true;
          this.updateStatus();
        }

        updateStatus() {
          const statusEl = this.container.querySelector('#chain-status');
          if (statusEl) {
            statusEl.textContent = this.dirty ? '有未保存的更改' : '已保存';
            statusEl.className = `toolbar-status ${this.dirty ? 'dirty' : ''}`;
          }
        }

        // 规则模板
        initRuleTemplates() {
          const categorySelect = this.container.querySelector('#rule-template-category');
          const templateSelect = this.container.querySelector('#rule-template-select');
          const applyBtn = this.container.querySelector('#rule-template-apply');

          if (!categorySelect || !templateSelect) return;

          const categories = typeof getTemplateCategories === 'function' ? getTemplateCategories() : [];
          categorySelect.innerHTML = '<option value="all">全部分类</option>';
          for (const cat of categories) {
            const opt = document.createElement('option');
            opt.value = cat.id;
            opt.textContent = `${cat.icon} ${cat.name}`;
            categorySelect.appendChild(opt);
          }

          const updateTemplateSelect = (categoryId) => {
            const templates = typeof getTemplatesByCategory === 'function' ? getTemplatesByCategory(categoryId) : [];
            templateSelect.innerHTML = '<option value="">选择模板...</option>';
            for (const tmpl of templates) {
              const opt = document.createElement('option');
              opt.value = tmpl.id;
              opt.textContent = `${tmpl.icon} ${tmpl.name}`;
              opt.title = tmpl.description;
              templateSelect.appendChild(opt);
            }
          };

          updateTemplateSelect('all');
          categorySelect.addEventListener('change', (e) => updateTemplateSelect(e.target.value));

          const applyTemplate = (templateId) => {
            if (!templateId) return;

            const template = typeof getTemplateById === 'function' ? getTemplateById(templateId) : null;
            if (!template || !template.rule) {
              showStatus('模板数据无效', 'error', 2000);
              return;
            }

            const toSelect = this.container.querySelector('#rule-to');
            if (toSelect && (template.action === 'direct' || template.action === 'block')) {
              toSelect.value = template.action;
            }

            const domainsInput = this.container.querySelector('#rule-domains');
            const ipsInput = this.container.querySelector('#rule-ips');
            const descInput = this.container.querySelector('#rule-description');

            const mergeLines = (base, extra) => {
              const out = [];
              const seen = new Set();
              for (const item of base) {
                if (!item) continue;
                if (seen.has(item)) continue;
                seen.add(item);
                out.push(item);
              }
              for (const item of extra) {
                if (!item) continue;
                if (seen.has(item)) continue;
                seen.add(item);
                out.push(item);
              }
              return out;
            };

            if (domainsInput) {
              const existing = (domainsInput.value || '').split('\n').map(s => s.trim()).filter(Boolean);
              const domains = Array.isArray(template.rule.domains) ? template.rule.domains : [];
              const merged = mergeLines(existing, domains);
              domainsInput.value = merged.join('\n');
            }

            if (ipsInput) {
              const existing = (ipsInput.value || '').split('\n').map(s => s.trim()).filter(Boolean);
              const ips = Array.isArray(template.rule.ips) ? template.rule.ips : [];
              const merged = mergeLines(existing, ips);
              ipsInput.value = merged.join('\n');
            }

            if (descInput) {
              const prev = String(descInput.value || '').trim();
              if (!prev) {
                descInput.value = `模板: ${template.name}`;
              } else if (prev.startsWith('模板:') && !prev.includes(template.name)) {
                descInput.value = `${prev} + ${template.name}`;
              }
            }

            showStatus(`已应用模板: ${template.name}`, 'success', 2000);
          };

          templateSelect.addEventListener('change', (e) => {
            applyTemplate(e.target.value);
          });

          if (applyBtn) {
            applyBtn.addEventListener('click', () => {
              if (!templateSelect.value) {
                showStatus('请先选择一个模板', 'error', 2000);
                return;
              }
              applyTemplate(templateSelect.value);
            });
          }
        }
      }

      async function initChainEditor() {
        if (chainEditorInitialized) return;

        const container = document.getElementById('panel-chain');
        if (!container) return;

        async function waitForProxyRestartError(prevRestartAt) {
          const timeoutMs = 20000;
          const scheduleWaitMs = 2000;
          const intervalMs = 500;

          const deadline = Date.now() + timeoutMs;
          const scheduleDeadline = Date.now() + scheduleWaitMs;

          let scheduled = false;
          while (Date.now() < deadline) {
            const status = await api.proxy.status();
            if (status && status.busy) {
              await sleep(intervalMs);
              continue;
            }

            const currentRestartAt = status && typeof status.lastRestartAt === 'string' ? status.lastRestartAt : '';
            if (!scheduled) {
              if (currentRestartAt && currentRestartAt !== prevRestartAt) {
                scheduled = true;
              } else if (Date.now() > scheduleDeadline) {
                return null;
              }
            }

            if (scheduled) {
              const restartError = status && typeof status.lastRestartError === 'string' ? status.lastRestartError : '';
              if (restartError) return restartError;
            }

            await sleep(intervalMs);
          }

          return null;
        }

        const chainApi = {
          get: async (path) => api.get(path),
          put: async (path, data) => api.put(path, data),
          post: async (path, data) => api.post(path, data)
        };

        chainListEditor = new ChainListEditor(container, chainApi);
        const current = getCurrentFRouter();
        chainListEditor.setFRouterId(current ? current.id : "");
        await chainListEditor.init();
        chainEditorInitialized = true;

        // 工具栏事件
        document.getElementById('chain-save')?.addEventListener('click', async () => {
          let prevRestartAt = '';
          try {
            const status = await api.proxy.status();
            prevRestartAt = status && typeof status.lastRestartAt === 'string' ? status.lastRestartAt : '';
          } catch {
            prevRestartAt = '';
          }

          const result = await chainListEditor.saveGraph();
          if (result.success) {
            showStatus('配置已保存', 'success');

            (async () => {
              try {
                const restartError = await waitForProxyRestartError(prevRestartAt);
                if (restartError) {
                  showStatus(`代理重启失败：${restartError}`, 'error', 6000);
                }
              } catch (err) {
                console.warn('Failed to check proxy restart error:', err);
              }
            })();
          } else {
            showStatus('保存失败: ' + result.error, 'error', 5000);
          }
        });

        document.getElementById('chain-reset')?.addEventListener('click', async () => {
          if (confirm('确定要重置为上次保存的配置吗？')) {
            await chainListEditor.loadGraph();
            showStatus('已重置', 'info');
          }
        });

        document.getElementById('chain-add-rule')?.addEventListener('click', () => {
          chainListEditor.addRule();
        });

        console.log('[ChainListEditor] 初始化完成');
      }

		      const loaders = {
		        "panel-home": () => loadHomePanel(),
		        panel1: loadFRouters,
		        panel2: loadConfigs,
		        "panel-nodes": () => {
		          startNodesPolling();
		          return loadNodes();
		        },
		        panel3: () => loadComponents(),
		        "panel-logs": () => {
		          ensureLogsTabs();
		          ensureKernelLogsPanel();
		          ensureAppLogsPanel();
		          startKernelLogsPolling();
		          startAppLogsPolling();
		          syncLogsTabUI();
		          return currentLogsTab === "app" ? loadAppLogs() : loadKernelLogs();
		        },
		        "panel-chain": initChainEditor,
	        "panel-settings": async () => {
	          await loadSystemProxySettings();
	        },
      };

      function openChainEditorPanel() {
        const target = "panel-chain";
        if (currentPanel === "panel-nodes") {
          stopNodesPolling();
        }
        if (currentPanel === "panel-logs") {
          stopKernelLogsPolling();
          stopAppLogsPolling();
        }
        if (target === currentPanel) {
          loaders[target]?.();
          return;
        }
        currentPanel = target;
        menu.querySelectorAll("button").forEach((btn) => btn.classList.remove("active"));
        // Chain Editor 作为 FRouter 面板的一部分：保持左侧高亮在 FRouter
        const activeBtn = menu.querySelector('button[data-target="panel1"]');
        if (activeBtn) activeBtn.classList.add("active");
        panels.forEach((panel) => {
          panel.classList.toggle("active", panel.id === target);
        });
        loaders[target]?.();
      }

      menu.addEventListener("click", (event) => {
        const button = event.target.closest("button[data-target]");
        if (!button) return;
        const target = button.dataset.target;
        if (target === currentPanel) return;

        const prevPanel = currentPanel;
        if (prevPanel === "panel-nodes" && target !== "panel-nodes") {
          stopNodesPolling();
        }
        if (prevPanel === "panel-logs" && target !== "panel-logs") {
          stopKernelLogsPolling();
          stopAppLogsPolling();
        }
        currentPanel = target;
        menu.querySelectorAll("button").forEach((btn) => btn.classList.remove("active"));
        button.classList.add("active");
        panels.forEach((panel) => {
          panel.classList.toggle("active", panel.id === target);
        });
        if (target === "panel-nodes") {
          startNodesPolling();
        }
        if (target === "panel-logs") {
          startKernelLogsPolling();
          startAppLogsPolling();
        }
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
		      const chainRouteSelect = document.getElementById("chain-route-select");
	      const nodeRefreshBtn = document.getElementById("node-refresh");
	      nodeRefreshBtn.addEventListener("click", () => loadFRouters({ notify: true }));
	      document.getElementById("config-refresh").addEventListener("click", () => loadConfigs({ notify: true }));
	      document.getElementById("component-refresh").addEventListener("click", () => loadComponents({ notify: true }));

	      const nodesTable = document.getElementById("nodes-table");
	      if (nodesTable) {
	        nodesTable.addEventListener("click", async (event) => {
	          const button = event.target.closest("button[data-action]");
	          if (!button) return;
	          const row = button.closest("tr[data-id]");
	          if (!row) return;
	          const id = row.dataset.id;
	          if (!id) return;

	          const action = button.dataset.action;
	          try {
            if (action === "ping-node") {
              await api.post(`/nodes/${id}/ping`, {});
              showStatus("节点延迟任务已提交", "info");
            } else if (action === "speedtest-node") {
              await api.post(`/nodes/${id}/speedtest`, {});
              showStatus("节点测速任务已提交", "info");
            } else if (action === "edit-node") {
              const node = Array.isArray(nodesCache) ? nodesCache.find((item) => item.id === id) : null;
              if (!node) {
                showStatus("节点未找到", "error", 4000);
                return;
              }
              openNodesModal(node);
              return;
            }

            setTimeout(() => {
              if (currentPanel === "panel-nodes") {
                loadNodes();
	              }
	            }, 1200);
	          } catch (err) {
	            showStatus(`节点操作失败：${err.message}`, "error", 6000);
	          }
	        });
	      }

	      const nodesBulkPingButton = document.getElementById("nodes-bulk-ping");
	      if (nodesBulkPingButton) {
	        nodesBulkPingButton.addEventListener("click", async () => {
	          if (!Array.isArray(nodesCache) || nodesCache.length === 0) {
	            showStatus("暂无节点", "error");
	            return;
	          }
	          try {
	            await api.post("/nodes/bulk/ping", {});
	            showStatus("批量延迟任务已排队", "info");
	            setTimeout(() => currentPanel === "panel-nodes" && loadNodes(), 1200);
	            setTimeout(() => currentPanel === "panel-nodes" && loadNodes(), 3000);
	          } catch (err) {
	            showStatus(`批量测延迟失败：${err.message}`, "error", 6000);
	          }
	        });
	      }

	      const nodesBulkSpeedButton = document.getElementById("nodes-bulk-speed");
	      if (nodesBulkSpeedButton) {
	        nodesBulkSpeedButton.addEventListener("click", async () => {
	          if (!Array.isArray(nodesCache) || nodesCache.length === 0) {
	            showStatus("暂无节点", "error");
	            return;
	          }
	          try {
	            await api.post("/nodes/bulk/speedtest", {});
	            showStatus("批量测速任务已排队", "info");
	            setTimeout(() => currentPanel === "panel-nodes" && loadNodes(), 1200);
	            setTimeout(() => currentPanel === "panel-nodes" && loadNodes(), 5000);
	          } catch (err) {
	            showStatus(`批量测速失败：${err.message}`, "error", 6000);
	          }
	        });
	      }

		      function ensureFRouterContextMenu() {
		        let menu = document.getElementById("frouter-context-menu");
		        if (menu) return menu;

	        menu = document.createElement("div");
	        menu.id = "frouter-context-menu";
	        // 复用 chain-editor.css 的右键菜单样式
	        menu.className = "slot-context-menu";
	        menu.innerHTML = `
	          <div class="slot-context-menu-item" data-action="open-chain">链路编辑</div>
	        `;
	        document.body.appendChild(menu);

	        menu.addEventListener("click", (e) => {
	          const item = e.target.closest(".slot-context-menu-item[data-action]");
	          if (!item) return;
	          menu.classList.remove("visible");

	          const action = item.dataset.action;
	          const frouterId = menu.dataset.frouterId || "";
	          if (action === "open-chain") {
	            if (frouterId) setCurrentFRouter(frouterId, { notify: false });
	            openChainEditorPanel();
	          }
	        });

	        document.addEventListener("click", () => menu.classList.remove("visible"));
	        window.addEventListener("blur", () => menu.classList.remove("visible"));
	        return menu;
	      }

      document.getElementById("chain-back")?.addEventListener("click", () => {
        const btn = menu.querySelector('button[data-target="panel1"]');
        if (btn) {
          btn.click();
          return;
        }
        currentPanel = "panel1";
        panels.forEach((panel) => {
          panel.classList.toggle("active", panel.id === "panel1");
        });
        loaders.panel1?.();
      });

      // TUN status click handler
      const tunStatusValue = document.getElementById("tun-status-value");
      if (tunStatusValue) {
        tunStatusValue.addEventListener("click", showTUNStatusDialog);
      }

	      // TUN toggle handler
	      const tunToggle = document.getElementById("tun-toggle");
	      if (tunToggle) {
	        tunToggle.addEventListener("change", (event) => {
	          updateTUNSetting(event.target.checked);
	        });
	      }

	      // Engine select handler
	      const engineSelect = document.getElementById("engine-select");
	      if (engineSelect) {
	        engineSelect.addEventListener("change", (event) => {
	          updateEngineSetting(event.target.value);
	        });
	      }

	      const systemProxyIgnoreInput = document.getElementById("system-proxy-ignore");
	      const systemProxySaveButton = document.getElementById("system-proxy-save");
	      const systemProxyResetButton = document.getElementById("system-proxy-reset");
	      const proxyToggleButton = document.getElementById("proxy-toggle");
	      if (proxyToggleButton) {
        proxyToggleButton.addEventListener("click", handleProxyToggle);
      }

      if (chainRouteSelect) {
        chainRouteSelect.addEventListener("change", (event) => {
          setCurrentFRouter(event.target.value, { notify: true });
        });
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
          const frouterName = form.name.value.trim();
          const tags = parseList(form.tags.value);

		          if (!frouterName) {
		            showStatus("请填写 FRouter 名称", "error");
		            return;
		          }

		          try {
		            await api.post("/frouters", { name: frouterName, tags });
		            form.reset();
		            showStatus("FRouter 添加成功", "success");
		            closeNodeModal();
		            await loadFRouters();
		          } catch (err) {
		            showStatus(`添加 FRouter 失败：${err.message}`, "error", 6000);
          }
        });
      }

      const nodesAddButton = document.getElementById("nodes-add-button");
      const nodesModal = document.getElementById("nodes-modal");
      const nodesModalClose = document.getElementById("nodes-modal-close");
      const nodesModalReset = document.getElementById("nodes-modal-reset");
      const nodesModalBackdrop = nodesModal ? nodesModal.querySelector(".modal-backdrop") : null;
      const nodesForm = document.getElementById("nodes-form");
      const nodesModalTitle = document.getElementById("nodes-modal-title");
      const nodesSubmitButton = nodesForm ? nodesForm.querySelector('button[type="submit"]') : null;
      const nodesProtocolSelect = document.getElementById("nodes-protocol-select");
      const nodesNetworkSelect = document.getElementById("nodes-network-select");
      const nodesTLSSelect = document.getElementById("nodes-tls-select");
      let editingNode = null;

      function setNodesModalMode(isEdit) {
        if (nodesModalTitle) {
          nodesModalTitle.textContent = isEdit ? "编辑节点" : "添加节点";
        }
        if (nodesSubmitButton) {
          nodesSubmitButton.textContent = isEdit ? "保存修改" : "创建节点";
        }
      }

      function updateNodesFieldVisibility() {
        if (!nodesForm) return;
        const protocol = nodesProtocolSelect?.value || "";
        const network = nodesNetworkSelect?.value || "tcp";
        const tls = nodesTLSSelect?.value || "";

        const ssFields = document.getElementById("nodes-ss-fields");
        const vmessFields = document.getElementById("nodes-vmess-fields");
        const trojanFields = document.getElementById("nodes-trojan-fields");
        if (ssFields) ssFields.style.display = "none";
        if (vmessFields) vmessFields.style.display = "none";
        if (trojanFields) trojanFields.style.display = "none";

        if (protocol === "shadowsocks" && ssFields) {
          ssFields.style.display = "grid";
        } else if ((protocol === "vmess" || protocol === "vless") && vmessFields) {
          vmessFields.style.display = "grid";
        } else if (protocol === "trojan" && trojanFields) {
          trojanFields.style.display = "block";
        }

        const wsFields = document.getElementById("nodes-ws-fields");
        const httpFields = document.getElementById("nodes-http-fields");
        const grpcFields = document.getElementById("nodes-grpc-fields");
        if (wsFields) wsFields.style.display = "none";
        if (httpFields) httpFields.style.display = "none";
        if (grpcFields) grpcFields.style.display = "none";

        if (network === "ws" && wsFields) {
          wsFields.style.display = "grid";
        } else if ((network === "http" || network === "h2") && httpFields) {
          httpFields.style.display = "grid";
        } else if (network === "grpc" && grpcFields) {
          grpcFields.style.display = "block";
        }

        const tlsFields = document.getElementById("nodes-tls-fields");
        if (tlsFields) {
          tlsFields.style.display = tls ? "grid" : "none";
        }
      }

      function fillNodesForm(node) {
        if (!nodesForm || !node) return;
        nodesForm.name.value = node.name || "";
        nodesForm.address.value = node.address || "";
        nodesForm.port.value = node.port ? String(node.port) : "";
        nodesForm.protocol.value = node.protocol || "shadowsocks";
        nodesForm.tags.value = Array.isArray(node.tags) ? node.tags.join(",") : "";

        const sec = node.security || {};
        nodesForm.ss_method.value = sec.method || "aes-256-gcm";
        nodesForm.ss_password.value = sec.password || "";
        nodesForm.vmess_uuid.value = sec.uuid || "";
        nodesForm.vmess_alterid.value = Number.isFinite(sec.alterId) ? String(sec.alterId) : "0";
        nodesForm.vmess_security.value = sec.encryption || sec.method || "auto";
        nodesForm.vless_flow.value = sec.flow || "";
        nodesForm.trojan_password.value = sec.password || "";

        const transportType = node.transport?.type || "tcp";
        nodesForm.network.value = transportType;
        nodesForm.ws_path.value = node.transport?.path || "";
        nodesForm.ws_host.value = node.transport?.host || "";
        nodesForm.h2_path.value = node.transport?.path || "";
        nodesForm.h2_host.value = node.transport?.host || "";
        nodesForm.grpc_service.value = node.transport?.serviceName || "";

        const tlsEnabled = node.tls && node.tls.enabled;
        nodesForm.tls.value = tlsEnabled ? (node.tls.type || "tls") : "";
        nodesForm.tls_sni.value = node.tls?.serverName || "";
        nodesForm.tls_fingerprint.value = node.tls?.fingerprint || "";
        nodesForm.tls_insecure.value = node.tls?.insecure ? "true" : "false";

        updateNodesFieldVisibility();
      }

      function setNodesReadonlyForSubscription(enabled) {
        if (!nodesForm) return;
        const lockedNames = [
          "address",
          "port",
          "protocol",
          "ss_method",
          "ss_password",
          "vmess_uuid",
          "vmess_alterid",
          "vmess_security",
          "vless_flow",
          "trojan_password",
          "network",
          "ws_path",
          "ws_host",
          "h2_path",
          "h2_host",
          "grpc_service",
          "tls",
          "tls_sni",
          "tls_fingerprint",
          "tls_insecure",
        ];
        for (const name of lockedNames) {
          const el = nodesForm.querySelector(`[name="${name}"]`);
          if (!el) continue;
          el.disabled = Boolean(enabled);
        }
      }

      function openNodesModal(node = null) {
        if (!nodesModal) return;
        nodesModal.classList.add("open");
        editingNode = node;
        if (nodesForm) nodesForm.reset();
        const isEdit = Boolean(editingNode);
        setNodesModalMode(isEdit);
        const shareLinkRow = document.getElementById("nodes-sharelink-row");
        if (shareLinkRow) {
          shareLinkRow.style.display = isEdit ? "none" : "block";
        }
        if (editingNode) {
          fillNodesForm(editingNode);
          const subscription = Boolean(editingNode.sourceConfigId);
          setNodesReadonlyForSubscription(subscription);
          if (subscription) {
            showStatus("订阅节点仅支持修改名称/标签", "info", 3200);
          }
        } else {
          setNodesReadonlyForSubscription(false);
          updateNodesFieldVisibility();
        }
      }

      function closeNodesModal() {
        if (!nodesModal) return;
        nodesModal.classList.remove("open");
        editingNode = null;
      }

      if (nodesAddButton) {
        nodesAddButton.addEventListener("click", () => openNodesModal());
      }
      if (nodesModalClose) {
        nodesModalClose.addEventListener("click", closeNodesModal);
      }
      if (nodesModalBackdrop) {
        nodesModalBackdrop.addEventListener("click", closeNodesModal);
      }
      document.addEventListener("keydown", (event) => {
        if (event.key === "Escape" && nodesModal?.classList.contains("open")) {
          closeNodesModal();
        }
      });
      if (nodesModalReset && nodesForm) {
        nodesModalReset.addEventListener("click", () => {
          if (editingNode) {
            fillNodesForm(editingNode);
            return;
          }
          nodesForm.reset();
          updateNodesFieldVisibility();
        });
      }

      if (nodesProtocolSelect) {
        nodesProtocolSelect.addEventListener("change", updateNodesFieldVisibility);
      }
      if (nodesNetworkSelect) {
        nodesNetworkSelect.addEventListener("change", updateNodesFieldVisibility);
      }
      if (nodesTLSSelect) {
        nodesTLSSelect.addEventListener("change", updateNodesFieldVisibility);
      }

      if (nodesForm) {
        nodesForm.addEventListener("submit", async (event) => {
          event.preventDefault();
          const form = event.target;
          const isEdit = Boolean(editingNode && editingNode.id);
          const isSubscriptionEdit = Boolean(isEdit && editingNode.sourceConfigId);
          const shareLink = (form.shareLink?.value || "").trim();
          const tags = parseList(form.tags.value);

          if (!isEdit && shareLink) {
            const payload = { shareLink };
            if (tags.length) {
              payload.tags = tags;
            }
            try {
              const resp = await api.post("/nodes/from-link", payload);
              const count = Array.isArray(resp?.nodes) ? resp.nodes.length : 0;
              showStatus(count > 1 ? `已导入 ${count} 个节点` : "节点已添加", "success");
              form.reset();
              closeNodesModal();
              await loadNodes();
            } catch (err) {
              showStatus(`导入节点失败：${err.message}`, "error", 6000);
            }
            return;
          }

          if (isSubscriptionEdit) {
            const name = form.name.value.trim();
            if (!name) {
              showStatus("请填写节点名称", "error", 5000);
              return;
            }
            try {
              await api.put(`/nodes/${editingNode.id}/meta`, { name, tags });
              showStatus("节点已更新", "success");
              closeNodesModal();
              await loadNodes();
            } catch (err) {
              showStatus(`更新节点失败：${err.message}`, "error", 6000);
            }
            return;
          }

          const protocol = form.protocol.value;
          const payload = {
            name: form.name.value.trim(),
            address: form.address.value.trim(),
            port: parseNumber(form.port.value),
            protocol,
          };
          if (tags.length) {
            payload.tags = tags;
          }
          if (!payload.name) {
            showStatus("请填写节点名称", "error", 5000);
            return;
          }
          if (!payload.address) {
            showStatus("请填写服务器地址", "error", 5000);
            return;
          }
          if (!payload.port || payload.port <= 0) {
            showStatus("请填写有效端口", "error", 5000);
            return;
          }

          const security = {};
          if (protocol === "shadowsocks") {
            security.method = form.ss_method.value.trim();
            security.password = form.ss_password.value.trim();
            if (!security.method || !security.password) {
              showStatus("Shadowsocks 需要加密方式和密码", "error", 5000);
              return;
            }
          } else if (protocol === "vmess" || protocol === "vless") {
            security.uuid = form.vmess_uuid.value.trim();
            security.alterId = parseNumber(form.vmess_alterid.value);
            const enc = form.vmess_security.value.trim();
            if (enc) {
              security.encryption = enc;
              security.method = enc;
            }
            const flow = form.vless_flow.value.trim();
            if (flow) security.flow = flow;
            if (!security.uuid) {
              showStatus("VMess/VLESS 需要填写 UUID", "error", 5000);
              return;
            }
          } else if (protocol === "trojan") {
            security.password = form.trojan_password.value.trim();
            if (!security.password) {
              showStatus("Trojan 需要填写密码", "error", 5000);
              return;
            }
          }
          if (Object.keys(security).length > 0) {
            payload.security = security;
          }

          const network = form.network.value || "tcp";
          if (network && network !== "tcp") {
            const transport = { type: network };
            if (network === "ws") {
              transport.path = form.ws_path.value.trim();
              transport.host = form.ws_host.value.trim();
            } else if (network === "http" || network === "h2") {
              transport.path = form.h2_path.value.trim();
              transport.host = form.h2_host.value.trim();
            } else if (network === "grpc") {
              transport.serviceName = form.grpc_service.value.trim();
            }
            payload.transport = transport;
          }

          const tlsType = form.tls.value;
          if (tlsType) {
            const tls = {
              enabled: true,
              type: tlsType,
              insecure: form.tls_insecure.value === "true",
            };
            const sni = form.tls_sni.value.trim();
            const fp = form.tls_fingerprint.value.trim();
            if (sni) tls.serverName = sni;
            if (fp) tls.fingerprint = fp;
            payload.tls = tls;
          }

          try {
            if (editingNode && editingNode.id) {
              await api.put(`/nodes/${editingNode.id}`, payload);
              showStatus("节点已更新", "success");
            } else {
              await api.post("/nodes", payload);
              showStatus("节点已添加", "success");
              form.reset();
            }
            closeNodesModal();
            await loadNodes();
          } catch (err) {
            showStatus(`${editingNode ? "更新" : "添加"}节点失败：${err.message}`, "error", 6000);
          }
        });
      }

      if (nodeGrid) {
        nodeGrid.addEventListener("click", async (event) => {
          const row = event.target.closest(".node-row[data-id]");
          if (!row) return;
	          const id = row.dataset.id;
          const actionTarget = event.target.closest("[data-action]");
          const action = actionTarget ? actionTarget.dataset.action : null;
          if (!action) {
            setCurrentFRouter(id, { notify: false });
            await applyFRouterSelection(id, { notify: true });
            return;
          }
          try {
            if (action === "ping-route") {
              await api.post(`/frouters/${id}/ping`);
              showStatus("延迟任务已排队", "info");
              await loadFRouters();
            } else if (action === "speed-route") {
              await api.post(`/frouters/${id}/speedtest`);
              showStatus("测速任务已排队", "info");
              await loadFRouters();
            }
          } catch (err) {
            showStatus(`操作失败：${err.message}`, "error", 6000);
	          }
	        });

	        nodeGrid.addEventListener("dblclick", (event) => {
	          const row = event.target.closest(".node-row[data-id]");
	          if (!row) return;
	          if (event.target.closest("[data-action]")) return;
	          setCurrentFRouter(row.dataset.id, { notify: false });
	          openChainEditorPanel();
	        });

	        nodeGrid.addEventListener("contextmenu", (event) => {
	          const row = event.target.closest(".node-row[data-id]");
	          if (!row) return;
	          event.preventDefault();

	          const id = row.dataset.id;
	          setCurrentFRouter(id, { notify: false, reloadGraph: false });

	          const menu = ensureFRouterContextMenu();
	          menu.dataset.frouterId = id;

	          const maxLeft = Math.max(0, window.innerWidth - 200);
	          const maxTop = Math.max(0, window.innerHeight - 120);
	          menu.style.left = `${Math.min(event.clientX, maxLeft)}px`;
	          menu.style.top = `${Math.min(event.clientY, maxTop)}px`;
	          menu.classList.add("visible");
	        });
	      }

      if (nodeTabs) {
        nodeTabs.addEventListener("click", (event) => {
          const button = event.target.closest(".node-tab[data-tag]");
          if (!button) return;
          const tag = button.dataset.tag;
          if (tag === currentFRouterTab) return;
          currentFRouterTab = tag;
          nodeTabs.querySelectorAll(".node-tab").forEach((tab) => tab.classList.remove("active"));
          button.classList.add("active");
          renderFRouters(froutersCache, currentFRouterId);
        });
      }

      const configModal = document.getElementById("config-modal");
      const configAddButton = document.getElementById("config-add-button");
      const configModalClose = document.getElementById("config-modal-close");
      const configModalBackdrop = configModal?.querySelector(".modal-backdrop");
      const configModalReset = document.getElementById("config-modal-reset");
      const configForm = document.getElementById("config-form");
      const configModalTitle = document.getElementById("config-modal-title");
      const configSubmitButton = configForm ? configForm.querySelector('button[type="submit"]') : null;
      let editingConfig = null;

      function durationToMinutes(value) {
        if (!value) return 0;
        if (typeof value === "number") {
          if (!Number.isFinite(value) || value <= 0) return 0;
          return Math.round(value / 60000000000);
        }
        if (typeof value === "string") {
          if (/^\d+$/.test(value)) {
            const num = Number(value);
            return Number.isFinite(num) ? Math.round(num / 60000000000) : 0;
          }
          let hours = 0;
          let minutes = 0;
          const hMatch = value.match(/(\\d+)h/);
          const mMatch = value.match(/(\\d+)m/);
          if (hMatch) hours = parseInt(hMatch[1], 10);
          if (mMatch) minutes = parseInt(mMatch[1], 10);
          return hours * 60 + minutes;
        }
        return 0;
      }

      function setConfigModalMode(isEdit) {
        if (configModalTitle) {
          configModalTitle.textContent = isEdit ? "编辑订阅" : "添加订阅";
        }
        if (configSubmitButton) {
          configSubmitButton.textContent = isEdit ? "保存修改" : "保存订阅";
        }
      }

      function fillConfigForm(cfg) {
        if (!configForm || !cfg) return;
        configForm.name.value = cfg.name || "";
        configForm.sourceUrl.value = cfg.sourceUrl || "";
        configForm.payload.value = cfg.payload || "";
        const minutes = durationToMinutes(cfg.autoUpdateInterval);
        configForm.autoUpdateInterval.value = Number.isFinite(minutes) ? String(minutes) : "0";
      }

      function openConfigModal(config = null) {
        if (!configModal) return;
        configModal.classList.add("open");
        editingConfig = config || null;
        if (configForm) configForm.reset();
        setConfigModalMode(Boolean(editingConfig));
        if (editingConfig) {
          fillConfigForm(editingConfig);
        }
      }

      function closeConfigModal() {
        if (!configModal) return;
        configModal.classList.remove("open");
        editingConfig = null;
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
          if (editingConfig) {
            fillConfigForm(editingConfig);
            return;
          }
          configForm.reset();
        });
      }

      if (configForm) {
        configForm.addEventListener("submit", async (event) => {
          event.preventDefault();
          const form = event.target;
          const isEdit = Boolean(editingConfig && editingConfig.id);
          const typedPayload = (form.payload.value || "").trim();
          const sourceUrlInput = form.sourceUrl.value.trim();
          const sourceUrl = sourceUrlInput || (isEdit ? (editingConfig?.sourceUrl || "") : "");
          const payload = {
            name: form.name.value.trim() || (isEdit ? (editingConfig?.name || "") : ""),
            format: "subscription",
            sourceUrl,
            payload: typedPayload || (isEdit ? (editingConfig?.payload || "") : ""),
            autoUpdateIntervalMinutes: parseNumber(form.autoUpdateInterval.value),
          };
          if (!payload.name) {
            showStatus("请填写名称", "error", 5000);
            return;
          }
          if (!payload.sourceUrl) {
            showStatus("请填写源/订阅链接", "error", 5000);
            return;
          }
          try {
            if (isEdit) {
              await api.put(`/configs/${editingConfig.id}`, payload);
              showStatus("配置已更新", "success");
            } else {
              await api.post("/configs/import", payload);
              form.reset();
              showStatus("配置添加成功", "success");
            }
            closeConfigModal();
            await Promise.all([loadConfigs(), loadFRouters(), loadNodes()]);
          } catch (err) {
            showStatus(`${isEdit ? "更新" : "添加"}配置失败：${err.message}`, "error", 6000);
          }
        });
      }

      function getSelectedFRouterIds() {
        if (!nodeGrid) return [];
        return Array.from(nodeGrid.querySelectorAll(".node-row[data-id]"))
          .map((card) => card.dataset.id)
          .filter(Boolean);
      }

      async function runBulkSpeedTest({ notify = true } = {}) {
        const ids = getSelectedFRouterIds();
        if (ids.length === 0) {
          if (notify) {
	            showStatus("暂无可测试 FRouter", "error");
          }
          return;
        }
        try {
          await api.post("/frouters/reset-speed", { ids });
          await loadFRouters();
          for (const id of ids) {
            await api.post(`/frouters/${id}/speedtest`);
          }
          if (notify) {
            showStatus("批量测速任务已排队", "info");
          }
          await loadFRouters();
        } catch (err) {
          if (notify) {
            showStatus(`批量测速失败：${err.message}`, "error", 6000);
          } else {
            console.error("自动测速失败:", err);
          }
        }
      }

      document.getElementById("node-bulk-ping").addEventListener("click", async () => {
        const ids = getSelectedFRouterIds();
        if (ids.length === 0) {
	          showStatus("暂无可测试 FRouter", "error");
          return;
        }
        try {
          await api.post("/frouters/bulk/ping", { ids });
          showStatus("批量延迟任务已排队", "info");
          await loadFRouters();
        } catch (err) {
          showStatus(`批量延迟失败：${err.message}`, "error", 6000);
        }
      });

      document.getElementById("node-bulk-speed").addEventListener("click", async () => {
        await runBulkSpeedTest({ notify: true });
      });

      document.getElementById("config-table").addEventListener("click", async (event) => {
        const button = event.target.closest("button[data-action]");
        if (!button) return;
        const tr = button.closest("tr[data-id]");
        if (!tr) return;
        const id = tr.dataset.id;
        const action = button.dataset.action;
        try {
	          if (action === "edit-config") {
	            const cfg = Array.isArray(configsCache) ? configsCache.find((item) => item.id === id) : null;
	            if (!cfg) {
	              showStatus("配置未找到", "error", 4000);
	              return;
	            }
	            openConfigModal(cfg);
	          } else if (action === "pull-nodes") {
	            await api.post(`/configs/${id}/pull-nodes`);
	            showStatus("订阅节点已同步", "success");
	            await Promise.all([loadConfigs(), loadFRouters(), loadNodes()]);
	          } else if (action === "delete-config") {
            if (!confirm("确认删除该配置？")) return;
            await api.delete(`/configs/${id}`);
            showStatus("配置已删除", "success");
            await Promise.all([loadConfigs(), loadFRouters(), loadNodes()]);
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
            updateCoreUI(coreStatus);
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
          updateCoreUI(coreStatus);
        });
      }

      // TUN 设置按钮事件
      const tunSetupBtn = document.getElementById("tun-setup-btn");

      if (tunSetupBtn) {
        tunSetupBtn.addEventListener("click", () => {
          setupTUN();
        });
      }

      renderSystemProxy(systemProxySettings);

      // 设置标签切换
      const settingsNavItems = document.querySelectorAll(".settings-nav-item");
      settingsNavItems.forEach((item) => {
        item.addEventListener("click", () => {
          const tab = item.dataset.settingsTab;

          // 更新导航项状态
          settingsNavItems.forEach((navItem) => navItem.classList.remove("active"));
          item.classList.add("active");

          // 更新内容显示
          document.querySelectorAll(".settings-content").forEach((content) => {
            content.classList.remove("active");
            content.style.display = "none";
          });

          const targetContent = document.getElementById(`settings-${tab}`);
          if (targetContent) {
            targetContent.classList.add("active");
            targetContent.style.display = "block";
          }
        });
      });

      // ===== 动态设置系统初始化 =====
      const settingsManager = new SettingsManager(SETTINGS_SCHEMA);
      const settingsRenderer = new SettingsRenderer(settingsManager);

      // 类别 ID 映射到 DOM 容器 ID
      const categoryToContainerId = {
        general: 'settings-general',
        proxy: 'settings-proxy',
        tun: 'settings-network',
        singbox: 'settings-singbox',
        advanced: 'settings-advanced'
      };

      let themeCatalog = [];

      function getThemesBaseHref() {
        const href = window.location.href;
        const marker = '/themes/';
        const idx = href.lastIndexOf(marker);
        if (idx === -1) return '';
        return href.slice(0, idx + marker.length);
      }

      function getCurrentThemeEntry() {
        const href = window.location.href;
        const marker = '/themes/';
        const idx = href.lastIndexOf(marker);
        if (idx === -1) return '';
        const rel = href.slice(idx + marker.length).split(/[?#]/)[0];
        try {
          return decodeURI(rel);
        } catch {
          return rel;
        }
      }

      function normalizeEntry(entry) {
        return String(entry || '').trim().replace(/^\/+/, '');
      }

      function resolveThemeHref(entry) {
        const base = getThemesBaseHref();
        const rel = normalizeEntry(entry);
        if (!base || !rel) return '';
        try {
          return new URL(rel, base).toString();
        } catch {
          return base + rel;
        }
      }

      function getCurrentThemeIdFromLocation(themes) {
        const entry = normalizeEntry(getCurrentThemeEntry());
        if (!entry) return '';
        const picked = themes.find((t) => t && normalizeEntry(t.entry) === entry);
        return picked && typeof picked.id === 'string' ? picked.id : '';
      }

      async function fetchThemes() {
        try {
          const payload = await api.get('/themes');
          const themes = payload && Array.isArray(payload.themes) ? payload.themes : [];
          return themes.filter((t) => t && typeof t.id === 'string' && t.id && t.hasIndex && typeof t.entry === 'string' && t.entry);
        } catch (err) {
          console.warn('[Theme] 加载主题列表失败:', err.message);
          return [];
        }
      }

      function themeLabel(theme) {
        const id = String(theme && theme.id ? theme.id : '').trim();
        if (!id) return '';

        const packLabel = String(theme && (theme.packName || theme.packId) ? (theme.packName || theme.packId) : '').trim();
        const subId = id.includes('/') ? id.split('/').pop() : id;
        const name = String(theme && theme.name ? theme.name : '').trim();

        if (packLabel) {
          return `${packLabel} / ${name || subId || id}`;
        }

        if (id === 'dark') return '深色主题';
        if (id === 'light') return '浅色主题';
        return name || id;
      }

      function applyThemeOptions(themeSelect, themes) {
        const current = String(themeSelect.value || '').trim();
        themeSelect.innerHTML = '';

        const sorted = [...themes].sort((a, b) => String(a.id).localeCompare(String(b.id)));
        for (const theme of sorted) {
          const id = String(theme.id || '').trim();
          if (!id) continue;
          const label = themeLabel(theme) || id;

          const opt = document.createElement('option');
          opt.value = id;
          opt.textContent = label;
          themeSelect.appendChild(opt);
        }

        if (current) {
          themeSelect.value = current;
        }
      }

      async function downloadThemeZip(exportId) {
        const url = `${api.client.baseURL}/themes/${encodeURIComponent(exportId)}/export`;
        const resp = await fetch(url);
        if (!resp.ok) {
          let error = `HTTP ${resp.status}`;
          try {
            const data = await resp.json();
            if (data && data.error) error = data.error;
          } catch {}
          throw new Error(error);
        }

        const blob = await resp.blob();
        const link = document.createElement('a');
        const objectUrl = URL.createObjectURL(blob);
        link.href = objectUrl;
        link.download = `${exportId}.zip`;
        document.body.appendChild(link);
        link.click();
        link.remove();
        setTimeout(() => URL.revokeObjectURL(objectUrl), 1000);
      }

      async function uploadThemeZip(file) {
        const url = `${api.client.baseURL}/themes/import`;
        const formData = new FormData();
        formData.append('file', file);

        const resp = await fetch(url, { method: 'POST', body: formData });
        if (!resp.ok) {
          let error = `HTTP ${resp.status}`;
          try {
            const data = await resp.json();
            if (data && data.error) error = data.error;
          } catch {}
          throw new Error(error);
        }
        return resp.json();
      }

      async function switchTheme(themeId) {
        const next = String(themeId || '').trim();
        if (!next) return;

        let themes = themeCatalog && themeCatalog.length > 0 ? themeCatalog : await fetchThemes();
        themeCatalog = themes;

        const current = getCurrentThemeIdFromLocation(themes);
        if (current && next === current) return;

        settingsManager.set('theme', next);
        const saved = await settingsManager.saveToAPI(api.client.baseURL);
        if (!saved.success) {
          showStatus(`保存主题失败：${saved.error || 'unknown error'}`, 'error', 6000);
          return;
        }

        let picked = themes.find((t) => t && t.id === next);
        if (!picked) {
          themes = await fetchThemes();
          themeCatalog = themes;
          picked = themes.find((t) => t && t.id === next);
        }
        const href = picked && picked.entry ? resolveThemeHref(picked.entry) : '';
        if (!href) {
          showStatus(`无法解析主题入口：${next}`, 'error', 6000);
          return;
        }

        showStatus('正在切换主题...', 'info', 1200);
        window.location.href = href;
      }

      function ensureThemeActions(themeSelect) {
        const label = themeSelect.closest('label');
        if (!label) return;

        if (document.getElementById('theme-actions')) {
          return;
        }

        const actions = document.createElement('div');
        actions.id = 'theme-actions';
        actions.style.cssText = 'grid-column:1/-1; display:flex; gap:10px; align-items:center; margin-top:8px; flex-wrap:wrap;';
        actions.innerHTML = `\n          <button type=\"button\" id=\"theme-import-btn\">导入主题(.zip)</button>\n          <button type=\"button\" id=\"theme-export-btn\">导出当前主题(.zip)</button>\n          <span style=\"font-size:12px; color:var(--text-tertiary);\">仅导入你信任的主题包（包含可执行代码）</span>\n        `;

        label.insertAdjacentElement('afterend', actions);

        const importBtn = document.getElementById('theme-import-btn');
        const exportBtn = document.getElementById('theme-export-btn');

        if (importBtn) {
          importBtn.addEventListener('click', () => {
            const input = document.createElement('input');
            input.type = 'file';
            input.accept = '.zip';
            input.onchange = async (e) => {
              const file = e.target.files && e.target.files[0];
              if (!file) return;
              try {
                showStatus('正在导入主题...', 'info', 2000);
                const result = await uploadThemeZip(file);
                const themeId = result && result.themeId ? String(result.themeId) : '';
                showStatus(`主题已导入：${themeId || 'unknown'}`, 'success', 3000);
                // 导入后刷新列表并可直接切换到新主题
                await setupThemeManager();
                if (themeId) {
                  const themeSelect = document.querySelector('[data-key="theme"]');
                  if (themeSelect) themeSelect.value = themeId;
                }
              } catch (err) {
                showStatus(`导入主题失败：${err.message}`, 'error', 6000);
              }
            };
            input.click();
          });
        }

        if (exportBtn) {
          exportBtn.addEventListener('click', async () => {
            try {
              let themes = themeCatalog && themeCatalog.length > 0 ? themeCatalog : await fetchThemes();
              themeCatalog = themes;

              const entry = normalizeEntry(getCurrentThemeEntry());
              let currentTheme = themes.find((t) => t && normalizeEntry(t.entry) === entry);
              if (!currentTheme) {
                const selected = String(themeSelect.value || '').trim();
                currentTheme = themes.find((t) => t && t.id === selected);
              }

              const exportId = currentTheme && currentTheme.packId ? String(currentTheme.packId) : (currentTheme && currentTheme.id ? String(currentTheme.id) : '');
              if (!exportId) {
                showStatus('无法识别当前主题', 'error', 6000);
                return;
              }

              showStatus('正在导出主题...', 'info', 2000);
              await downloadThemeZip(exportId);
              showStatus('主题已导出', 'success', 2000);
            } catch (err) {
              showStatus(`导出主题失败：${err.message}`, 'error', 6000);
            }
          });
        }
      }

      async function setupThemeManager() {
        const themeSelect = document.querySelector('[data-key="theme"]');
        if (!themeSelect) return;

        if (!themeSelect.dataset.themeManagerBound) {
          themeSelect.dataset.themeManagerBound = '1';
          themeSelect.addEventListener('change', async (e) => {
            try {
              await switchTheme(e.target.value);
            } catch (err) {
              showStatus(`切换主题失败：${err.message}`, 'error', 6000);
            }
          });
        }

        ensureThemeActions(themeSelect);

        const themes = await fetchThemes();
        themeCatalog = themes;
        if (themes.length > 0) {
          applyThemeOptions(themeSelect, themes);
        }

        const current = getCurrentThemeIdFromLocation(themes);
        if (current) {
          themeSelect.value = current;
        }
      }

      async function maybeRedirectToSavedTheme() {
        const desired = String(settingsManager.get('theme') || '').trim();
        if (!desired) return;

        const themes = await fetchThemes();
        themeCatalog = themes;
        const current = getCurrentThemeIdFromLocation(themes);
        if (current && desired === current) return;

        const picked = themes.find((t) => t && t.id === desired);
        const href = picked && picked.entry ? resolveThemeHref(picked.entry) : '';
        if (!href) return;

        window.location.href = href;
      }

      // 渲染所有设置类别
      function renderAllSettings() {
        for (const [categoryId, containerId] of Object.entries(categoryToContainerId)) {
          const container = document.getElementById(containerId);
          if (container) {
            container.innerHTML = settingsRenderer.renderCategory(categoryId);
            settingsRenderer.bindEvents(container);
          }
        }

        setupThemeManager();

        // TUN：配置按钮（renderAllSettings 会重建 DOM）
        const tunSetupBtn = document.getElementById("tun-setup-btn");
        if (tunSetupBtn && !tunSetupBtn.dataset.bound) {
          tunSetupBtn.dataset.bound = "1";
          tunSetupBtn.addEventListener("click", () => {
            setupTUN();
          });
        }

        // 重新渲染后刷新一次 TUN UI（避免状态面板显示为默认值）
        if (tunStatusCache) {
          updateTUNUI(tunStatusCache);
        }
      }

      // 初始化渲染
      renderAllSettings();

      // 导入设置按钮
      const settingsImportBtn = document.getElementById('settings-import-btn');
      if (settingsImportBtn) {
        settingsImportBtn.addEventListener('click', () => {
          const input = document.createElement('input');
          input.type = 'file';
          input.accept = '.json';
          input.onchange = async (e) => {
            const file = e.target.files[0];
            if (!file) return;
            try {
              const text = await file.text();
              const result = settingsManager.importJSON(text);
              if (result.success) {
                showStatus(`导入成功：${result.imported} 项设置`, 'success');
                renderAllSettings();
              } else {
                showStatus(`导入失败：${result.error}`, 'error', 5000);
              }
            } catch (err) {
              showStatus(`读取文件失败：${err.message}`, 'error', 5000);
            }
          };
          input.click();
        });
      }

      // 导出设置按钮
      const settingsExportBtn = document.getElementById('settings-export-btn');
      if (settingsExportBtn) {
        settingsExportBtn.addEventListener('click', () => {
          const json = settingsManager.exportJSON();
          const blob = new Blob([json], { type: 'application/json' });
          const url = URL.createObjectURL(blob);
          const a = document.createElement('a');
          a.href = url;
          a.download = `vea-settings-${new Date().toISOString().split('T')[0]}.json`;
          a.click();
          URL.revokeObjectURL(url);
          showStatus('设置已导出', 'success');
        });
      }

      // 恢复默认按钮
      const settingsResetBtn = document.getElementById('settings-reset-btn');
      if (settingsResetBtn) {
        settingsResetBtn.addEventListener('click', () => {
          if (!confirm('确认恢复所有设置为默认值？')) return;
          settingsManager.resetToDefaults();
          renderAllSettings();
          showStatus('已恢复默认设置', 'success');
        });
      }

      // ===== 系统代理端口（ProxyConfig.inboundPort）联动 =====
      // 注意：
      // - sing-box / mihomo(clash): mixed 端口同时提供 HTTP + SOCKS
      let proxyPortApplyTimeout = null;

      async function syncProxyPortFromBackend() {
        try {
          const cfg = await api.get("/proxy/config");
          const port = cfg && typeof cfg.inboundPort === "number" ? cfg.inboundPort : 0;
          if (port > 0) {
            settingsManager.values["proxy.port"] = port; // 直接写入，避免触发监听器
          }
        } catch (err) {
          console.warn("[Settings] 同步代理端口失败:", err.message);
        }
      }

      async function applyProxyPortSetting(port) {
        const normalized = parseInt(String(port), 10);
        if (!Number.isFinite(normalized) || normalized < 1024 || normalized > 65535) {
          showStatus("代理端口无效（范围 1024-65535）", "error", 5000);
          return;
        }

        let currentCfg = null;
        try {
          currentCfg = await api.get("/proxy/config");
        } catch {
          currentCfg = null;
        }

        const inboundMode = currentCfg && currentCfg.inboundMode ? currentCfg.inboundMode : "mixed";

        // TUN 模式下端口不生效，但仍可提前保存到配置中（切回非 TUN 时生效）。
        if (inboundMode === "tun") {
          try {
            await api.put("/proxy/config", { inboundPort: normalized });
            showStatus("已保存代理端口（当前为 TUN 模式，切回非 TUN 后生效）", "info", 4000);
          } catch (err) {
            showStatus(`保存代理端口失败：${err.message}`, "error", 6000);
          }
          return;
        }

        // 非 TUN：强制保持 mixed（系统代理场景需要 HTTP + SOCKS）
        try {
          await api.put("/proxy/config", { inboundMode: "mixed", inboundPort: normalized });
        } catch (err) {
          showStatus(`保存代理端口失败：${err.message}`, "error", 6000);
          return;
        }

        // 如果内核正在运行，需要重启才能让端口变更生效。
        let status = null;
        try {
          status = await api.proxy.status();
          coreStatus = status || coreStatus;
        } catch {
          status = null;
        }

        const running = Boolean(status && status.running);
        if (running) {
          const frouterId = (status && status.frouterId) ? status.frouterId : (getCurrentFRouter()?.id || "");
          if (!frouterId) {
            showStatus("已更新端口，但无法自动重启：请先选择一个 FRouter", "warn", 6000);
            return;
          }

          showStatus("正在应用代理端口（重启内核）...", "info", 2000);
          try {
            await api.post("/proxy/start", { frouterId });
            await new Promise((r) => setTimeout(r, 500));
          } catch (err) {
            showStatus(`重启内核失败：${err.message}`, "error", 6000);
            return;
          }

          // 系统代理已启用时，重启后需要重新应用系统代理（端口可能发生变化）。
          if (systemProxySettings && systemProxySettings.enabled) {
            try {
              await api.put("/settings/system-proxy", {
                enabled: true,
                ignoreHosts: collectIgnoreHosts(),
              });
            } catch (err) {
              console.warn("[SystemProxy] 重新应用系统代理失败:", err.message);
            }
          }
        }

        await refreshCoreStatus();
        await loadSystemProxySettings();
        await loadIPGeo();

        // 提示端口更新结果
        try {
          const latest = await api.proxy.status();
          if (latest && latest.running) {
            showStatus(`代理端口已更新为 ${normalized}`, "success", 2000);
          } else {
            showStatus(`代理端口已更新为 ${normalized}`, "success", 2000);
          }
        } catch {
          showStatus(`代理端口已更新为 ${normalized}`, "success", 2000);
        }
      }

      // 监听设置变化，保存到后端 API
      let saveTimeout = null;
      for (const category of SETTINGS_SCHEMA.categories) {
        for (const setting of category.settings) {
          settingsManager.on(setting.key, (value, oldValue, key) => {
            // 防抖：300ms 内只保存一次
            if (saveTimeout) clearTimeout(saveTimeout);
            saveTimeout = setTimeout(async () => {
              await settingsManager.saveToAPI(api.client.baseURL);
            }, 300);

            // 特殊联动：系统代理端口 -> 后端 ProxyConfig.inboundPort
            if (key === "proxy.port") {
              if (proxyPortApplyTimeout) clearTimeout(proxyPortApplyTimeout);
              proxyPortApplyTimeout = setTimeout(async () => {
                await applyProxyPortSetting(value);
              }, 300);
            }
          });
        }
      }

      // 从后端 API 加载已保存的设置
      (async function loadSettingsFromAPI() {
        await settingsManager.loadFromAPI(api.client.baseURL);
        await syncProxyPortFromBackend();
        renderAllSettings();
        await maybeRedirectToSavedTheme();
      })();

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
              await refreshCoreStatus();
            } else if (action === "uninstall-component") {
              if (!confirm("确认卸载该组件？这将删除本地安装文件。")) return;
              await api.post(`/components/${id}/uninstall`);
              showStatus("组件已卸载", "success");
              await loadComponents();
              await refreshCoreStatus();
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

      // Initial load
      loadFRouters();
      loadNodes();
      loaders[currentPanel]?.();
      ensureFRoutersPolling();
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
  
