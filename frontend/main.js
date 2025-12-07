const { app, BrowserWindow, ipcMain, dialog, Tray, Menu, nativeImage } = require('electron')
const { spawn } = require('child_process')
const path = require('path')
const http = require('http')

// 禁用沙箱以支持 root 权限运行（TUN 模式需要）
app.commandLine.appendSwitch('no-sandbox')
app.commandLine.appendSwitch('disable-gpu-sandbox')

let veaProcess = null
let mainWindow = null
let tray = null
let isQuitting = false  // 防止退出时的无限循环

/**
 * 检查服务是否已启动
 */
function checkService(callback) {
  const options = {
    hostname: '127.0.0.1',
    port: 18080,
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

/**
 * 等待服务启动
 * pkexec 需要用户输入密码，所以等待时间要足够长
 */
function waitForService(maxAttempts = 60, interval = 500) {
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
  const veaBinary = isDev
    ? path.join(__dirname, '../vea')
    : path.join(process.resourcesPath, 'vea')

  console.log(`Starting Vea service from: ${veaBinary}`)

  const args = ['--addr', ':18080']
  if (isDev) {
    args.push('--dev')
  }

  // TUN 模式需要管理员/root 权限，启动时就使用 pkexec 提权
  const platform = process.platform
  let command, spawnArgs, spawnOptions

  if (platform === 'linux') {
    // Linux: 使用 pkexec 启动（需要密码）
    console.log('Linux: Starting Vea service with pkexec')
    command = 'pkexec'
    spawnArgs = ['env', 'DISPLAY=' + (process.env.DISPLAY || ':0'), veaBinary, ...args]
    spawnOptions = {
      stdio: ['ignore', 'pipe', 'pipe'],
      env: process.env
    }
  } else if (platform === 'darwin') {
    // macOS: 检查是否已经是 root，否则提示用户使用 sudo 启动
    const isRoot = process.getuid && process.getuid() === 0
    if (isRoot) {
      console.log('macOS: Running as root')
      command = veaBinary
      spawnArgs = args
      spawnOptions = {
        stdio: ['ignore', 'pipe', 'pipe']
      }
    } else {
      console.log('macOS: Starting Vea service (may require sudo)')
      command = veaBinary
      spawnArgs = args
      spawnOptions = {
        stdio: ['ignore', 'pipe', 'pipe']
      }
    }
  } else if (platform === 'win32') {
    // Windows: 直接运行（应该已经以管理员身份启动）
    console.log('Windows: Starting Vea service (expecting administrator privileges)')
    command = veaBinary
    spawnArgs = args
    spawnOptions = {
      stdio: ['ignore', 'pipe', 'pipe']
    }
  } else {
    console.error(`Unsupported platform: ${platform}`)
    return
  }

  veaProcess = spawn(command, spawnArgs, spawnOptions)

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

  // 直接加载默认主题（dark.html）
  // 主题切换功能在应用内通过重新加载 HTML 文件实现
  mainWindow.loadFile(path.join(__dirname, 'theme/dark.html'))

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

/**
 * 获取代理状态
 */
function getProxyStatus() {
  return new Promise((resolve) => {
    const options = {
      hostname: '127.0.0.1',
      port: 18080,
      path: '/proxy/status',
      method: 'GET',
      timeout: 2000
    }

    const req = http.request(options, (res) => {
      let data = ''
      res.on('data', chunk => data += chunk)
      res.on('end', () => {
        try {
          const status = JSON.parse(data)
          resolve(status)
        } catch {
          resolve({ running: false })
        }
      })
    })

    req.on('error', () => resolve({ running: false }))
    req.on('timeout', () => {
      req.destroy()
      resolve({ running: false })
    })

    req.end()
  })
}

/**
 * 启动代理服务（通过 API）
 * 与主页启动代理按钮逻辑一致：启动 Xray 核心 + 启用系统代理
 */
function startProxyViaAPI() {
  return new Promise((resolve) => {
    // 第一步：启动 Xray 核心
    const startOptions = {
      hostname: '127.0.0.1',
      port: 18080,
      path: '/xray/start',
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      timeout: 5000
    }

    const startReq = http.request(startOptions, (res) => {
      let data = ''
      res.on('data', chunk => data += chunk)
      res.on('end', () => {
        if (res.statusCode >= 200 && res.statusCode < 300) {
          console.log('Xray core started')
          // 等待 500ms 后启用系统代理
          setTimeout(() => {
            enableSystemProxy().then(resolve)
          }, 500)
        } else {
          console.error('Failed to start Xray:', data)
          resolve(false)
        }
      })
    })

    startReq.on('error', (err) => {
      console.error('Start proxy error:', err)
      resolve(false)
    })
    startReq.on('timeout', () => {
      startReq.destroy()
      resolve(false)
    })

    // 发送空 body（使用默认节点）
    startReq.write('{}')
    startReq.end()
  })
}

/**
 * 启用系统代理
 */
function enableSystemProxy() {
  return new Promise((resolve) => {
    const options = {
      hostname: '127.0.0.1',
      port: 18080,
      path: '/settings/system-proxy',
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      timeout: 3000
    }

    const req = http.request(options, (res) => {
      let data = ''
      res.on('data', chunk => data += chunk)
      res.on('end', () => {
        if (res.statusCode >= 200 && res.statusCode < 300) {
          console.log('System proxy enabled')
          resolve(true)
        } else {
          console.error('Failed to enable system proxy:', data)
          resolve(false)
        }
      })
    })

    req.on('error', (err) => {
      console.error('Enable system proxy error:', err)
      resolve(false)
    })
    req.on('timeout', () => {
      req.destroy()
      resolve(false)
    })

    req.write(JSON.stringify({
      enabled: true,
      ignoreHosts: ['localhost', '127.0.0.1', '::1', '*.local']
    }))
    req.end()
  })
}

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
  const fs = require('fs')
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
  const status = await getProxyStatus()
  const isRunning = Boolean(status.running)
  const statusText = isRunning ? '代理运行中' : '代理已停止'
  const statusIcon = isRunning ? '🟢' : '⚪'

  // 更新托盘图标
  const iconPath = getTrayIconPath(isRunning)
  const icon = nativeImage.createFromPath(iconPath)
  tray.setImage(icon)

  // 更新提示文字
  tray.setToolTip(isRunning ? 'Vea - 代理运行中' : 'Vea - 代理已停止')

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
    {
      label: isRunning ? '停止代理' : '启动代理',
      click: async () => {
        if (isRunning) {
          await stopProxyViaAPI()
        } else {
          // 启动代理：启动 Xray 核心 + 启用系统代理（与主页按钮逻辑一致）
          await startProxyViaAPI()
        }
        // 延迟更新菜单状态
        setTimeout(updateTrayMenu, 500)
      }
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
 * 应用就绪
 */
app.whenReady().then(async () => {
  // 总是启动服务（确保使用最新的二进制文件和权限配置）
  // 如果服务已在运行，startVeaService 会检测到端口占用并跳过
  startVeaService()

  // 等待服务启动（最长 30 秒，给用户足够时间输入密码）
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
      '3. 端口 18080 被占用\n\n' +
      '请检查后重试。'
    )
    app.quit()
    return
  }

  createWindow()
  createTray()

  // 定期更新托盘菜单状态
  setInterval(updateTrayMenu, 5000)

  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createWindow()
    }
  })
})

/**
 * 停止代理服务（通过 API）
 */
function stopProxyViaAPI() {
  return new Promise((resolve) => {
    const options = {
      hostname: '127.0.0.1',
      port: 18080,
      path: '/proxy/stop',
      method: 'POST',
      timeout: 3000
    }

    const req = http.request(options, (res) => {
      console.log('Proxy stopped via API')
      resolve(true)
    })

    req.on('error', () => resolve(false))
    req.on('timeout', () => {
      req.destroy()
      resolve(false)
    })

    req.end()
  })
}

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
  // 防止无限循环
  if (isQuitting) return
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

  // 延迟一下让清理完成
  setTimeout(() => {
    app.exit(0)
  }, 500)
})
