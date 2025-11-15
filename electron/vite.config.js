import { defineConfig } from 'vite'

export default defineConfig({
  root: './renderer',
  base: './',
  build: {
    outDir: '../dist/renderer',
    emptyOutDir: true
  }
})
