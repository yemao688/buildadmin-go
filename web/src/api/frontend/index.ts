import createAxios from '/@/utils/axios'
import { isEmpty } from 'lodash-es'
import { useSiteConfig } from '/@/stores/siteConfig'
import { useUserInfo } from '/@/stores/userInfo'

export const indexUrl = '/api/index/'

/**
 * 前台初始化请求：获取全站配置（site）、登录态会员信息（userInfo）、
 * 语言与币种列表（language/currency）。
 *
 * 业务门户入口调用（买家端 /、卖家端 /seller 等各自接入）：
 * - site → siteConfig store
 * - userInfo 非空 → userInfo store
 *
 * @param callback 初始化完成回调（返回接口原始响应）
 */
export function initialize(callback?: (res: ApiResponse) => void) {
    const siteConfig = useSiteConfig()
    const userInfo = useUserInfo()

    createAxios({
        url: indexUrl + 'index',
        method: 'get',
    }).then((res) => {
        if (!isEmpty(res.data.site)) {
            siteConfig.$patch(res.data.site)
        }
        if (!isEmpty(res.data.userInfo)) {
            userInfo.dataFill(res.data.userInfo)
        }
        siteConfig.setInitialize(true)
        typeof callback == 'function' && callback(res)
    })
}
