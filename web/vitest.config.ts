import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "node:path";

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./vitest.setup.ts"],
    css: true,
    exclude: ["node_modules", ".next", "e2e", "playwright-report"],
    // userEvent-driven tests (typing, debounce timers) have been observed to
    // exceed vitest's 5000ms default under CPU contention (e.g. Docker
    // build/CI runners), causing false failures unrelated to the component
    // code. Raised to give real work headroom without masking genuine hangs.
    testTimeout: 15000,
  },
  resolve: {
    alias: {
      "@": path.resolve(import.meta.dirname, "./src"),
    },
  },
});
