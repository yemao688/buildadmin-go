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
    // 门户语言目录按路径首段解析：/admin → backend，/seller → seller，
    // /buyer → buyer，根路径 / → frontend。业务新开门户只需建
    // lang/<portal>/{lang}/ 目录，无需改本文件。
    const portalName = to.path.split('/')[1] || ''
    const langDir = portalName === 'admin' ? 'backend' : portalName || 'frontend'
    const prefix = './' + langDir + '/' + lang

    // 去除路径前缀（/admin 或 /seller 等门户前缀），页面语言包按相对路径加载
    const relativePath = to.path.slice(1 + portalName.length)
    if (relativePath && relativePath !== '/') loadPath.push(prefix + relativePath + '.ts')

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
