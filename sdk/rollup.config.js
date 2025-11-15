import resolve from '@rollup/plugin-node-resolve'
import terser from '@rollup/plugin-terser'
import { readFileSync } from 'fs'

const pkg = JSON.parse(readFileSync('./package.json', 'utf-8'))

const banner = `/**
 * @vea/sdk v${pkg.version}
 * (c) ${new Date().getFullYear()} Vea Project
 * @license MIT
 */`

export default [
  // UMD build (浏览器 + Node.js) - 完整版（包含工具函数）
  {
    input: 'src/index.js',
    output: {
      file: 'dist/vea-sdk.umd.js',
      format: 'umd',
      name: 'VeaSDK',
      exports: 'named',
      banner
    },
    plugins: [resolve()]
  },

  // UMD minified
  {
    input: 'src/index.js',
    output: {
      file: 'dist/vea-sdk.umd.min.js',
      format: 'umd',
      name: 'VeaSDK',
      exports: 'named',
      banner,
      sourcemap: true
    },
    plugins: [resolve(), terser()]
  },

  // ESM build (现代浏览器 + 打包工具)
  {
    input: 'src/index.js',
    output: {
      file: 'dist/vea-sdk.esm.js',
      format: 'es',
      banner
    },
    plugins: [resolve()]
  },

  // CommonJS build (Node.js)
  {
    input: 'src/index.js',
    output: {
      file: 'dist/vea-sdk.cjs.js',
      format: 'cjs',
      exports: 'named',
      banner
    },
    plugins: [resolve()]
  }
]
