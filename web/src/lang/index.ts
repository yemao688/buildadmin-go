import { isEmpty, uniq } from 'lodash-es'
import type { App } from 'vue'
import type { Composer, I18n } from 'vue-i18n'
import { createI18n } from 'vue-i18n'
import { handleMsglist } from './merge'
import { useConfig } from '/@/stores/config'
import { isAdminApp } from '/@/utils/common'

/*
 * 默认只引入 element-plus 的中英文语言包
 * 其他语言包请自行在此 import,并添加到 assignLocale 内
 * 动态 import 只支持相对路径，所以无法按需 import element-plus 的语言包
 * 但i18n的 messages 内是按需载入的
 */
import elementEnLocale from 'element-plus/es/locale/lang/en'
import elementZhcnLocale from 'element-plus/es/locale/lang/zh-cn'

export let i18n: {
    global: Composer
}

// 准备要合并的语言包
/*
 * 新增语言步骤：
 * 1. 在 web/src/lang/<locale>/ 目录添加语言包文件（glob 自动发现，无需修改本文件的 glob 分支）
 * 2. 在此处加一行 element-plus locale 映射，例如：'fr': [elementFrLocale]
 *    并同步顶部 import：import elementFrLocale from 'element-plus/es/locale/lang/fr'
 * 3. 根目录补 globs-<locale>.ts（框架全局语言包）
 */
const assignLocale: anyObj = {
    'zh-cn': [elementZhcnLocale],
    en: [elementEnLocale],
}

export async function loadLang(app: App) {
    const config = useConfig()

    // 按应用域选择语言：后台（admin 应用）用 admin 域语言（管理员可切换，默认 zh-cn）；
    // 门户应用用 portal 域语言（默认取后端 /api/index/index 的 default_language，用户可在前台切换）。
    // 注意：门户入口须在 loadLang 前完成 initPortalLang（见 main.ts），否则首帧语言不正确。
    const isAdmin = isAdminApp()
    const rawLocale = isAdmin ? config.lang.defaultLang : config.portalLang.defaultLang
    // 后端 default_language 可能返回前端尚未支持的语言（未登记到 assignLocale 且无 globs-<locale>.ts），
    // 回退到 zh-cn，避免动态 import 失败导致应用无法启动
    const locale = assignLocale[rawLocale] ? rawLocale : 'zh-cn'

    // 加载框架全局语言包
    const lang = await import(`./globs-${locale}.ts`)
    const message = lang.default ?? {}

    // 按需加载语言包文件的句柄
    // 全量 glob（自动发现所有语言目录）：新增语言只需在 web/src/lang/<locale>/ 下建目录并添加
    // 语言包文件，无需再修改本文件的语言分支；loadAndMergeMessages 按具体路径精确匹配取用。
    // 根目录下的 globs-*.ts / autoload.ts / index.ts 等不属于语言包目录，不会被本 glob 收录。
    window.loadLangHandle = {
        ...import.meta.glob(['./*/**/*.ts']),
    }

    /*
     * 加载页面语言包 import.meta.glob 的路径不能使用变量 import() 在 Vite 中目录名不能使用变量(编译后,文件名可以)
     * common 语言包全量 glob（含全部语言目录），getLangFileMessage 内按当前 locale 过滤
     */
    assignLocale[locale].push(getLangFileMessage(import.meta.glob('./common/**/*.ts', { eager: true }), locale))

    const messages = {
        [locale]: {
            ...message,
        },
    }

    // 合并语言包(含element-puls、页面语言包)
    Object.assign(messages[locale], ...assignLocale[locale])

    i18n = createI18n({
        locale: locale,
        legacy: false, // 组合式api
        globalInjection: true, // 挂载$t,$d等到全局
        fallbackLocale: isAdmin ? config.lang.fallbackLang : config.portalLang.fallbackLang,
        messages,
    })

    app.use(i18n as I18n)
    return i18n
}

