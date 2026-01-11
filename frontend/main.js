const { app, BrowserWindow, ipcMain, dialog, Tray, Menu, nativeImage } = require('electron')
const { spawn } = require('child_process')
const path = require('path')
const http = require('http')
const fs = require('fs')
const crypto = require('crypto')

// ============================================================================
// 配置常量
// ============================================================================

/**
 * 后端服务端口
 * 固定端口：避免端口漂移导致前后端对不齐。
 */
const VEA_PORT = 19080

/**
 * 服务启动超时配置
 * 服务启动等待时间上限
 */
const SERVICE_STARTUP_MAX_ATTEMPTS = 60
const SERVICE_STARTUP_INTERVAL = 500  // ms

/**
 * 托盘状态更新间隔（ms）
 */
const TRAY_UPDATE_INTERVAL = 5000

// 内核随应用启动：只启动内核，不自动启用系统代理，避免无意影响全局设置。

// 注：Electron sandbox 在部分发行方式下容易触发兼容性问题，这里保持禁用以减少启动失败。
app.commandLine.appendSwitch('no-sandbox')
app.commandLine.appendSwitch('disable-gpu-sandbox')

let veaProcess = null
let mainWindow = null
let tray = null
let isQuitting = false  // 防止退出时的无限循环
let cleanupInProgress = false
let startupThemeEntryPath = null

// ============================================================================
// 应用自更新（electron-updater）
// ============================================================================

let autoUpdater = null
let updaterState = 'idle' // idle | checking | downloading | downloaded
let updaterInitError = null
let installUpdateOnQuit = false

function getAutoUpdateSupport() {
  if (!app.isPackaged) {
    return { supported: false, message: '开发模式不支持自动更新' }
  }

  const platform = process.platform
  if (platform !== 'win32' && platform !== 'darwin') {
    return { supported: false, message: '当前平台不支持自动更新，请前往 GitHub Releases 下载最新版' }
  }

  return { supported: true, message: '' }
}

function sendAutoUpdateEvent(payload) {
  if (!mainWindow || mainWindow.isDestroyed()) return
  mainWindow.webContents.send('app:update:event', payload)
}

function initAutoUpdater() {
  if (autoUpdater || updaterInitError) return

  const support = getAutoUpdateSupport()
  if (!support.supported) return

  try {
    autoUpdater = require('electron-updater').autoUpdater
  } catch (err) {
    updaterInitError = err
    console.error('[Updater] Failed to load electron-updater:', err && err.message ? err.message : err)
    return
  }

  autoUpdater.allowPrerelease = false
  autoUpdater.autoDownload = true

  autoUpdater.on('checking-for-update', () => {
    updaterState = 'checking'
    sendAutoUpdateEvent({ type: 'checking-for-update', message: '正在检查更新...' })
  })

  autoUpdater.on('update-available', (info) => {
    updaterState = 'downloading'
    sendAutoUpdateEvent({
      type: 'update-available',
      info,
      message: info && info.version ? `发现新版本 ${info.version}，正在下载...` : '发现新版本，正在下载...',
    })
  })

  autoUpdater.on('update-not-available', (info) => {
    updaterState = 'idle'
    sendAutoUpdateEvent({ type: 'update-not-available', info, message: '已是最新版本' })
  })

  autoUpdater.on('download-progress', (progress) => {
    sendAutoUpdateEvent({ type: 'download-progress', progress })
  })

  autoUpdater.on('update-downloaded', (info) => {
    updaterState = 'downloaded'
    sendAutoUpdateEvent({
      type: 'update-downloaded',
      info,
      message: '更新已下载，将自动重启安装...',
    })

    setTimeout(() => {
      installUpdateOnQuit = true
      isQuitting = true
      app.quit()
    }, 1500)
  })

  autoUpdater.on('error', (err) => {
    updaterState = 'idle'
    sendAutoUpdateEvent({
      type: 'error',
      message: err && err.message ? err.message : String(err),
    })
  })
}

// ============================================================================
// 通用 HTTP 请求工具函数
// ============================================================================

