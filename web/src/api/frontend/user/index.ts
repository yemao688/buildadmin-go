import createAxios from '/@/utils/axios'
import { useUserInfo } from '/@/stores/userInfo'

export const userUrl = '/api/user/'

export function postLogin(params: anyObj = {}) {
    return createAxios({
        url: userUrl + 'login',
        method: 'POST',
        data: params,
    })
}

export function postRegister(params: anyObj = {}) {
    return createAxios({
        url: userUrl + 'register',
        method: 'POST',
        data: params,
    })
}

export function postLogout() {
    const userInfo = useUserInfo()
    return createAxios({
        url: userUrl + 'logout',
        method: 'POST',
        data: {
            refreshToken: userInfo.getToken('refresh'),
        },
    })
}
