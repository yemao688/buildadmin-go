import NProgress from 'nprogress'
import 'nprogress/nprogress.css'
import { createRouter, createWebHashHistory } from 'vue-router'
import langAutoLoadMap from '/@/lang/autoload'
import { loadAndMergeMessages } from '/@/lang/index'
import staticRoutes from '/@/router/static'
import { adminBaseRoutePath } from '/@/router/static/adminBase'
import { useConfig } from '/@/stores/config'
import { loading } from '/@/utils/loading'

const router = createRouter({
    history: createWebHashHistory(),
    routes: staticRoutes,
})

router.beforeEach(async (to) => {
    NProgress.configure({ showSpinner: false })
    NProgress.start()
    if (!window.existLoading) {
        loading.show()
        window.existLoading = true
    }

    // 按需动态加载页面的语言包-start
    const config = useConfig()
    const loadPath: string[] = []
    const lang = config.lang.defaultLang
    if (to.path in langAutoLoadMap) {
        loadPath.push(...langAutoLoadMap[to.path as keyof typeof langAutoLoadMap])
    }
    // 门户语言目录：后台 → backend，其余（前台/业务门户）→ frontend
    const prefix = (to.path.startsWith(adminBaseRoutePath) ? './backend/' : './frontend/') + lang

    // 去除 path 中的 /admin；前台/业务门户按自身 path 加载
    const adminPath = to.path.startsWith(adminBaseRoutePath) ? to.path.slice(adminBaseRoutePath.length) : to.path
    if (adminPath && adminPath !== '/') loadPath.push(prefix + adminPath + '.ts')

    // 根据路由 name 加载的语言包
    if (to.name) {
        loadPath.push(prefix + '/' + to.name.toString() + '.ts')
    }

    // 路由公共语言包
    loadPath.push(prefix + '.ts')

    // 等待语言包加载并合并完成后再放行路由，避免页面已渲染但语言包未就绪
    await loadAndMergeMessages(loadPath, prefix, lang)
    // 动态加载语言包-end
})

// 路由加载后
router.afterEach(() => {
    if (window.existLoading) {
        loading.hide()
    }
    NProgress.done()
})

export default router
