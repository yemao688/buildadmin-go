/*
 * 框架占位首页
 * 业务添加自己的门户后，删除本文件即可接管 `/` 路由。
 */
import type { RouteRecordRaw } from 'vue-router'

const pageTitle = (name: string): string => {
    return `pagesTitle.${name}`
}

const homePlaceholder: RouteRecordRaw = {
    path: '/',
    name: 'home',
    component: () => import('/@/views/frontend/index.vue'),
    meta: {
        title: pageTitle('home'),
    },
}

export default homePlaceholder
