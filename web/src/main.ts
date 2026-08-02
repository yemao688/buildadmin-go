import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import { loadLang } from '/@/lang/index'
import { registerIcons } from '/@/utils/common'
import { refreshTokenRequest } from '/@/utils/axios'
import ElementPlus from 'element-plus'
import mitt from 'mitt'
import pinia from '/@/stores/index'
import { useAdminInfo } from '/@/stores/adminInfo'
import { useBaAccount } from '/@/stores/baAccount'
import { useUserInfo } from '/@/stores/userInfo'
import { directives } from '/@/utils/directives'
import { registerTokenProvider } from '/@/utils/tokenProvider'
import 'element-plus/dist/index.css'
import 'element-plus/theme-chalk/display.css'
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

async function start() {
    const app = createApp(App)
    app.use(pinia)

    // 全局语言包加载
    await loadLang(app)

    app.use(router)
    app.use(ElementPlus)

    // 全局注册
    directives(app) // 指令
    registerIcons(app) // icons

    app.mount('#app')

    // modules start mark, Please do not remove.

    app.config.globalProperties.eventBus = mitt()
}
start()
