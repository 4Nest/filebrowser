import fs from "node:fs";
import path from "node:path";
import { defineConfig, type Plugin } from "vite";
import vue from "@vitejs/plugin-vue";
import VueI18nPlugin from "@intlify/unplugin-vue-i18n/vite";
import legacy from "@vitejs/plugin-legacy";
import { compression } from "vite-plugin-compression2";

const plugins = [
  vue(),
  VueI18nPlugin({
    include: [path.resolve(__dirname, "./src/i18n/**/*.json")],
  }),
  legacy({
    // defaults already drop IE support
    targets: ["defaults"],
  }),
  compression({ include: /\.js$/, deleteOriginalAssets: false }),
];

const resolve = {
  alias: {
    // vue: "@vue/compat",
    "@/": `${path.resolve(__dirname, "src")}/`,
  },
};

// https://vitejs.dev/config/
export default defineConfig(({ command }) => {
  if (command === "serve") {
    const apiTarget = process.env.FB_API_TARGET || "http://127.0.0.1:8080";
    const wsTarget = apiTarget.replace(/^http/, "ws");
    const devHtmlPlugin: Plugin = {
      name: "fb-dev-html",
      configureServer(server) {
        server.middlewares.use(async (req, res, next) => {
          const url = req.url || "/";
          const pathname = url.split("?")[0];
          const isSpaRoute =
            pathname === "/" ||
            pathname === "/index.html" ||
            pathname === "/login" ||
            pathname.startsWith("/files") ||
            pathname.startsWith("/settings") ||
            pathname.startsWith("/share");
          if (!isSpaRoute) return next();

          const rawHtml = fs.readFileSync(
            path.resolve(__dirname, "./public/index.html"),
            "utf-8"
          );
          const fbConfig = {
            AuthMethod: "json",
            BaseURL: "",
            CSS: false,
            Color: "",
            DisableExternal: false,
            DisableUsedPercentage: false,
            EnableExec: false,
            EnableThumbs: true,
            HideLoginButton: false,
            LoginPage: true,
            Name: "File Browser",
            NoAuth: false,
            ReCaptcha: false,
            ResizePreview: false,
            Signup: false,
            StaticURL: "",
            Theme: "",
            TusSettings: { chunkSize: 10485760, retryCount: 5 },
            Version: "v2.52.0-nest-dev",
          };
          const rendered = rawHtml
            .replace(/\[\{\[\s*if \.ReCaptcha\s*-?\]\}\][\s\S]*?\[\{\[\s*end\s*\]\}\]/g, "")
            .replace(/\[\{\[\s*if \.CSS\s*-?\]\}\][\s\S]*?\[\{\[\s*end\s*\]\}\]/g, "")
            .replace(/\[\{\[\s*if \.Name\s*-?\]\}\][\s\S]*?\[\{\[\s*end\s*\]\}\]/g, "File Browser")
            .replace(/\[\{\[\s*if \.Color\s*-?\]\}\][\s\S]*?\[\{\[\s*end\s*\]\}\]/g, "#2979ff")
            .replace(/\[\{\[\s*\.StaticURL\s*\]\}\]/g, "")
            .replace(/\[\{\[\s*\.Json\s*\]\}\]/g, JSON.stringify(fbConfig));
          const transformed = await server.transformIndexHtml(url, rendered);
          res.statusCode = 200;
          res.setHeader("Content-Type", "text/html; charset=utf-8");
          res.end(transformed);
        });
      },
    };
    return {
      plugins: [devHtmlPlugin, ...plugins],
      resolve,
      server: {
        host: "127.0.0.1",
        port: 5173,
        strictPort: true,
        proxy: {
          "/api/command": {
            target: wsTarget,
            ws: true,
            changeOrigin: true,
          },
          "/api": {
            target: apiTarget,
            changeOrigin: true,
          },
        },
      },
    };
  } else {
    // command === 'build'
    return {
      plugins,
      resolve,
      base: "",
      build: {
        rollupOptions: {
          input: {
            index: path.resolve(__dirname, "./public/index.html"),
          },
          output: {
            manualChunks: (id) => {
              // bundle dayjs files in a single chunk
              // this avoids having small files for each locale
              if (id.includes("dayjs/")) {
                return "dayjs";
                // bundle i18n in a separate chunk
              } else if (id.includes("i18n/")) {
                return "i18n";
              }
            },
          },
        },
      },
      experimental: {
        renderBuiltUrl(filename, { hostType }) {
          if (hostType === "js") {
            return { runtime: `window.__prependStaticUrl("${filename}")` };
          } else if (hostType === "html") {
            return `[{[ .StaticURL ]}]/${filename}`;
          } else {
            return { relative: true };
          }
        },
      },
    };
  }
});
