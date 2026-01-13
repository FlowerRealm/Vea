const path = require('path')
const fs = require('fs')
const crypto = require('crypto')

const BUILTIN_THEMES = ['dark', 'light']
const MARKER_NAME = '.vea-bundled-theme.json'
const INJECTED_SHARED_DIR_NAME = '_shared'

async function pathExists(pathname) {
  try {
    await fs.promises.access(pathname)
    return true
  } catch {
    return false
  }
}

async function copyDirRecursive(srcDir, destDir) {
  await fs.promises.mkdir(destDir, { recursive: true })

  const entries = await fs.promises.readdir(srcDir, { withFileTypes: true })
  for (const ent of entries) {
    const srcPath = path.join(srcDir, ent.name)
    const destPath = path.join(destDir, ent.name)

    if (ent.isDirectory()) {
      await copyDirRecursive(srcPath, destPath)
      continue
    }

    if (ent.isFile()) {
      // fs.promises.cp 在 asar 源目录下会触发 ENOTDIR，这里改用 readFile/writeFile 兼容 asar。
      const data = await fs.promises.readFile(srcPath)
      await fs.promises.writeFile(destPath, data)
      continue
    }

    if (ent.isSymbolicLink()) {
      throw new Error(`refusing to copy symlink: ${srcPath}`)
    }
  }
}

async function computeDirHash(rootDir, { ignoreNames = [] } = {}) {
  const hash = crypto.createHash('sha256')
  const ignores = new Set([MARKER_NAME, ...ignoreNames])

  const walk = async (dir) => {
    const entries = (await fs.promises.readdir(dir, { withFileTypes: true }))
      .sort((a, b) => a.name.localeCompare(b.name))

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

async function readMarker(dir) {
  const markerPath = path.join(dir, MARKER_NAME)
  try {
    const raw = await fs.promises.readFile(markerPath, 'utf8')
    const data = JSON.parse(raw)
    return data && typeof data === 'object' ? data : null
  } catch {
    return null
  }
}

async function writeMarker(dir, bundledHash) {
  const markerPath = path.join(dir, MARKER_NAME)
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

async function syncThemeSharedModule(bundledRoot, themeDir) {
  const srcDir = path.join(bundledRoot, INJECTED_SHARED_DIR_NAME)
  const srcEntry = path.join(srcDir, 'js', 'app.js')
  if (!await pathExists(srcEntry)) {
    console.warn(`[Theme] bundled shared module is missing: ${srcEntry}`)
    return
  }

  const destDir = path.join(themeDir, INJECTED_SHARED_DIR_NAME)

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
    await copyDirRecursive(srcDir, destDir)
    if (bundledHash) {
      await writeMarker(destDir, bundledHash)
    }
  } catch (e) {
    console.error(`[Theme] copy theme shared dir failed: ${e.message}`)
  }
}

async function syncBundledTheme({ themeId, themesRoot, bundledRoot, backupRoot }) {
  const destDir = path.join(themesRoot, themeId)
  const destIndex = path.join(destDir, 'index.html')

  const srcDir = path.join(bundledRoot, themeId)
  const srcIndex = path.join(srcDir, 'index.html')
  if (!await pathExists(srcIndex)) {
    console.warn(`[Theme] bundled theme is missing: ${srcIndex}`)
    return
  }

  let bundledHash = ''
  try {
    // INJECTED_SHARED_DIR_NAME 是运行期注入目录：不参与主题“用户修改判定”。
    bundledHash = await computeDirHash(srcDir, { ignoreNames: [INJECTED_SHARED_DIR_NAME] })
  } catch (e) {
    console.warn(`[Theme] compute bundled theme hash failed (${themeId}): ${e.message}`)
  }

  if (!bundledHash && await pathExists(destIndex)) {
    await syncThemeSharedModule(bundledRoot, destDir)
    return
  }

  let shouldInstall = true

  if (await pathExists(destIndex) && bundledHash) {
    try {
      const marker = await readMarker(destDir)
      const currentHash = await computeDirHash(destDir, { ignoreNames: [INJECTED_SHARED_DIR_NAME] })

      if (marker && marker.bundledHash) {
        if (currentHash !== marker.bundledHash) {
          // 用户已修改过内置主题：不覆盖，尊重用户修改。
          await syncThemeSharedModule(bundledRoot, destDir)
          return
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
        const backupDir = path.join(backupRoot, themeId)
        try {
          await fs.promises.rm(backupDir, { recursive: true, force: true })
        } catch (e) {
          console.warn(`[Theme] remove bundled theme backup failed: ${e.message}`)
        }
        try {
          await copyDirRecursive(destDir, backupDir)
          console.warn(`[Theme] backed up existing bundled theme (${themeId}) to ${backupDir}`)
        } catch (e) {
          console.warn(`[Theme] backup bundled theme failed (${themeId}): ${e.message}`)
        }
      }
    } catch (e) {
      console.warn(`[Theme] bundled theme sync check failed (${themeId}): ${e.message}`)
    }
  }

  if (shouldInstall) {
    try {
      await fs.promises.rm(destDir, { recursive: true, force: true })
    } catch (e) {
      console.warn(`[Theme] remove existing theme dir failed: ${e.message}`)
    }

    try {
      await copyDirRecursive(srcDir, destDir)
      if (bundledHash) {
        await writeMarker(destDir, bundledHash)
      }
      console.log(`[Theme] installed bundled theme: ${themeId}`)
    } catch (e) {
      console.error(`[Theme] copy bundled theme failed (${themeId}): ${e.message}`)
      return
    }
  }

  await syncThemeSharedModule(bundledRoot, destDir)
}

/**
 * 将内置主题复制/同步到 userData/themes，并处理旧版本升级与用户手动修改的场景。
 */
async function ensureBundledThemes(userDataDir) {
  const themesRoot = path.join(userDataDir, 'themes')
  const bundledRoot = path.join(__dirname, 'theme')
  const backupRoot = path.join(themesRoot, '.vea-bundled-theme-backup')

  await fs.promises.mkdir(themesRoot, { recursive: true })

  for (const themeId of BUILTIN_THEMES) {
    await syncBundledTheme({ themeId, themesRoot, bundledRoot, backupRoot })
  }
}

module.exports = {
  ensureBundledThemes,
}
