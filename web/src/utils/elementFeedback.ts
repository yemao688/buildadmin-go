import type { LoadingOptions } from 'element-plus'

/**
 * Element Plus 反馈类能力（Loading / Notification）的按需加载门面。
 * axios 与 api/common 位于门户静态导入图，不能静态引入 element-plus，
 * 否则整个组件库会重新进入首屏；这些能力只在请求 loading 与会话提示时用到，
 * 动态导入把它们的代价推迟到真正触发时。
 */

/**
 * 打开全屏 Loading。打开是异步的（等待组件库 chunk），
 * isStillNeeded 用于在 chunk 到达时确认请求还没结束，避免闪一下空 Loading。
 * @returns 已打开时返回关闭句柄，请求已结束时返回 null
 */
export async function openLoading(options: LoadingOptions, isStillNeeded: () => boolean): Promise<{ close: () => void } | null> {
    const [loading] = await Promise.all([
        import('element-plus/es/components/loading/index'),
        import('element-plus/es/components/loading/style/css'),
    ])
    return isStillNeeded() ? loading.ElLoading.service(options) : null
}

/**
 * 弹出通知提示（错误 / 成功 / 业务码提示）；调用方无需等待
 */
export function notify(options: anyObj): void {
    void Promise.all([import('element-plus/es/components/notification/index'), import('element-plus/es/components/notification/style/css')]).then(
        ([notification]) => {
            notification.ElNotification(options)
        }
    )
}
