/// <reference types="vitest/config" />
import {defineConfig} from 'vite'
import {svelte} from '@sveltejs/vite-plugin-svelte'
import {svelteTesting} from '@testing-library/svelte/vite'

// https://vitejs.dev/config/
export default defineConfig({
  // svelteTesting only adjusts config under VITEST: it adds the `browser`
  // resolve condition so component tests load Svelte's client build (mount)
  // instead of its SSR build. It has no effect on dev/build.
  plugins: [svelte(), svelteTesting()],
  test: {
    // Pure-logic helpers (e.g. urlParams) run in Node. Component tests that need
    // a DOM can opt into jsdom per-file via a `// @vitest-environment jsdom` header.
    globals: true,
    environment: 'node',
    include: ['src/**/*.{test,spec}.{ts,js}'],
  },
})
