import { defineStore } from 'pinia'
import { reactive, ref } from 'vue'
import { STORE_CONFIG } from '/@/stores/constant/cacheKey'
import type { Crud, CurrencyOption, Lang, Layout, PortalCurrency, PortalLang } from '/@/stores/interface'
import { useNavTabs } from '/@/stores/navTabs'

export const useConfig = defineStore(
    'config',
    () => {
        const layout: Layout = reactive({
            // 全局
            showDrawer: false,
            shrink: false,
            layoutMode: 'Default',
            mainAnimation: 'slide-right',
            isDark: false,

            // 菜单栏
            menuBackground: ['#ffffff', '#1d1e1f'],
            menuColor: ['#303133', '#CFD3DC'],
            menuActiveBackground: ['#ffffff', '#1d1e1f'],
            menuActiveColor: ['#409eff', '#3375b9'],
            menuHoverBackground: ['#ecf5ff', '#18222c'],
            menuWidth: 260,
            menuDefaultIcon: 'fa fa-circle-o',
            menuCollapse: false,
            menuUniqueOpened: false,
            menuShowTopBar: true,
            menuTopBarBackground: ['#fcfcfc', '#1d1e1f'],
            menuTopBarColor: ['#409eff', '#3375b9'],
            menuTopBarCenter: false,
            menuTopBarLogo: false,
            menuToolBarAutoHide: false,
            menuToolBarColor: ['#303133', '#CFD3DC'],
            menuToolBarHoverColor: ['#409eff', '#CFD3DC'],
            menuToolBarHoverBackground: ['#ecf5ff', '#18222c'],

            // 主菜单栏额外配置项（一些布局存在主次两个菜单栏）
            menuBackgroundPrimary: ['#f5f5f5', '#18222c'],
            menuActiveBackgroundPrimary: ['#c6e2ff', '#1d1e1f'],

            // 左分布局独有菜单栏配置
            menuWidthLeftSplit: 180,
            menuHoverBackgroundLeftSplit: ['#ebebeb', '#213d5b'],

            // 顶栏
            headerBarTabColor: ['#000000', '#CFD3DC'],
            headerBarTabActiveColor: ['#000000', '#409EFF'],
            headerBarBackground: ['#ffffff', '#1d1e1f'],
            headerBarHoverBackground: ['#f5f5f5', '#18222c'],
            headerBarTabActiveBackground: ['#f5f5f5', '#141414'],
            headerBarTabActiveBackgroundFloating: ['#ffffff', '#1d1e1f'],

            // tour
            // 布局漫游式引导
            layoutTour: false,
            layoutTourUnfinished: true,
        })

        const lang: Lang = reactive({
            defaultLang: 'zh-cn',
            fallbackLang: 'zh-cn',
            langArray: [
                { name: 'zh-cn', value: '中文简体' },
                { name: 'en', value: 'English' },
            ],
        })

        // 前台语言域（/api/* 与门户页）：全局一个前台默认语言（非 per-portal），
        // 默认语言 = 后端 country_language 第一条，由 /api/index/index 返回的 default_language
        // 字段经 initPortalLang 写入（后端并行线提供）；用户可在前台显式切换（setPortalLang）。
        // 与 lang（后台域）互相独立、互不覆盖。
        const portalLang: PortalLang = reactive({
            defaultLang: 'zh-cn',
            fallbackLang: 'zh-cn',
        })

        // 前台语言是否已被用户显式设置过（随 storeConfig_v3 一并持久化）。
        // 策略：为 true 后 initPortalLang 不再用后端默认语言覆盖用户选择；
        // 为 false 时每次启动都接受后端默认语言（例如管理员调整了 country_language 顺序）。
        // 老用户 localStorage 的 storeConfig_v3 中无此字段时默认 false，向后兼容。
        const portalLangSet = ref(false)

        // 前台货币域（/api/* 与门户页）：全局一个前台默认货币（非 per-portal），
        // 默认货币 = 后端 country_currency 第一条（weigh DESC），由 /api/index/index 返回的
        // currency 数组经 initPortalCurrency 写入（后端并行线提供）；用户可在前台显式切换（setPortalCurrency）。
        // 与后台 country/currency 管理互相独立。currencyArray/currencyRates 是数据（供切换组件渲染与
        // formatMoney 换算），defaultCurrency 是用户选择，二者分开维护；缺省 CNY 兜底。
        const portalCurrency: PortalCurrency = reactive({
            defaultCurrency: 'CNY',
            currencySet: false,
            currencyArray: [] as CurrencyOption[],
            currencyRates: {} as Record<string, number>,
        })

        const crud: Crud = reactive({
            syncType: 'manual',
            syncedUpdate: 'yes',
            syncAutoPublic: 'no',
        })

        function menuWidth() {
            // 菜单折叠时基本宽度
            const menuCollapseBaseWidth = 64

            // 左分布局特有
            if (layout.layoutMode == 'LeftSplit') {
                const navTabs = useNavTabs()

                // 本布局带来的额外菜单宽度，主菜单宽度 80 + 次级菜单的左右内边距 16
                const modeMenuWidth = 96
                // 最终菜单宽度
                let leftSplitMenuWidth = layout.menuCollapse
                    ? menuCollapseBaseWidth + modeMenuWidth
                    : parseInt(layout.menuWidthLeftSplit.toString()) + modeMenuWidth

                // 无次级菜单，固定宽度
                if (!navTabs.state.childrenMenus.length) {
                    leftSplitMenuWidth = 80
                }

                // 小屏模式
                if (layout.shrink) {
                    return layout.menuCollapse ? 0 : `${leftSplitMenuWidth}px`
                }

                return `${leftSplitMenuWidth}px`
            }

            // 小屏模式
            if (layout.shrink) {
                return layout.menuCollapse ? 0 : `${layout.menuWidth}px`
            }

            // 菜单是否折叠
            return layout.menuCollapse ? `${menuCollapseBaseWidth}px` : `${layout.menuWidth}px`
        }

        function setLang(val: string) {
            lang.defaultLang = val
        }

        /**
         * 前台语言切换（由前台语言切换入口调用，如 editDefaultLang(lang, 'portal')）：
         * 标记为用户显式选择，此后 initPortalLang 不再用后端默认语言覆盖。
         */
        function setPortalLang(val: string) {
            portalLang.defaultLang = val
            portalLangSet.value = true
        }

        /**
         * 用后端默认语言初始化前台语言域（由 /api/index/index 返回的 default_language 字段驱动，
         * 后端并行线实现）。仅在用户尚未显式设置过前台语言时生效，避免覆盖用户选择。
         */
        function initPortalLang(val: string) {
            if (!val || portalLangSet.value) return
            portalLang.defaultLang = val
        }

        /**
         * 前台货币切换（由前台货币切换入口调用，如 currency-switch 组件 change 事件）：
         * 标记为用户显式选择，此后 initPortalCurrency 不再用后端默认货币覆盖。
         */
        function setPortalCurrency(val: string) {
            portalCurrency.defaultCurrency = val
            portalCurrency.currencySet = true
        }

        /**
         * 用后端返回的货币列表初始化前台货币域（由 /api/index/index 返回的 currency 数组驱动，
         * 数组按 weigh DESC 排序、第一项为后端默认货币；后端并行线实现）。
         * - 货币列表（currencyArray/currencyRates）是数据，始终以最新后端列表为准
         *   （用户已显式切换后仍更新，供切换组件渲染）；
         * - 默认货币是选择，仅在用户尚未显式设置过前台货币时写入，避免覆盖用户选择。
         */
        function initPortalCurrency(code: string, currencyArray: CurrencyOption[], currencyRates: Record<string, number>) {
            if (currencyArray.length > 0) {
                portalCurrency.currencyArray = currencyArray
            }
            if (currencyRates && Object.keys(currencyRates).length > 0) {
                portalCurrency.currencyRates = currencyRates
            }
            if (!code || portalCurrency.currencySet) return
            portalCurrency.defaultCurrency = code
        }

        function setLayoutMode(data: string) {
            layout.layoutMode = data
        }

        const setLayout = (name: keyof Layout, value: any) => {
            ;(layout[name] as any) = value
        }

        const getColorVal = function (name: keyof Layout): string {
            const colors = layout[name] as string[]
            if (layout.isDark) {
                return colors[1]
            } else {
                return colors[0]
            }
        }

        const setCrud = (name: keyof Crud, value: any) => {
            ;(crud[name] as any) = value
        }

        return {
            layout,
            lang,
            crud,
            portalLang,
            portalLangSet,
            portalCurrency,
            menuWidth,
            setLang,
            setPortalLang,
            initPortalLang,
            setPortalCurrency,
            initPortalCurrency,
            setLayoutMode,
            setLayout,
            getColorVal,
            setCrud,
        }
    },
    {
        persist: {
            key: STORE_CONFIG,
        },
    }
)
