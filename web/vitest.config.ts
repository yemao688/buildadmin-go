import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'
import { defineConfig } from 'vitest/config'

/**
 * Vitest 独立配置：只复用 Vite 的模块解析（vue 插件、'/@' 别名、扩展名顺序），
 * 不引入 vite.config.ts 的 svgBuilder / 热更新 / 构建切块等与单测无关的链路。
 */
export default defineConfig({
    plugins: [vue()],
    resolve: {
        alias: { '/@': resolve(__dirname, './src') },
        // 与 vite.config.ts 保持同一解析顺序（.ts/.tsx 优先）
        extensions: ['.ts', '.tsx', '.js', '.jsx', '.mjs', '.json']
    },
    test: {
        environment: 'happy-dom',
        include: ['src/**/*.test.ts'],
        restoreMocks: true
    }
})
