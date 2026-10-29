import { createApp } from 'vue'
import mitt from 'mitt'
import App from './App.vue'
import pinia from '/@/stores/index'
import router from '/@/router'
import { refreshTokenRequest } from '/@/utils/axios'
import { registerTokenProvider } from '/@/utils/tokenProvider'
import { useAdminInfo } from '/@/stores/adminInfo'
import { useBaAccount } from '/@/stores/baAccount'
import { useUserInfo } from '/@/stores/userInfo'
import { resetChunkReloadCounter, setupChunkReloadWatchdog } from '/@/utils/chunkReload'
import { resolveAppMode, setAppMode } from '/@/utils/appMode'
import 'font-awesome/css/font-awesome.min.css'
import '/@/styles/index.scss'
// modules import mark, Please do not remove.

registerTokenProvider({
    domain: 'admin',
    header: 'batoken',
    store: () => useAdminInfo(),
    loginRoute: 'adminLogin',
    refreshType: 'admin-refresh',
})

registerTokenProvider({
    domain: 'baAccount',
    header: 'ba-user-token',
    store: () => useBaAccount(),
    loginRoute: '',
    refreshType: 'user-refresh',
    useAnotherToken: true,
})

registerTokenProvider({
    domain: 'userInfo',
    header: 'ba-user-token',
    store: () => useUserInfo(),
    loginRoute: '',
    useAnotherToken: true,
    refresher: ({ provider, lang, refreshToken, token }) => refreshTokenRequest(provider, lang, refreshToken, token),
})

/**
 * 入口只做与域名无关的启动：固定应用域、注册 chunk 看门狗、创建应用与 pinia，
 * 其余依赖按域分流到 bootstrap/admin 或 bootstrap/portal 动态加载，
 * 避免后台专用的 Element Plus / 图标 barrel 进入门户首屏静态导入图。
 */
async function start() {
    const mode = resolveAppMode()
    setAppMode(mode)

    // 看门狗必须先于域 bootstrap 的动态 import 注册：该 import 自身 404 时也要能自愈
    setupChunkReloadWatchdog(router)

    const app = createApp(App)
    app.use(pinia)

    const { default: bootstrap } = mode === 'admin' ? await import('./bootstrap/admin') : await import('./bootstrap/portal')
    await bootstrap(app)

    app.mount('#app')

    // mount 成功 = 本次启动正常：重置自动刷新预算，给下一次部署全新计数
    resetChunkReloadCounter()

    // modules start mark, Please do not remove.

    app.config.globalProperties.eventBus = mitt()
}
start()
