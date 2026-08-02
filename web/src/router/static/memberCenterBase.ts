import { RouterView, type RouteRecordRaw } from 'vue-router'

/**
 * 会员中心基础路由路径
 */
export const frontendBaseRoutePath = '/user'
export const memberCenterBaseRoutePath = frontendBaseRoutePath

/*
 * 会员中心基础静态路由
 *
 * The portal layout is intentionally optional in the skeleton. RouterView
 * keeps this route available as a dynamic-route parent without restoring a
 * removed portal component.
 */
export const frontendBaseRoute: RouteRecordRaw = {
    path: frontendBaseRoutePath,
    name: 'user',
    component: RouterView,
    redirect: memberCenterBaseRoutePath + '/loading',
    meta: {
        title: `pagesTitle.user`,
    },
    children: [
        {
            path: 'loading/:to?',
            name: 'userMainLoading',
            component: () => import('/@/layouts/common/components/loading.vue'),
            meta: {
                title: `pagesTitle.loading`,
            },
        },
    ],
}

export const memberCenterBaseRoute = frontendBaseRoute
export default frontendBaseRoute
