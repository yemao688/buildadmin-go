import { adminBaseRoutePath } from '/@/router/static/adminBase'

/**
 * 应用域：后台（/admin）与门户/API 渠道共用同一 HTML 入口，但运行时依赖完全不同。
 * 域在文档启动时确定一次并固定（Element Plus、语言域、App.vue 的 locale 都依赖它），
 * 运行中不得切换；跨域导航由 router 守卫整页重载完成。
 */
export type AppMode = 'admin' | 'portal'

// 域切换整页重载的一次性标记：重载后仍不匹配说明重载没有生效，必须放弃导航避免死循环
const DOMAIN_RELOAD_KEY = 'ba-app-domain-reload'

let currentMode: AppMode | null = null

/**
 * 判断路径是否属于后台应用域
 */
export function isAdminPath(path: string): boolean {
    return new RegExp(`^${adminBaseRoutePath}`).test(path)
}

/**
 * 从当前地址推断应用域。只读 hash：hash 路由下路径就是 hash 内容，
 * 且启动阶段 router 尚未安装（currentRoute 恒为 '/'），不能用 router 判断。
 */
export function resolveAppMode(): AppMode {
    const hashPath = window.location.hash.replace(/^#/, '').split('?')[0]
    return isAdminPath(hashPath) ? 'admin' : 'portal'
}

/**
 * 固定本次文档的应用域，必须在创建应用、加载语言、挂载路由之前调用
 */
export function setAppMode(mode: AppMode): void {
    currentMode = mode
}

/**
 * 取本次文档的应用域；未显式固定时按地址推断（仅兜底，正常启动路径都会先 setAppMode）
 */
export function getAppMode(): AppMode {
    if (!currentMode) currentMode = resolveAppMode()
    return currentMode
}

function dropDomainReloadMark() {
    try {
        sessionStorage.removeItem(DOMAIN_RELOAD_KEY)
    } catch {
        // sessionStorage 不可用（隐私模式）：无标记可清
    }
}

function hasDomainReloadMark(): boolean {
    try {
        return sessionStorage.getItem(DOMAIN_RELOAD_KEY) === '1'
    } catch {
        return false
    }
}

function markDomainReload() {
    try {
        sessionStorage.setItem(DOMAIN_RELOAD_KEY, '1')
    } catch {
        // sessionStorage 不可用：无法防循环，仍执行一次重载
    }
}

/**
 * 路由域守卫：目标路由的应用域与本次文档已固定的域不一致时整页重载一次。
 * 文档内跨域无法修复——插件、指令、语言与 App.vue 的 locale 都已按启动域固化。
 * @returns true 允许继续导航；false 已触发重载或放弃导航（调用方须直接中断）
 */
export function ensureRouteAppDomain(fullPath: string): boolean {
    const expected: AppMode = isAdminPath(fullPath) ? 'admin' : 'portal'
    if (getAppMode() === expected) {
        dropDomainReloadMark()
        return true
    }

    if (hasDomainReloadMark()) {
        dropDomainReloadMark()
        console.warn(`[appMode] 应用域未随整页重载切换，已放弃导航：${fullPath}`)
        return false
    }

    markDomainReload()
    // 只改 hash 不会触发文档重载，必须显式 reload 让入口按新域重新初始化
    window.location.hash = fullPath
    window.location.reload()
    return false
}