/**
 * 发送 HTTP 请求到后端 API
 * @param {Object} options - 请求选项
 * @param {string} options.path - API 路径
 * @param {string} [options.method='GET'] - HTTP 方法
 * @param {Object} [options.body] - 请求体（会自动 JSON 序列化）
 * @param {number} [options.timeout=3000] - 超时时间（ms）
 * @returns {Promise<{success: boolean, data?: any, error?: string}>}
 */
function apiRequest({ path, method = 'GET', body = null, timeout = 3000 }) {
  return new Promise((resolve) => {
    const options = {
      hostname: '127.0.0.1',
      port: VEA_PORT,
      path,
      method,
      timeout,
      headers: body ? { 'Content-Type': 'application/json' } : {}
    }

    const req = http.request(options, (res) => {
      let data = ''
      res.on('data', chunk => data += chunk)
      res.on('end', () => {
        const success = res.statusCode >= 200 && res.statusCode < 300
        try {
          const parsed = data ? JSON.parse(data) : null
          resolve({ success, data: parsed, statusCode: res.statusCode })
        } catch {
          resolve({ success, data, statusCode: res.statusCode })
        }
      })
    })

    req.on('error', (err) => {
      console.error(`[API] ${method} ${path} error:`, err.message)
      resolve({ success: false, error: err.message })
    })

    req.on('timeout', () => {
      req.destroy()
      console.error(`[API] ${method} ${path} timeout`)
      resolve({ success: false, error: 'timeout' })
    })

    if (body) {
      req.write(JSON.stringify(body))
    }
    req.end()
  })
}

/**
 * 简单的健康检查请求
 * @param {Function} callback - 回调函数，参数为是否健康
 */
function checkService(callback) {
  const options = {
    hostname: '127.0.0.1',
    port: VEA_PORT,
    path: '/health',
    method: 'GET',
    timeout: 1000
  }

  const req = http.request(options, (res) => {
    callback(res.statusCode === 200)
  })

  req.on('error', () => callback(false))
  req.on('timeout', () => {
    req.destroy()
    callback(false)
  })

  req.end()
}

// ============================================================================
// 服务管理
// ============================================================================

function resolveVeaBinaryPath(isDev) {
  const baseDir = isDev ? path.join(__dirname, '..') : process.resourcesPath
  const candidates = process.platform === 'win32'
    ? ['vea.exe', 'vea']
    : ['vea']

  for (const name of candidates) {
    const candidate = path.join(baseDir, name)
    if (fs.existsSync(candidate)) {
      return candidate
    }
  }

  // Keep a deterministic path for error messages even when missing.
  return path.join(baseDir, candidates[0])
}

/**
 * 等待服务启动
 */
function waitForService(maxAttempts = SERVICE_STARTUP_MAX_ATTEMPTS, interval = SERVICE_STARTUP_INTERVAL) {
  return new Promise((resolve, reject) => {
    let attempts = 0
    const check = () => {
      checkService((isReady) => {
        if (isReady) {
          console.log('Vea service is ready')
          resolve()
        } else if (attempts < maxAttempts) {
          attempts++
          setTimeout(check, interval)
        } else {
          reject(new Error('Vea service failed to start within timeout'))
        }
      })
    }
    check()
  })
}

/**
 * 启动 Vea 服务
 */
