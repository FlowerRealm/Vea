const { contextBridge, ipcRenderer } = require('electron')

// 暴露窗口控制 API 到渲染进程
contextBridge.exposeInMainWorld('electronAPI', {
  minimizeWindow: () => ipcRenderer.send('window-minimize'),
  maximizeWindow: () => ipcRenderer.send('window-maximize'),
  closeWindow: () => ipcRenderer.send('window-close'),
  checkForUpdates: () => ipcRenderer.invoke('app:update:check'),
  onUpdateEvent: (handler) => {
    if (typeof handler !== 'function') return () => {}
    const listener = (_event, payload) => handler(payload)
    ipcRenderer.on('app:update:event', listener)
    return () => ipcRenderer.removeListener('app:update:event', listener)
  },
})
