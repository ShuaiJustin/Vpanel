import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'

const apiProxyTarget = process.env.VITE_API_PROXY_TARGET || 'http://127.0.0.1:8081'
const usePolling = /^(1|true)$/i.test(process.env.VITE_USE_POLLING || '')
const pollingInterval = Number(process.env.VITE_POLLING_INTERVAL || process.env.CHOKIDAR_INTERVAL || 300)
const hmrHost = process.env.VITE_HMR_HOST || undefined
const hmrClientPort = Number(process.env.VITE_HMR_CLIENT_PORT || 0) || undefined
const openBrowser = process.env.VITE_OPEN_BROWSER !== 'false'
const enableBuildSourceMap = /^(1|true)$/i.test(process.env.VITE_BUILD_SOURCEMAP || '')

const normalizeModuleId = (id) => id.replace(/\\/g, '/')

const manualChunks = (id) => {
  const normalizedId = normalizeModuleId(id)

  if (!normalizedId.includes('/node_modules/')) {
    return undefined
  }

  if (
    normalizedId.includes('/node_modules/vue/') ||
    normalizedId.includes('/node_modules/vue-router/') ||
    normalizedId.includes('/node_modules/pinia/') ||
    normalizedId.includes('/node_modules/@vue/')
  ) {
    return 'vue-core'
  }

  if (normalizedId.includes('/node_modules/@element-plus/icons-vue/')) {
    return 'element-plus-icons'
  }

  if (normalizedId.includes('/node_modules/element-plus/')) {
    return 'element-plus'
  }

  if (normalizedId.includes('/node_modules/echarts/core')) {
    return 'echarts-core'
  }

  if (normalizedId.includes('/node_modules/echarts/renderers')) {
    return 'echarts-renderers'
  }

  if (normalizedId.includes('/node_modules/chart.js/')) {
    return 'chartjs'
  }

  if (
    normalizedId.includes('/node_modules/axios/') ||
    normalizedId.includes('/node_modules/qrcode/')
  ) {
    return 'utils'
  }

  return undefined
}

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
    },
    extensions: ['.js', '.ts', '.vue', '.json'],
  },
  server: {
    host: '0.0.0.0',
    port: 5173,
    open: openBrowser,
    cors: true,
    hmr: {
      overlay: false,
      ...(hmrHost ? { host: hmrHost } : {}),
      ...(hmrClientPort ? { clientPort: hmrClientPort } : {}),
    },
    watch: {
      usePolling,
      interval: pollingInterval,
    },
    proxy: {
      '/api': {
        target: apiProxyTarget,
        changeOrigin: true,
        secure: false,
        ws: true,
        rewrite: (path) => path,
        timeout: 240000,
        configure: (proxy, options) => {
          proxy.options.timeout = 240000;
          
          const proxyState = {
            retries: {},
            maxRetries: 5,
            retryDelay: 1000,
          };
          
          proxy.on('error', (err, req, res) => {
            const reqId = `${req.method}-${req.url}-${Date.now()}`;
            proxyState.retries[reqId] = proxyState.retries[reqId] || 0;
            
            console.warn(`Proxy error (${reqId}):`, err.message, {
              url: req.url,
              method: req.method,
              code: err.code,
              retries: proxyState.retries[reqId]
            });
            
            const retryableErrors = ['ECONNRESET', 'ECONNREFUSED', 'ETIMEDOUT', 'ESOCKETTIMEDOUT', 'EPIPE'];
            
            if (retryableErrors.includes(err.code) && proxyState.retries[reqId] < proxyState.maxRetries) {
              proxyState.retries[reqId]++;
              
              const delay = proxyState.retryDelay * Math.pow(2, proxyState.retries[reqId] - 1);
              
              console.log(`连接问题 (${err.code}), 重试中... (${proxyState.retries[reqId]}/${proxyState.maxRetries}), 延迟: ${delay}ms`);
              
              if (req.method === 'GET' || req.url.includes('/xray/') || req.url.includes('/sse/')) {
                setTimeout(() => {
                  console.log(`重新尝试请求: ${reqId}`);
                  proxy.web(req, res, options);
                }, delay);
                return;
              }
            }
            
            if (req.url.includes('/sse') || req.headers.accept === 'text/event-stream') {
              console.log(`SSE连接错误 (${err.code}), 忽略...`);
              return;
            }
            
            if (!res.writableEnded) {
              const statusCode = err.code === 'ECONNREFUSED' ? 503 : 500;
              const errorMessage = {
                error: true,
                code: err.code,
                message: `代理服务器错误: ${err.message}`,
                url: req.url,
                retryable: retryableErrors.includes(err.code),
                retries: proxyState.retries[reqId],
                timestamp: new Date().toISOString()
              };
              
              res.writeHead(statusCode, {
                'Content-Type': 'application/json',
              });
              res.end(JSON.stringify(errorMessage));
            }
          });
          
          proxy.on('proxyReq', (proxyReq, req) => {
            if (req.url.includes('/xray/')) {
              console.log(`请求Xray API: ${req.method} ${req.url}`);
            }
          });

          proxy.on('proxyRes', (proxyRes, req) => {
            if (req.url.includes('/xray/') && proxyRes.statusCode >= 400) {
              console.warn(`Xray API错误响应: ${proxyRes.statusCode} ${req.method} ${req.url}`);
            }
          });
        }
      },
    },
  },
  optimizeDeps: {
    include: ['vue', 'vue-router', 'pinia', 'element-plus', 'axios', 'qrcode'],
  },
  css: {
    preprocessorOptions: {
      scss: {
        api: 'modern-compiler',
      },
    },
  },
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
    sourcemap: enableBuildSourceMap,
    reportCompressedSize: false,
    chunkSizeWarningLimit: 650,
    minify: 'terser',
    terserOptions: {
      compress: {
        drop_console: true,
        drop_debugger: true,
      },
    },
    rollupOptions: {
      output: {
        chunkFileNames: 'assets/js/[name]-[hash].js',
        entryFileNames: 'assets/js/[name]-[hash].js',
        assetFileNames: 'assets/[ext]/[name]-[hash].[ext]',
        manualChunks
      }
    }
  },
}) 