function startVeaService() {
  // 开发模式：使用项目根目录的二进制
  // 生产模式：使用打包后的 resources 目录
  const isDev = !app.isPackaged
  const veaBinary = resolveVeaBinaryPath(isDev)

  console.log(`Starting Vea service from: ${veaBinary}`)
  if (!fs.existsSync(veaBinary)) {
    console.error(`Vea binary not found: ${veaBinary}`)
  }

  // 确保 vea 有执行权限（AppImage 打包后可能丢失）
  try {
    fs.chmodSync(veaBinary, 0o755)
  } catch (e) {
    console.log(`chmod failed (may be read-only): ${e.message}`)
  }

  // 确定数据目录（开发/生产统一使用 userData，避免写入仓库/安装目录）
  const userDataDir = app.getPath('userData')
  const dataDir = path.join(userDataDir, 'data')
  const statePath = path.join(dataDir, 'state.json')

  // artifacts 必须是可写目录：用于组件/Geo/rule-set/运行期配置（不要写进安装目录或 resources 目录）。
  const artifactsDir = path.join(userDataDir, 'artifacts')

  const args = ['--addr', `:${VEA_PORT}`, '--state', statePath]
  if (isDev) {
    args.push('--dev')
  }
  console.log(`Vea state file: ${statePath}`)
  console.log(`Vea artifacts dir: ${artifactsDir}`)

  // 仅在“配置 TUN / Setup TUN”时触发提权（由后端 /tun/setup 内部处理）。
  // 启动服务本身必须保持为普通用户态，避免每次打开应用都弹出密码框。
  veaProcess = spawn(veaBinary, args, {
    stdio: ['ignore', 'pipe', 'pipe'],
    env: {
      ...process.env,
      VEA_USER_DATA_DIR: userDataDir,
    },
  })

  veaProcess.stdout.on('data', (data) => {
    console.log(`[Vea] ${data.toString().trim()}`)
  })

  veaProcess.stderr.on('data', (data) => {
    console.error(`[Vea Error] ${data.toString().trim()}`)
  })

  veaProcess.on('error', (err) => {
    console.error('Failed to start Vea service:', err)
  })

  veaProcess.on('exit', (code, signal) => {
    console.log(`Vea service exited with code ${code} and signal ${signal}`)
    veaProcess = null
  })
}

// ============================================================================
// 窗口管理
// ============================================================================

/**
 * 创建主窗口
 */
function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1280,
    height: 800,
    minWidth: 800,
    minHeight: 600,
    frame: false,
    webPreferences: {
      nodeIntegration: false,
      contextIsolation: true,
      sandbox: false,  // 禁用沙箱以支持 root 权限运行
      preload: path.join(__dirname, 'preload.js')
    },
    title: 'Vea Console'
  })

  // 主题统一从 userData/themes/<id>/index.html 加载
  // 主题切换功能在应用内通过重新加载 HTML 文件实现
  const fallback = path.join(__dirname, 'theme', 'dark', 'index.html')
  const entry = startupThemeEntryPath && fs.existsSync(startupThemeEntryPath) ? startupThemeEntryPath : fallback
  mainWindow.loadFile(entry)

  // F12 打开开发者工具
  mainWindow.webContents.on('before-input-event', (event, input) => {
    if (input.key === 'F12') {
      mainWindow.webContents.toggleDevTools()
    }
  })

  // 关闭窗口时隐藏到托盘，而不是退出
  mainWindow.on('close', (event) => {
    if (!isQuitting) {
      event.preventDefault()
      mainWindow.hide()
    }
  })

  mainWindow.on('closed', () => {
    mainWindow = null
  })
}

// ============================================================================
// 主题目录初始化与入口解析
// ============================================================================

