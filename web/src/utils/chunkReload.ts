import type { Router } from 'vue-router'

// chunk 加载失败自愈看门狗：
// SPA 部署后旧 chunk 404（index.html 引用旧 hash 资源，发版后旧文件被删）时，
// 已打开页面的路由分包与语言包动态 import 会触发 "Failed to fetch dynamically imported module"，
// 需要手动刷新。后端已配缓存头治本，这里做前端兜底：检测到 chunk 加载失败自动刷新。
// 循环防护：sessionStorage 记录上次刷新时间戳与累计次数——3 秒节流限速，
// 累计超上限（MAX_AUTO_RELOADS）后放弃自动刷新，避免发布窗口（新 index.html
// 已生效、chunk 仍在传输）内反复刷新形成死循环。

// 记录自动刷新状态的 sessionStorage 键
const SESSION_STORAGE_KEY = 'chunk-reload-ts'

// 自动刷新最小间隔（毫秒）
const RELOAD_INTERVAL = 3000

// 自动刷新累计次数上限：超过后放弃，避免发布窗口内无限循环刷新
const MAX_AUTO_RELOADS = 3

/**
 * 匹配 chunk 加载失败错误（宽匹配覆盖 vite/rolldown 差异文案，
 * 涵盖路由分包与语言包 import.glob 两条懒加载链）
 */
function isChunkLoadError(error: unknown): boolean {
    return error instanceof Error && /Failed to fetch dynamically imported module|Importing a module script failed|Loading chunk/.test(error.message)
}

/**
 * 节流 + 次数上限的自动刷新：3 秒内只刷一次，累计超过上限后放弃
 * （sessionStorage 不可用时优雅降级为不刷新，不抛错）
 */
function reloadOnce() {
    try {
        const now = Date.now()
        const last = Number(sessionStorage.getItem(SESSION_STORAGE_KEY) ?? 0)
        const count = Number(sessionStorage.getItem(SESSION_STORAGE_KEY + '-count') ?? 0)
        if (now - last < RELOAD_INTERVAL) return
        if (count >= MAX_AUTO_RELOADS) return
        sessionStorage.setItem(SESSION_STORAGE_KEY, String(now))
        sessionStorage.setItem(SESSION_STORAGE_KEY + '-count', String(count + 1))
        location.reload()
    } catch {
        // sessionStorage 不可用（隐私模式等）：不做自动刷新，避免白屏循环
    }
}

/**
 * 重置自动刷新预算：应用成功启动（mount 完成）后调用，给每次部署清零计数。
 * 不能放在 window load——入口自身永久损坏时 load 也会触发，会重新开启无限刷新循环；
 * 以 mount 成功为门（失败则启动中断，不会调用），节流+上限防护保持完整。
 */
export function resetChunkReloadCounter() {
    try {
        sessionStorage.removeItem(SESSION_STORAGE_KEY + '-count')
    } catch {
        // sessionStorage 不可用（隐私模式等）：无需重置
    }
}

/**
 * 注册 chunk 加载失败自愈看门狗（入口处调用一次即可）：
 * - router.onError 捕获路由分包懒加载失败
 * - unhandledrejection 捕获 loadLang 等非路由动态 import 失败
 * 两个入口共用同一匹配与节流逻辑
 */
export function setupChunkReloadWatchdog(router: Router) {
    const handler = (error: unknown) => {
        if (isChunkLoadError(error)) reloadOnce()
    }
    router.onError(handler)
    window.addEventListener('unhandledrejection', (event) => handler(event.reason))
}