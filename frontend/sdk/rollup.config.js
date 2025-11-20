import resolve from '@rollup/plugin-node-resolve'
import { readFileSync } from 'fs'

const pkg = JSON.parse(readFileSync('./package.json', 'utf-8'))

const banner = `/**
 * @vea/sdk v${pkg.version}
 * (c) ${new Date().getFullYear()} Vea Project
 * @license MIT
 */`

export default {
  // ESM build (现代浏览器 + Electron + 打包工具)
  input: 'src/vea-sdk.js',
  output: {
    file: 'dist/vea-sdk.esm.js',
    format: 'es',
    banner
  },
  plugins: [resolve()]
}