async function ensureBundledThemes(userDataDir) {
  const themesRoot = path.join(userDataDir, 'themes')

  const bundledRoot = path.join(__dirname, 'theme')
  const builtinThemes = ['dark', 'light']
  const markerName = '.vea-bundled-theme.json'
  const backupRoot = path.join(themesRoot, '.vea-bundled-theme-backup')
  const injectedSharedDirName = '_shared'

  const pathExists = async (pathname) => {
    try {
      await fs.promises.access(pathname)
      return true
    } catch {
      return false
    }
  }

  const computeDirHash = async (rootDir, { ignoreNames = [] } = {}) => {
    const hash = crypto.createHash('sha256')
    const ignores = new Set([markerName, ...ignoreNames])

    const walk = async (dir) => {
      const entries = (await fs.promises.readdir(dir, { withFileTypes: true })).sort((a, b) => a.name.localeCompare(b.name))
      for (const ent of entries) {
        if (ignores.has(ent.name)) {
          continue
        }
        const full = path.join(dir, ent.name)
        const rel = path.relative(rootDir, full).split(path.sep).join('/')
        if (ent.isDirectory()) {
          hash.update(`dir:${rel}\n`)
          await walk(full)
          continue
        }
        if (ent.isFile()) {
          hash.update(`file:${rel}\n`)
          hash.update(await fs.promises.readFile(full))
          hash.update('\n')
          continue
        }
      }
    }

    await walk(rootDir)
    return hash.digest('hex')
  }

  const readMarker = async (dir) => {
    const markerPath = path.join(dir, markerName)
    try {
      const raw = await fs.promises.readFile(markerPath, 'utf8')
      const data = JSON.parse(raw)
      return data && typeof data === 'object' ? data : null
    } catch {
      return null
    }
  }

  const writeMarker = async (dir, bundledHash) => {
    const markerPath = path.join(dir, markerName)
    const payload = {
      bundledHash,
      installedAt: new Date().toISOString()
    }
    try {
      await fs.promises.writeFile(markerPath, JSON.stringify(payload, null, 2), 'utf8')
    } catch (e) {
      console.warn(`[Theme] write bundled theme marker failed: ${e.message}`)
    }
  }

  const syncThemeSharedModule = async (themeDir) => {
    const srcDir = path.join(bundledRoot, injectedSharedDirName)
    const srcEntry = path.join(srcDir, 'js', 'app.js')
    if (!await pathExists(srcEntry)) {
      console.warn(`[Theme] bundled shared module is missing: ${srcEntry}`)
      return
    }

    const destDir = path.join(themeDir, injectedSharedDirName)

    let bundledHash = ''
    try {
      bundledHash = await computeDirHash(srcDir)
    } catch (e) {
      console.warn(`[Theme] compute bundled shared hash failed: ${e.message}`)
    }

    if (!bundledHash && await pathExists(destDir)) {
      return
    }

    if (await pathExists(destDir) && bundledHash) {
      try {
        const marker = await readMarker(destDir)
        const currentHash = await computeDirHash(destDir)
        if (marker && marker.bundledHash && marker.bundledHash === bundledHash && currentHash === bundledHash) {
          return
        }
      } catch (e) {
        console.warn(`[Theme] bundled shared sync check failed: ${e.message}`)
      }
    }

    try {
      await fs.promises.rm(destDir, { recursive: true, force: true })
    } catch (e) {
      console.warn(`[Theme] remove theme shared dir failed: ${e.message}`)
    }

    try {
      await fs.promises.cp(srcDir, destDir, { recursive: true })
      if (bundledHash) {
        await writeMarker(destDir, bundledHash)
      }
    } catch (e) {
      console.error(`[Theme] copy theme shared dir failed: ${e.message}`)
    }
  }

  await fs.promises.mkdir(themesRoot, { recursive: true })

  for (const id of builtinThemes) {
    const destDir = path.join(themesRoot, id)
    const destIndex = path.join(destDir, 'index.html')

    const srcDir = path.join(bundledRoot, id)
    const srcIndex = path.join(srcDir, 'index.html')
    if (!await pathExists(srcIndex)) {
      console.warn(`[Theme] bundled theme is missing: ${srcIndex}`)
      continue
    }

    let bundledHash = ''
    try {
      // injectedSharedDirName 是运行期注入目录：不参与主题“用户修改判定”。
      bundledHash = await computeDirHash(srcDir, { ignoreNames: [injectedSharedDirName] })
    } catch (e) {
      console.warn(`[Theme] compute bundled theme hash failed (${id}): ${e.message}`)
    }

    if (!bundledHash && await pathExists(destIndex)) {
      await syncThemeSharedModule(destDir)
      continue
    }

    let shouldInstall = true

    if (await pathExists(destIndex) && bundledHash) {
      try {
        const marker = await readMarker(destDir)
        const currentHash = await computeDirHash(destDir, { ignoreNames: [injectedSharedDirName] })

        if (marker && marker.bundledHash) {
          if (currentHash !== marker.bundledHash) {
            // 用户已修改过内置主题：不覆盖，尊重用户修改。
            await syncThemeSharedModule(destDir)
            continue
          }
          if (marker.bundledHash === bundledHash) {
            shouldInstall = false
          }
        } else if (currentHash === bundledHash) {
          // 旧版本没有 marker：当前内容与 bundled 一致，补写 marker 即可。
          await writeMarker(destDir, bundledHash)
          shouldInstall = false
        } else {
          // 旧版本没有 marker 且内容不一致：不确定是否用户改过。
          // 为避免“升级后 UI 不更新”，这里做一次备份再覆盖。
          await fs.promises.mkdir(backupRoot, { recursive: true })
          const backupDir = path.join(backupRoot, id)
          try {
            await fs.promises.rm(backupDir, { recursive: true, force: true })
          } catch (e) {
            console.warn(`[Theme] remove bundled theme backup failed: ${e.message}`)
          }
          try {
            await fs.promises.cp(destDir, backupDir, { recursive: true })
            console.warn(`[Theme] backed up existing bundled theme (${id}) to ${backupDir}`)
          } catch (e) {
            console.warn(`[Theme] backup bundled theme failed (${id}): ${e.message}`)
          }
        }
      } catch (e) {
        console.warn(`[Theme] bundled theme sync check failed (${id}): ${e.message}`)
      }
    }

    if (shouldInstall) {
      try {
        await fs.promises.rm(destDir, { recursive: true, force: true })
      } catch (e) {
        console.warn(`[Theme] remove existing theme dir failed: ${e.message}`)
      }

      try {
        await fs.promises.cp(srcDir, destDir, { recursive: true })
        if (bundledHash) {
          await writeMarker(destDir, bundledHash)
        }
        console.log(`[Theme] installed bundled theme: ${id}`)
      } catch (e) {
        console.error(`[Theme] copy bundled theme failed (${id}): ${e.message}`)
        continue
      }
    }

    await syncThemeSharedModule(destDir)
  }
}

