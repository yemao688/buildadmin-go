import type { App } from 'vue'
import { auth } from '/@/utils/auth'
import { directives } from '/@/utils/directives'

/**
 * 后台域指令注册：通用指令 + v-auth。
 * 单独拆文件是为了让门户入口只注册通用指令，不把 navTabs/auth 拉进门户静态图。
 */
export function adminDirectives(app: App): void {
    directives(app)
    app.directive('auth', {
        mounted(el, binding) {
            if (!binding.value) return false
            if (!auth(binding.value)) el.parentNode.removeChild(el)
        },
    })
}
