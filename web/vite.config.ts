import { readdirSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    {
      name: "pdfjs-resources",
      generateBundle(_options, bundle) {
        for (const output of Object.values(bundle)) {
          if (
            output.type === "chunk" &&
            output.moduleIds.some((id) => id.includes("/@napi-rs/canvas"))
          )
            this.error(
              "Node-only PDF canvas must not enter the browser bundle",
            );
        }
        this.emitFile({
          type: "asset",
          fileName: "pdfjs/LICENSE",
          source: readFileSync(
            new URL("./node_modules/pdfjs-dist/LICENSE", import.meta.url),
          ),
        });
        for (const directory of ["cmaps", "standard_fonts", "wasm"]) {
          const root = fileURLToPath(
            new URL(`./node_modules/pdfjs-dist/${directory}/`, import.meta.url),
          );
          for (const name of readdirSync(root))
            this.emitFile({
              type: "asset",
              fileName: `pdfjs/${directory}/${name}`,
              source: readFileSync(`${root}/${name}`),
            });
        }
      },
      configureServer(server) {
        server.middlewares.use("/pdfjs", (request, response, next) => {
          const path = request.url?.split("?")[0] || "";
          if (!/^\/(cmaps|standard_fonts|wasm)\/[a-zA-Z0-9_.-]+$/.test(path))
            return next();
          try {
            const data = readFileSync(
              new URL(`./node_modules/pdfjs-dist${path}`, import.meta.url),
            );
            response.setHeader(
              "Content-Type",
              path.endsWith(".wasm")
                ? "application/wasm"
                : "application/octet-stream",
            );
            response.end(data);
          } catch {
            next();
          }
        });
      },
    },
  ],
  build: { outDir: "../internal/web/static/build", emptyOutDir: true },
  test: {
    include: ["src/**/*.test.{ts,tsx}"],
    execArgv: ["--no-experimental-webstorage"],
    environment: "jsdom",
    setupFiles: ["./src/test-setup.ts"],
  },
});
