import type { App } from 'vue'
import ElementPlus from 'element-plus'
import adminThemeCss from 'element-plus/dist/index.css?inline'
import displayCss from 'element-plus/theme-chalk/display.css?inline'
import adminOverridesCss from '/@/styles/admin.scss?inline'
import router from '/@/router'
import { loadLang } from '/@/lang/index'
import { registerIcons } from '/@/utils/common'
import { registerElementPlusIcons } from '/@/utils/elementIcons'
import { directives } from '/@/utils/directives'

/**
 * 注入后台样式。主题/覆盖样式以 ?inline 作为字符串随后台 chunk 加载，
 * 不进入入口 CSS（Vite 8/Rolldown 会把异步 chunk 的 CSS 并入入口）。
 */
function injectStyle(css: string): void {
    const style = document.createElement('style')
    style.textContent = css
    document.head.appendChild(style)
}

/**
 * 后台域启动：完整 Element Plus + 后台图标 + 后台样式。
 * 只有本模块允许静态引入 element-plus 与图标 barrel。
 */
export default async function bootstrapAdmin(app: App): Promise<void> {
    injectStyle(adminThemeCss)
    injectStyle(displayCss)
    injectStyle(adminOverridesCss)

    await loadLang(app)
    app.use(router)
    app.use(ElementPlus)
    directives(app)
    registerIcons(app)
    registerElementPlusIcons(app)
}
