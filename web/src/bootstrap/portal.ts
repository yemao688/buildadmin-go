import type { App } from 'vue'
import axios from 'axios'
// 从具体组件模块导入（不用 element-plus 根 barrel）：根 barrel 与后台的完整插件共享同一模块，
// 会把整个组件库重新拉回门户静态图
import { ElButton, ElButtonGroup } from 'element-plus/es/components/button/index'
import { ElCard } from 'element-plus/es/components/card/index'
import { ElConfigProvider } from 'element-plus/es/components/config-provider/index'
import { ElDropdown, ElDropdownItem, ElDropdownMenu } from 'element-plus/es/components/dropdown/index'
import { ElIcon } from 'element-plus/es/components/icon/index'
// 按组件引入样式：每个 style/css 会连带引入其依赖组件的样式，避免整包 element-plus CSS 进首屏
import 'element-plus/es/components/button/style/css'
import 'element-plus/es/components/button-group/style/css'
import 'element-plus/es/components/card/style/css'
import 'element-plus/es/components/config-provider/style/css'
import 'element-plus/es/components/dropdown/style/css'
import 'element-plus/es/components/dropdown-menu/style/css'
import 'element-plus/es/components/dropdown-item/style/css'
import 'element-plus/es/components/icon/style/css'
import '/@/styles/portal.scss'
import router from '/@/router'
import { indexUrl } from '/@/api/frontend'
import { loadLang } from '/@/lang/index'
import { useConfig } from '/@/stores/config'
import { getUrl } from '/@/utils/axios'
import { registerIcons } from '/@/utils/common'
import { directives } from '/@/utils/directives'

/**
 * 门户域可用的 Element Plus 组件白名单（框架门户示例实际用到的组件）。
 * 业务门户需要更多组件时在此追加，并补一行对应的 style/css 导入；
 * 若门户大量使用组件，可改用 installFullElementPlus 整体接管（首屏代价回到完整组件库）。
 */
const portalComponents = [ElConfigProvider, ElCard, ElDropdown, ElDropdownMenu, ElDropdownItem, ElButton, ElButtonGroup, ElIcon]

/**
 * 逃生口：门户整包引入 Element Plus（含完整样式），供大量使用组件库的下游门户使用
 */
export async function installFullElementPlus(app: App): Promise<void> {
    const { default: ElementPlus, elementPlusCss, displayCss } = await import('./elementPlusFull')
    for (const css of [elementPlusCss, displayCss]) {
        const style = document.createElement('style')
        style.textContent = css
        document.head.appendChild(style)
    }
    app.use(ElementPlus)
}

/**
 * 门户域首帧配置：先取后端默认语言（/api/index/index 返回的 default_language），
 * 在 loadLang 之前完成 initPortalLang，保证首帧渲染语言正确；失败或字段缺失时保持默认 'zh-cn'，
 * 不阻塞启动。此请求在 vue-i18n 挂载前发出，响应消息无需翻译；think-lang 此时为 portalLang
 * 当前值（默认 zh-cn），不影响。
 */
async function initPortalConfig(): Promise<void> {
    const config = useConfig()
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
                currencyList.map((item: anyObj) => ({ code: item.code, name: item.name, symbol: item.symbol })),
                Object.fromEntries(currencyList.map((item: anyObj) => [item.code, Number(item.rate) || 1]))
            )
        }
    } catch (e) {
        console.warn('[i18n] 获取门户默认语言/货币失败，使用默认 zh-cn / CNY', e)
    }
}

/**
 * 门户域启动：只注册白名单组件与门户样式，不引入完整 Element Plus 与图标 barrel。
 */
export default async function bootstrapPortal(app: App): Promise<void> {
    for (const component of portalComponents) app.use(component)
    await initPortalConfig()
    await loadLang(app)
    app.use(router)
    directives(app)
    registerIcons(app)
}