async function loadFrontendThemeSetting() {
  const result = await apiRequest({ path: '/settings/frontend', timeout: 2000 })
  if (!result.success || !result.data) {
    return 'dark'
  }
  const theme = result.data && typeof result.data.theme === 'string' ? result.data.theme.trim() : ''
  return theme || 'dark'
}

async function resolveThemeEntryPath(userDataDir, themeId) {
  const themesRoot = path.join(userDataDir, 'themes')
  const wanted = typeof themeId === 'string' ? themeId.trim() : ''

  const toEntryPath = (entry) => {
    const rel = typeof entry === 'string' ? entry.trim() : ''
    if (!rel) return ''
    if (rel.startsWith('/') || rel.includes('\\') || rel.includes(':')) return ''
    const parts = rel.split('/').filter(Boolean)
    if (parts.some((p) => p === '.' || p === '..')) return ''
    return path.join(themesRoot, ...parts)
  }

  // 优先通过后端 /themes 返回的 entry 解析入口（兼容主题包 manifest 多层目录）。
  if (wanted) {
    try {
      const result = await apiRequest({ path: '/themes', timeout: 2000 })
      const themes = result.success && result.data && Array.isArray(result.data.themes)
        ? result.data.themes
        : []
      const picked = themes.find((t) => t && typeof t.id === 'string' && t.id === wanted)
      const entryPath = picked && picked.entry ? toEntryPath(picked.entry) : ''
      if (entryPath && fs.existsSync(entryPath)) {
        return entryPath
      }
    } catch (e) {
      console.warn('[Theme] resolve theme entry via /themes failed:', e.message)
    }
  }

  // 兼容旧逻辑：themeId 为目录名，入口固定为 index.html。
  if (wanted) {
    const legacy = path.join(themesRoot, wanted, 'index.html')
    if (fs.existsSync(legacy)) {
      return legacy
    }
  }

  const fallback = path.join(themesRoot, 'dark', 'index.html')
  if (fs.existsSync(fallback)) {
    return fallback
  }

  return path.join(__dirname, 'theme', 'dark', 'index.html')
}

/**
 * 显示主窗口
 */
function showMainWindow() {
  if (mainWindow) {
    mainWindow.show()
    mainWindow.focus()
  } else {
    createWindow()
  }
}

// ============================================================================
// 代理控制 API
// ============================================================================

/**
 * 获取代理状态
 */
async function getProxyStatus() {
  const result = await apiRequest({ path: '/proxy/status', timeout: 2000 })
  if (result.success && result.data) {
    return result.data
  }
  return { running: false }
}

/**
 * 启动内核（通过 API）
 * 仅确保内核运行，不修改系统代理开关
 */
