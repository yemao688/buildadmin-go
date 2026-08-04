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
    // 门户语言目录注册表：key = 路由前缀，value = lang 目录名。
    // 后台固定映射 backend；根门户（/ 及其所有无前缀页面）默认 frontend；
    // 业务新增带前缀门户（/seller、/buyer…）在此追加一行即可。
    const portalLangDirs: Record<string, string> = {
        [adminBaseRoutePath]: 'backend',
    }
    const portalPrefix = '/' + (to.path.split('/')[1] || '')
    const langDir = portalLangDirs[portalPrefix] ?? 'frontend'
    const prefix = './' + langDir + '/' + lang

    // 页面语言包按相对路径加载：注册门户剥离其前缀，根门户用完整路径
    const relativePath = portalLangDirs[portalPrefix] ? to.path.slice(portalPrefix.length) : to.path
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
