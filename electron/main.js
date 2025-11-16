const { app, BrowserWindow } = require('electron')
const { spawn } = require('child_process')
const path = require('path')
const http = require('http')

let veaProcess = null
let mainWindow = null

/**
 * 检查服务是否已启动
 */
function checkService(callback) {
  const options = {
    hostname: '127.0.0.1',
    port: 8080,
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
 */
function waitForService(maxAttempts = 20, interval = 500) {
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

  veaProcess = spawn(veaBinary, ['--addr', ':8080'], {
    stdio: ['ignore', 'pipe', 'pipe']
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

/**
 * 创建主窗口
 */
function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1280,
    height: 800,
    minWidth: 800,
    minHeight: 600,
    webPreferences: {
      nodeIntegration: false,
      contextIsolation: true,
      sandbox: true
    },
    title: 'Vea Console'
  })

  // 开发模式：加载开发服务器
  // 生产模式：加载打包后的 HTML
  const isDev = !app.isPackaged
  if (isDev) {
    mainWindow.loadFile(path.join(__dirname, 'renderer/index.html'))
    // 可选：打开开发者工具
    // mainWindow.webContents.openDevTools()
  } else {
    mainWindow.loadFile(path.join(__dirname, 'renderer/index.html'))
  }

  mainWindow.on('closed', () => {
    mainWindow = null
  })
}

/**
 * 应用就绪
 */
app.whenReady().then(async () => {
  // 检查服务是否已经在运行
  checkService((isRunning) => {
    if (!isRunning) {
      startVeaService()
    } else {
      console.log('Vea service is already running')
    }
  })

  // 等待服务启动
  try {
    await waitForService()
  } catch (err) {
    console.error('Service startup timeout:', err)
    // 即使超时也继续创建窗口，让用户看到错误信息
  }

  createWindow()

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
  // 停止 Vea 服务
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
app.on('before-quit', () => {
  if (veaProcess) {
    veaProcess.kill('SIGTERM')
  }
})