async function startKernelViaAPI() {
  const status = await getProxyStatus()
  if (status && (status.running || status.busy)) {
    console.log('Kernel already running')
    return true
  }

  let frouterId = ''

  const configResult = await apiRequest({ path: '/proxy/config', timeout: 2000 })
  if (configResult.success && configResult.data && configResult.data.frouterId) {
    frouterId = configResult.data.frouterId
  }

  if (!frouterId) {
    const froutersResult = await apiRequest({ path: '/frouters', timeout: 5000 })
    const frouters = froutersResult.success && froutersResult.data && Array.isArray(froutersResult.data.frouters)
      ? froutersResult.data.frouters
      : []
    frouterId = frouters.length > 0 && frouters[0] && frouters[0].id ? frouters[0].id : ''
  }

  if (!frouterId) {
    console.warn('Failed to start kernel: no frouter available')
    return false
  }

  const startResult = await apiRequest({
    path: '/proxy/start',
    method: 'POST',
    body: { frouterId },
    timeout: 8000
  })

  if (!startResult.success) {
    console.error('Failed to start kernel:', startResult.error || startResult.data)
    return false
  }

  console.log('Kernel started')
  return true
}

/**
 * 停止代理服务（通过 API）
 */
async function stopProxyViaAPI() {
  const result = await apiRequest({
    path: '/proxy/stop',
    method: 'POST',
    timeout: 3000
  })

  if (result.success) {
    console.log('Proxy stopped via API')
  }
  return result.success
}

// ============================================================================
// 系统托盘
// ============================================================================

/**
 * 获取托盘图标路径（根据代理状态）
 * @param {boolean} isRunning - 代理是否运行中
 */
function getTrayIconPath(isRunning) {
  const platform = process.platform
  const suffix = isRunning ? 'on' : 'off'
  let iconPath

  if (platform === 'darwin') {
    // macOS: 使用 Template 图标（自动适应深色/浅色模式）
    // macOS Template 图标不支持颜色变化，保持原样
    iconPath = path.join(__dirname, 'assets', 'tray-iconTemplate@2x.png')
  } else if (platform === 'win32') {
    // Windows: 使用带状态的 ICO
    iconPath = path.join(__dirname, 'assets', `icon-${suffix}.ico`)
  } else {
    // Linux: 使用 22x22 带状态的 PNG
    iconPath = path.join(__dirname, 'assets', `tray-icon-${suffix}-22.png`)
  }

  // 如果图标文件不存在，使用默认图标
  if (!fs.existsSync(iconPath)) {
    console.warn(`Tray icon not found at ${iconPath}, using fallback`)
    iconPath = path.join(__dirname, 'assets', 'icon.png')
  }

  return iconPath
}

/**
 * 创建系统托盘
 */
function createTray() {
  // 初始使用停止状态图标
  const iconPath = getTrayIconPath(false)
  const icon = nativeImage.createFromPath(iconPath)
  tray = new Tray(icon)

  // 设置托盘提示文字
  tray.setToolTip('Vea Proxy Manager')

  // 更新托盘菜单（会同时更新图标）
  updateTrayMenu()

  // 双击托盘图标显示窗口
  tray.on('double-click', () => {
    showMainWindow()
  })

  // 单击托盘图标（Linux/Windows 显示菜单，macOS 默认行为）
  const platform = process.platform
  if (platform !== 'darwin') {
    tray.on('click', () => {
      showMainWindow()
    })
  }
}

/**
 * 更新托盘菜单和图标
 */
async function updateTrayMenu() {
  if (!tray) return

  const status = await getProxyStatus()
  const isRunning = Boolean(status.running)
  const statusText = isRunning ? '内核运行中' : '内核未运行'
  const statusIcon = isRunning ? '🟢' : '⚪'

  // 更新托盘图标
  const iconPath = getTrayIconPath(isRunning)
  const icon = nativeImage.createFromPath(iconPath)
  tray.setImage(icon)

  // 更新提示文字
  tray.setToolTip(isRunning ? 'Vea - 内核运行中' : 'Vea - 内核未运行')

  const contextMenu = Menu.buildFromTemplate([
    {
      label: `${statusIcon} ${statusText}`,
      enabled: false
    },
    { type: 'separator' },
    {
      label: '显示主窗口',
      click: () => showMainWindow()
    },
    { type: 'separator' },
    {
      label: '退出 Vea',
      click: () => {
        isQuitting = true
        app.quit()
      }
    }
  ])

  tray.setContextMenu(contextMenu)
}

// ============================================================================
// IPC 处理器
// ============================================================================

/**
 * 窗口控制 IPC 处理器
 */
