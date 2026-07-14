import NProgress from 'nprogress'
import 'nprogress/nprogress.css'
import { createRouter, createWebHashHistory } from 'vue-router'
import langAutoLoadMap from '/@/lang/autoload'
import { loadAndMergeMessages } from '/@/lang/index'
import staticRoutes from '/@/router/static'
import { adminBaseRoutePath } from '/@/router/static/adminBase'
import { useConfig } from '/@/stores/config'
import { isAdminApp } from '/@/utils/common'
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
    let prefix = ''
    if (isAdminApp(to.fullPath)) {
        prefix = './backend/' + lang

        // 去除 path 中的 /admin
        const adminPath = to.path.slice(to.path.indexOf(adminBaseRoutePath) + adminBaseRoutePath.length)
        if (adminPath) loadPath.push(prefix + adminPath + '.ts')
    } else {
        prefix = './frontend/' + lang
        loadPath.push(prefix + to.path + '.ts')
    }

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
