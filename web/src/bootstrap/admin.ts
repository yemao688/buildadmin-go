import type { App } from 'vue'
import ElementPlus from 'element-plus'
import 'element-plus/theme-chalk/display.css'
// 后台样式必须排在 element-plus 基样式之后：element.scss 的覆盖项靠后置声明生效
import '/@/styles/admin.scss'
import router from '/@/router'
import { loadLang } from '/@/lang/index'
import { registerIcons } from '/@/utils/common'
import { registerElementPlusIcons } from '/@/utils/elementIcons'
import { directives } from '/@/utils/directives'

/**
 * 后台域启动：完整 Element Plus + 后台图标 + 后台样式。
 * 只有本模块允许静态引入 element-plus 与图标 barrel。
 */
export default async function bootstrapAdmin(app: App): Promise<void> {
    await loadLang(app)
    app.use(router)
    app.use(ElementPlus)
    directives(app)
    registerIcons(app)
    registerElementPlusIcons(app)
}
