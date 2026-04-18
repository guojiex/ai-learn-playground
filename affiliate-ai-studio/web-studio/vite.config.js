import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "node:path";
const GO_BACKEND = process.env.AFFILIATE_GO_URL ?? "http://localhost:8080";
export default defineConfig({
    plugins: [react()],
    resolve: {
        alias: {
            "@": path.resolve(__dirname, "src"),
        },
    },
    server: {
        port: 5173,
        proxy: {
            "/api": { target: GO_BACKEND, changeOrigin: true },
            "/healthz": { target: GO_BACKEND, changeOrigin: true },
            "/readyz": { target: GO_BACKEND, changeOrigin: true },
            "/learn": { target: GO_BACKEND, changeOrigin: true },
            "/static": { target: GO_BACKEND, changeOrigin: true },
        },
    },
    build: {
        manifest: true,
        outDir: path.resolve(__dirname, "../go/web/static/dist"),
        emptyOutDir: true,
        rollupOptions: {
            input: path.resolve(__dirname, "src/main.tsx"),
        },
    },
    test: {
        environment: "jsdom",
        globals: true,
        setupFiles: "./vitest.setup.ts",
    },
});
