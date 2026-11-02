/// <reference types="vitest/config" />
import { mkdirSync, writeFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { defineConfig, type Plugin } from 'vite'
import react from '@vitejs/plugin-react'

// Vite empties dist on every build. The tracked dist/.gitkeep keeps
// `//go:embed all:frontend/dist` compiling on a fresh clone (D-03), so put it
// back after each build to keep git status clean.
function restoreGitkeep(): Plugin {
  return {
    name: 'restore-gitkeep',
    apply: 'build',
    closeBundle() {
      const dist = fileURLToPath(new URL('./dist/', import.meta.url))
      mkdirSync(dist, { recursive: true })
      writeFileSync(fileURLToPath(new URL('./dist/.gitkeep', import.meta.url)), '')
    },
  }
}

// index.html carries the production Content-Security-Policy. The dev server
// needs inline scripts (React refresh) and a websocket, so it serves the page
// without that tag; production builds keep it.
function devWithoutCsp(): Plugin {
  return {
    name: 'dev-without-csp',
    apply: 'serve',
    transformIndexHtml(html) {
      return html.replace(/\s*<meta http-equiv="Content-Security-Policy"[^>]*>/, '')
    },
  }
}

export default defineConfig({
  plugins: [react(), restoreGitkeep(), devWithoutCsp()],
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    restoreMocks: true,
  },
})
