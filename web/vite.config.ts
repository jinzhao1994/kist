import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: { outDir: "../internal/web/static/build", emptyOutDir: true },
  test: {
    execArgv: ["--no-experimental-webstorage"],
    environment: "jsdom",
    setupFiles: ["./src/test-setup.ts"],
  },
});
