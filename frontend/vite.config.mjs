import { defineConfig } from "vite";

export default defineConfig({
  build: {
    emptyOutDir: false,
    rollupOptions: {
      output: {
        entryFileNames: "assets/index.js",
        chunkFileNames: "assets/[name].js",
        assetFileNames: (assetInfo) => {
          const name = String(assetInfo?.name || "");
          if (name.endsWith(".css")) {
            return "assets/index.css";
          }
          return "assets/[name][extname]";
        },
      },
    },
  },
});
