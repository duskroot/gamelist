import { defineConfig, loadEnv } from "vite";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  const target = env.VITE_API_PROXY_TARGET || "http://127.0.0.1:8080";

  return {
    server: {
      host: "0.0.0.0",
      port: 5173,
      proxy: {
        "/table": {
          target,
          changeOrigin: true,
        },
      },
    },
    preview: {
      host: "0.0.0.0",
      port: 4173,
      proxy: {
        "/table": {
          target,
          changeOrigin: true,
        },
      },
    },
  };
});
