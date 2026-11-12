import { useNavTabs } from '/@/stores/navTabs'
import { getCurrentRoutePath } from '/@/utils/common'

/**
 * 前端鉴权（后台链路专用）。
 * 提供 string 时按当前路由 path 自动拼接并鉴权；提供 { name, subNodeName } 时按菜单 name 鉴权
 * （不受当前 path 限制，检查按钮须先查上级菜单 name 再查 subNodeName）。
 * 依赖后台标签页 store 的 authNode 缓存，门户不得引用；门户入口图从这里隔离 navTabs。
 */
export function auth(node: string): boolean
export function auth(node: { name: string; subNodeName?: string }): boolean
export function auth(node: string | { name: string; subNodeName?: string }) {
    const store = useNavTabs()
    if (typeof node === 'string') {
        const path = getCurrentRoutePath()
        if (store.state.authNode.has(path)) {
            const subNodeName = path + (path == '/' ? '' : '/') + node
            if (store.state.authNode.get(path)!.some((v: string) => v == subNodeName)) {
                return true
            }
        }
    } else {
        // 节点列表中没有找到 name
        if (!node.name || !store.state.authNode.has(node.name)) return false

        // 无需继续检查子节点或未找到子节点
        if (!node.subNodeName || store.state.authNode.get(node.name)?.includes(node.subNodeName)) return true
    }
    return false
}