ipcMain.on('window-minimize', () => {
  if (mainWindow) mainWindow.minimize()
})

ipcMain.on('window-maximize', () => {
  if (mainWindow) {
    if (mainWindow.isMaximized()) {
      mainWindow.unmaximize()
    } else {
      mainWindow.maximize()
    }
  }
})

ipcMain.on('window-close', () => {
  if (mainWindow) mainWindow.close()
})

/**
 * 手动检查更新（渲染进程触发）
 */
ipcMain.handle('app:update:check', async () => {
  const support = getAutoUpdateSupport()
  if (!support.supported) {
    return { supported: false, message: support.message }
  }

  initAutoUpdater()
  if (!autoUpdater) {
    return { supported: false, message: '自动更新初始化失败' }
  }

  if (updaterState === 'checking' || updaterState === 'downloading') {
    return { supported: true, started: false, message: '更新任务进行中，请稍候...' }
  }

  updaterState = 'checking'
  try {
    await autoUpdater.checkForUpdates()
    return { supported: true, started: true }
  } catch (err) {
    updaterState = 'idle'
    sendAutoUpdateEvent({
      type: 'error',
      message: err && err.message ? err.message : String(err),
    })
    return { supported: true, started: false, message: err && err.message ? err.message : String(err) }
  }
})

// ============================================================================
// 应用生命周期
// ============================================================================

/**
 * 应用就绪
 */
app.whenReady().then(async () => {
  // 仅在支持的平台 + 打包态初始化（手动检查更新时触发实际动作）。
  initAutoUpdater()

  // 总是启动服务（确保使用最新的二进制文件和权限配置）
  // 如果服务已在运行，startVeaService 会检测到端口占用并跳过
  startVeaService()

  // 等待服务启动
  try {
    await waitForService()
  } catch (err) {
    console.error('Service startup timeout:', err)
    // 显示错误对话框
    dialog.showErrorBox(
      'Vea 启动失败',
      '后端服务未能在规定时间内启动。\n\n' +
      '可能的原因：\n' +
      '1. 用户取消了授权\n' +
      '2. 服务启动超时\n' +
      `3. 端口 ${VEA_PORT} 被占用\n\n` +
      '请检查后重试。'
    )
    app.quit()
    return
  }

  // 内核随应用启动（不自动启用系统代理）
  await startKernelViaAPI()

  // 主题初始化：缺少内置主题时从 app resources 复制到 userData/themes
  const userDataDir = app.getPath('userData')
  await ensureBundledThemes(userDataDir)

  // 启动前读取后端前端设置 theme（默认 dark）
  const themeId = await loadFrontendThemeSetting()
  startupThemeEntryPath = await resolveThemeEntryPath(userDataDir, themeId)

  createWindow()
  createTray()

  // 定期更新托盘菜单状态
  setInterval(updateTrayMenu, TRAY_UPDATE_INTERVAL)

  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createWindow()
    }
  })
})

/**
 * 所有窗口关闭
 */
app.on('window-all-closed', () => {
  // 后台运行模式：窗口关闭时不退出应用，保持托盘图标运行
  // 只有当用户通过托盘菜单选择"退出"时才会真正退出
  if (process.platform === 'darwin') {
    // macOS: 默认行为，保持应用运行
  }
  // Linux/Windows: 由于我们有托盘图标，也保持应用运行
  // 不调用 app.quit()
})

/**
 * 应用退出前清理
 */
app.on('before-quit', async (event) => {
  if (cleanupInProgress) return
  cleanupInProgress = true
  isQuitting = true

  // 阻止立即退出，先清理
  event.preventDefault()

  // 销毁托盘图标
  if (tray) {
    tray.destroy()
    tray = null
  }

  // 先通过 API 停止代理
  await stopProxyViaAPI()

  if (veaProcess) {
    veaProcess.kill('SIGTERM')
  }

  if (installUpdateOnQuit && autoUpdater) {
    installUpdateOnQuit = false
    try {
      autoUpdater.quitAndInstall()
      return
    } catch (err) {
      console.error('[Updater] quitAndInstall failed:', err && err.message ? err.message : err)
    }
  }

  // 延迟一下让清理完成
  setTimeout(() => {
    app.exit(0)
  }, 500)
})
