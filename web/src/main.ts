import { createApp } from 'vue'
import axios from 'axios'
import App from './App.vue'
import router from './router'
import { loadLang } from '/@/lang/index'
import { indexUrl } from '/@/api/frontend'
import { getUrl } from '/@/utils/axios'
import { isAdminApp, registerIcons } from '/@/utils/common'
import { refreshTokenRequest } from '/@/utils/axios'
import ElementPlus from 'element-plus'
import mitt from 'mitt'
import pinia from '/@/stores/index'
import { useAdminInfo } from '/@/stores/adminInfo'
import { useBaAccount } from '/@/stores/baAccount'
import { useConfig } from '/@/stores/config'
import { useUserInfo } from '/@/stores/userInfo'
import { directives } from '/@/utils/directives'
import { registerTokenProvider } from '/@/utils/tokenProvider'
import { resetChunkReloadCounter, setupChunkReloadWatchdog } from '/@/utils/chunkReload'
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

    const config = useConfig()

    // 门户应用入口：先请求后端默认语言（/api/index/index 返回的 default_language，后端并行线提供），
    // 在 loadLang 之前完成 initPortalLang，保证首帧渲染语言正确；失败或字段缺失时保持默认 'zh-cn'，
    // 不阻塞启动。此请求在 vue-i18n 挂载前发出，响应消息无需翻译；
    // think-lang 此时为 portalLang 当前值（默认 zh-cn），不影响。
    if (!isAdminApp()) {
        try {
            const res = await axios.get(getUrl() + indexUrl + 'index')
            const data = res?.data?.data
            const defaultLang = data?.default_language
            if (defaultLang) config.initPortalLang(defaultLang)

            // 同一请求一并返回 currency 数组（按 weigh DESC 排序，第一项为后端默认货币）：
            // 对称 initPortalLang，写入默认货币与货币列表。列表（currencyArray/currencyRates）
            // 是数据，始终以最新后端列表为准；默认货币仅在用户显式选择前写入（initPortalCurrency 内部判断）。
            const currencyList = data?.currency
            if (Array.isArray(currencyList) && currencyList.length > 0) {
                config.initPortalCurrency(
                    currencyList[0].code,
                    currencyList.map((item) => ({ code: item.code, name: item.name, symbol: item.symbol })),
                    Object.fromEntries(currencyList.map((item) => [item.code, Number(item.rate) || 1]))
                )
            }
        } catch (e) {
            console.warn('[i18n] 获取门户默认语言/货币失败，使用默认 zh-cn / CNY', e)
        }
    }

    // chunk 加载失败自愈看门狗（须在 loadLang 之前注册：初始语言包 chunk
    // 404 时 loadLang 抛错会中断后续代码，未注册的看门狗无法兜底该失败）
    setupChunkReloadWatchdog(router)

    // 全局语言包加载
    await loadLang(app)

    app.use(router)
    app.use(ElementPlus)

    // 全局注册
    directives(app) // 指令
    registerIcons(app) // icons

    app.mount('#app')

    // mount 成功 = 本次启动正常：重置自动刷新预算，给下一次部署全新计数
    resetChunkReloadCounter()

    // modules start mark, Please do not remove.

    app.config.globalProperties.eventBus = mitt()
}
start()
