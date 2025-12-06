const { app, BrowserWindow, ipcMain, dialog } = require('electron')
const { spawn } = require('child_process')
const path = require('path')
const http = require('http')

// 禁用沙箱以支持 root 权限运行（TUN 模式需要）
app.commandLine.appendSwitch('no-sandbox')
app.commandLine.appendSwitch('disable-gpu-sandbox')

let veaProcess = null
let mainWindow = null
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

  mainWindow.on('closed', () => {
    mainWindow = null
  })
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
app.on('window-all-closed', async () => {
  // 先通过 API 停止代理
  console.log('Stopping proxy via API...')
  await stopProxyViaAPI()

  // 停止 Vea 服务进程
  if (veaProcess) {
    console.log('Stopping Vea service...')
    veaProcess.kill('SIGTERM')

    // 等待 2 秒后强制杀死
    setTimeout(() => {
      if (veaProcess) {
        console.log('Force killing Vea service...')
        veaProcess.kill('SIGKILL')
      }
    }, 2000)
  }

  // macOS 下保持应用运行
  if (process.platform !== 'darwin') {
    app.quit()
  }
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
