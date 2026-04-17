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
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
    sourcemap: enableBuildSourceMap,
    reportCompressedSize: false,
    chunkSizeWarningLimit: 500,
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
        manualChunks: {
          // Vue core (~100KB) - Core Vue framework and state management
          'vue-core': ['vue', 'vue-router', 'pinia'],
          
          // Element Plus (~400KB) - UI component library
          'element-plus': ['element-plus', '@element-plus/icons-vue'],
          
          // ECharts (~200KB with selective imports) - Only used chart types
          // Selective imports in utils/charts.ts: LineChart, BarChart, PieChart
          // Components: Title, Tooltip, Grid, Legend
          // Renderer: CanvasRenderer only
          'echarts': [
            'echarts/core',
            'echarts/renderers',
            'vue-echarts'
          ],
          
          // Chart.js (~200KB) - Used in user stats
          'chartjs': ['chart.js'],
          
          // Utils (~100KB) - HTTP client and utilities
          'utils': ['axios', 'qrcode'],
        }
      }
    }
  },
}) 
