import * as elIcons from '@element-plus/icons-vue'
import type { App } from 'vue'

/**
 * 全局注册 Element Plus 图标（后台域专用）。
 * 图标 barrel 未压缩约 490KB，且 namespace 导入无法 tree-shake，
 * 因此本模块只能被后台 bootstrap 静态引入，不得进入门户的静态导入图。
 */
export function registerElementPlusIcons(app: App): void {
    const icons = elIcons as any
    for (const name in icons) {
        app.component(`el-icon-${icons[name].name}`, icons[name])
    }
}