function getLangFileMessage(mList: any, locale: string) {
    let msg: anyObj = {}
    locale = '/' + locale
    for (const path in mList) {
        // 全量 glob 时其它语言的文件也会被导入，必须按路径过滤出当前 locale 的文件
        // （路径含 /<locale>/ 或以 /<locale>.ts 结尾），否则其它语言文件会以错误的 pathName 混入合并结果
        if (!path.includes(locale + '/') && !path.endsWith(locale + '.ts')) continue
        if (mList[path].default) {
            //  获取文件名
            const pathName = path.slice(path.lastIndexOf(locale) + (locale.length + 1), path.lastIndexOf('.'))
            if (pathName.indexOf('/') > 0) {
                msg = handleMsglist(msg, mList[path].default, pathName)
            } else {
                msg[pathName] = mList[path].default
            }
        }
    }
    return msg
}

export function mergeMessage(message: anyObj, pathName = '') {
    if (isEmpty(message)) return
    if (!pathName) {
        return i18n.global.mergeLocaleMessage(i18n.global.locale.value, message)
    }
    let msg: anyObj = {}
    if (pathName.indexOf('/') > 0) {
        msg = handleMsglist(msg, message, pathName)
    } else {
        msg[pathName] = message
    }
    i18n.global.mergeLocaleMessage(i18n.global.locale.value, msg)
}

// 已加载的语言包文件路径，避免重复 import 与合并
const loadedLangPaths = new Set<string>()

/**
 * 按需加载并合并路由对应的语言包文件
 * @param rawPaths 语言包文件相对路径，支持 ${lang} 占位符
 * @param prefix 当前语言的前缀（如 ./backend/zh-cn），用于剥离出文件命名空间
 * @param lang 当前语言
 */
export async function loadAndMergeMessages(rawPaths: string[], prefix: string, lang: string) {
    const paths = uniq(rawPaths).map((path) => path.replaceAll('${lang}', lang))
    await Promise.all(
        paths.map(async (path) => {
            if (loadedLangPaths.has(path)) return
            const loader = window.loadLangHandle[path] as undefined | (() => Promise<{ default: anyObj }>)
            if (!loader) return
            loadedLangPaths.add(path)
            try {
                const res = await loader()
                const pathName = path.slice(path.lastIndexOf(prefix) + (prefix.length + 1), path.lastIndexOf('.'))
                mergeMessage(res.default, pathName)
            } catch (e) {
                // 加载失败时移除记录以允许下次重试，且不阻断路由跳转
                loadedLangPaths.delete(path)
                console.error(`[i18n] 语言包加载失败: ${path}`, e)
            }
        })
    )
}

/**
 * 切换默认语言
 * @param lang 语言代码（zh-cn / en / 新增语言；须已登记到 assignLocale，否则忽略切换）
 * @param domain 语言域：'admin' = 后台域（默认，现状行为）；'portal' = 前台域（写入 portalLang 并标记为用户显式选择）
 */
export function editDefaultLang(lang: string, domain: 'admin' | 'portal' = 'admin'): void {
    // 切换前校验目标语言已在前端语言注册表登记（assignLocale 含 element-plus
    // locale 映射）。后端启用（country_language 有）但前端无语言包（assignLocale
    // 无）的语言，reload 后 loadLang 会兜底回退 zh-cn 闪回，这里直接拒绝切换。
    if (!assignLocale[lang]) {
        console.warn(`[i18n] language "${lang}" has no frontend locale pack, switch ignored`)
        return
    }

    const config = useConfig()
    if (domain == 'portal') {
        // 前台域：标记为用户显式选择，此后后端 default_language 不再覆盖
        config.setPortalLang(lang)
    } else {
        config.setLang(lang)
    }

    /*
     * 语言包是按需加载的,比如默认语言为中文,则只在app实例内加载了中文语言包,所以切换语言需要进行 reload
     */
    location.reload()
}

export { handleMsglist, mergeMsg } from './merge'
