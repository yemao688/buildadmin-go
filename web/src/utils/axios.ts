import type { AxiosRequestConfig, Method } from 'axios'
import axios from 'axios'
import { ElLoading, ElNotification, type LoadingOptions } from 'element-plus'
import { refreshToken } from '/@/api/common'
import { i18n } from '/@/lang/index'
import router from '/@/router/index'
import adminBaseRoute from '/@/router/static/adminBase'
import { useConfig } from '/@/stores/config'
import { SYSTEM_ZINDEX } from '/@/stores/constant/common'
import { isAdminApp } from '/@/utils/common'
import {
    getTokenProvider,
    getTokenProviderByRefreshType,
    getTokenProviders,
    type TokenProvider,
} from '/@/utils/tokenProvider'

window.requests = []
window.tokenRefreshing = false
const pendingMap = new Map()
let userTokenRefreshing = false
const userRequests: Array<(token: string) => void> = []
const loadingInstance: LoadingInstance = {
    target: null,
    count: 0,
}

/**
 * 根据运行环境获取基础请求URL
 */
export const getUrl = (): string => {
    const value: string = import.meta.env.VITE_AXIOS_BASE_URL as string
    return value == 'getCurrentDomain' ? window.location.protocol + '//' + window.location.host : value
}

/**
 * 根据运行环境获取基础请求URL的端口
 */
export const getUrlPort = (): string => {
    const url = getUrl()
    return new URL(url).port
}

export function refreshTokenRequest(provider: TokenProvider, lang: string, refreshToken: string | undefined, token: string | undefined) {
    return axios
        .post(
            '/api/common/refreshToken',
            {
                refreshToken,
            },
            {
                baseURL: getUrl(),
                headers: {
                    'think-lang': lang,
                    server: true,
                    [provider.header]: token ?? '',
                },
                responseType: 'json',
            }
        )
        .then((response) => {
            if (response.data?.code !== 1 || !response.data.data?.token) {
                return Promise.reject(response.data)
            }
            return response.data.data.token as string
        })
}

/**
 * 创建`Axios`
 * 默认开启`reductDataFormat(简洁响应)`,返回类型为`ApiPromise`
 * 关闭`reductDataFormat`,返回类型则为`AxiosPromise`
 */
function createAxios<Data = any, T = ApiPromise<Data>>(axiosConfig: AxiosRequestConfig, options: Options = {}, loading: LoadingOptions = {}): T {
    const config = useConfig()

    const Axios = axios.create({
        baseURL: getUrl(),
        timeout: 1000 * 10,
        headers: {
            'think-lang': config.lang.defaultLang,
            server: true,
        },
        responseType: 'json',
    })

    // 自定义后台入口
    if (adminBaseRoute.path != '/admin' && isAdminApp() && /^\/admin\//.test(axiosConfig.url!)) {
        axiosConfig.url = axiosConfig.url!.replace(/^\/admin\//, adminBaseRoute.path + '.php/')
    }

    // 合并默认请求选项
    options = Object.assign(
        {
            cancelDuplicateRequest: true, // 是否开启取消重复请求, 默认为 true
            loading: false, // 是否开启loading层效果, 默认为false
            reductDataFormat: true, // 是否开启简洁的数据结构响应, 默认为true
            showErrorMessage: true, // 是否开启接口错误信息展示,默认为true
            showCodeMessage: true, // 是否开启code不为1时的信息提示, 默认为true
            showSuccessMessage: false, // 是否开启code为1时的信息提示, 默认为false
            anotherToken: '', // 当前请求使用另外的用户token
        },
        options
    )

    const isUserRequest = (url?: string) => !isAdminApp() && /^\/api\//.test(url ?? '')
    const getRequestProvider = (url?: string) => {
        if (options.tokenDomain) return getTokenProvider(options.tokenDomain)
        if (options.anotherToken) return getTokenProvider('baAccount')
        return getTokenProvider(isUserRequest(url) ? 'userInfo' : 'admin')
    }
    const getRefreshProvider = () => {
        if (options.tokenDomain) return getTokenProvider(options.tokenDomain)
        return getTokenProvider(options.anotherToken ? 'baAccount' : 'admin')
    }
    const getRefreshFailureProvider = (provider: TokenProvider) => {
        if (options.tokenDomain || options.anotherToken || isAdminApp()) return provider
        return getTokenProvider('baAccount')
    }
    const setProviderHeader = (headers: anyObj, provider: TokenProvider, token: string) => {
        headers[provider.header] = token
    }
    const retryTokenRequest = (requestConfig: AxiosRequestConfig, provider: TokenProvider) => {
        if (!userTokenRefreshing) {
            userTokenRefreshing = true
            const store = provider.store()
            const refresh = provider.refresher
                ? provider.refresher({
                    provider,
                    lang: config.lang.defaultLang,
                    refreshToken: store.getToken('refresh'),
                    token: store.getToken('auth'),
                })
                : refreshToken(provider.domain).then((res) => {
                    if (!res.data?.token) return Promise.reject(res)
                    return res.data.token as string
                })
            return refresh
                .then((token) => {
                    store.setToken(token, 'auth')
                    userTokenRefreshing = false
                    userRequests.forEach((callback) => callback(token))
                    userRequests.length = 0
                    return Axios(requestConfig)
                })
                .catch((err) => {
                    store.removeToken()
                    userTokenRefreshing = false
                    userRequests.forEach((callback) => callback(''))
                    userRequests.length = 0
                    return Promise.reject(err)
                })
                .finally(() => {
                    userTokenRefreshing = false
                })
        }

        return new Promise((resolve) => {
            userRequests.push((token) => {
                const headers = (requestConfig.headers ?? {}) as anyObj
                headers[provider.header] = token
                requestConfig.headers = headers
                resolve(Axios(requestConfig))
            })
        })
    }

    // 请求拦截
    Axios.interceptors.request.use(
        (config) => {
            removePending(config)
            options.cancelDuplicateRequest && addPending(config)
            // 创建loading实例
            if (options.loading) {
                loadingInstance.count++
                if (loadingInstance.count === 1) {
                    loadingInstance.target = ElLoading.service(loading)
                }
            }

            // 自动携带token
            if (config.headers) {
                const adminProvider = getTokenProvider('admin')
                const token = adminProvider.store().getToken()
                if (token) (config.headers as anyObj)[adminProvider.header] = token
                const userToken = options.anotherToken
                const requestProvider = getRequestProvider(config.url)
                if (userToken) {
                    (config.headers as anyObj)[requestProvider.header] = userToken
                } else if (requestProvider.domain != adminProvider.domain) {
                    const requestToken = requestProvider.store().getToken()
                    if (requestToken) (config.headers as anyObj)[requestProvider.header] = requestToken
                }
            }

            return config
        },
        (error) => {
            return Promise.reject(error)
        }
    )

    // 响应拦截
    Axios.interceptors.response.use(
        (response) => {
            removePending(response.config)
            options.loading && closeLoading(options) // 关闭loading

            if (response.config.responseType == 'json') {
                if (response.data && response.data.code !== 1) {
                    const requestProvider = getRequestProvider(response.config.url)
                    if (isUserRequest(response.config.url) && requestProvider.store().getToken() && (response.data.code == 401 || response.data.code == 409)) {
                        return retryTokenRequest(response.config, requestProvider)
                    }
                    if (response.data.code == 409) {
                        if (!window.tokenRefreshing) {
                            window.tokenRefreshing = true
                            const refreshProvider = getRefreshProvider()
                            return refreshToken(refreshProvider.domain)
                                .then((res) => {
                                    const responseProvider = getTokenProviderByRefreshType(res.data.type) ??
                                        (refreshProvider.refreshType ? undefined : refreshProvider)
                                    if (responseProvider) {
                                        responseProvider.store().setToken(res.data.token, 'auth')
                                        setProviderHeader(response.headers as anyObj, responseProvider, `${res.data.token}`)
                                        window.requests.forEach((cb) => cb(res.data.token, res.data.type))
                                    }
                                    window.requests = []
                                    return Axios(response.config)
                                })
                                .catch((err) => {
                                    const failureProvider = getRefreshFailureProvider(refreshProvider)
                                    failureProvider.store().removeToken()
                                    const loginRoute = failureProvider.loginRoute
                                    if (typeof loginRoute == 'function') {
                                        loginRoute()
                                        return Promise.reject(err)
                                    }
                                    if (loginRoute) {
                                        if (router.currentRoute.value.name != loginRoute) {
                                            router.push({ name: loginRoute })
                                            return Promise.reject(err)
                                        }
                                    }
                                    setProviderHeader(response.headers as anyObj, failureProvider, '')
                                    window.requests.forEach((cb) => cb('', failureProvider.refreshType ?? failureProvider.domain))
                                    window.requests = []
                                    return Axios(response.config)
                                })
                                .finally(() => {
                                    window.tokenRefreshing = false
                                })
                        } else {
                            const refreshProvider = getRefreshProvider()
                            return new Promise((resolve) => {
                                // 用函数形式将 resolve 存入，等待刷新后再执行
                                window.requests.push((token: string, type: string) => {
                                    const responseProvider = getTokenProviderByRefreshType(type) ?? refreshProvider
                                    setProviderHeader(response.headers as anyObj, responseProvider, `${token}`)
                                    resolve(Axios(response.config))
                                })
                            })
                        }
                    }
                    if (options.showCodeMessage) {
                        ElNotification({
                            type: 'error',
                            message: response.data.msg,
                            zIndex: SYSTEM_ZINDEX,
                        })
                    }
                    // 自动跳转到路由name或path
                    if (response.data.code == 302) {
                        router.push({ path: response.data.data.routePath ?? '', name: response.data.data.routeName ?? '' })
                    }
                    if (response.data.code == 303) {
                        let routerPath = adminBaseRoute.path

                        // 需要登录，清理 token，转到登录页
                        if (response.data.data.type == 'need login') {
                            const loginProvider = getRequestProvider(response.config.url)
                            loginProvider.store().removeToken()
                            const loginRoute = loginProvider.loginRoute
                            if (typeof loginRoute == 'function') {
                                loginRoute()
                            } else if (loginRoute) {
                                if (router.currentRoute.value.name != loginRoute) {
                                    router.push({ name: loginRoute })
                                }
                            } else {
                                routerPath += '/login'
                                router.push({ path: routerPath })
                            }
                        } else {
                            router.push({ path: routerPath })
                        }
                    }
                    // code不等于1, 页面then内的具体逻辑就不执行了
                    return Promise.reject(response.data)
                } else if (options.showSuccessMessage && response.data && response.data.code == 1) {
                    ElNotification({
                        message: response.data.msg ? response.data.msg : i18n.global.t('axios.Operation successful'),
                        type: 'success',
                        zIndex: SYSTEM_ZINDEX,
                    })
                }
            }

            return options.reductDataFormat ? response.data : response
        },
        (error) => {
            error.config && removePending(error.config)
            options.loading && closeLoading(options) // 关闭loading
            if (error.config && isUserRequest(error.config.url) && getRequestProvider(error.config.url).store().getToken() && [401, 409].includes(error.response?.status)) {
                return retryTokenRequest(error.config, getRequestProvider(error.config.url))
            }
            options.showErrorMessage && httpErrorStatusHandle(error) // 处理错误状态码
            return Promise.reject(error) // 错误继续返回给到具体页面
        }
    )
    return Axios(axiosConfig) as T
}

export default createAxios

/**
 * 处理异常
 * @param {*} error
 */
function httpErrorStatusHandle(error: any) {
    // 处理被取消的请求
    if (axios.isCancel(error)) return console.error(i18n.global.t('axios.Automatic cancellation due to duplicate request:') + error.message)
    let message = ''
    if (error && error.response) {
        switch (error.response.status) {
            case 302:
                message = i18n.global.t('axios.Interface redirected!')
                break
            case 400:
                message = i18n.global.t('axios.Incorrect parameter!')
                break
            case 401:
                message = i18n.global.t('axios.You do not have permission to operate!')
                break
            case 403:
                message = i18n.global.t('axios.You do not have permission to operate!')
                break
            case 404:
                message = i18n.global.t('axios.Error requesting address:') + error.response.config.url
                break
            case 408:
                message = i18n.global.t('axios.Request timed out!')
                break
            case 409:
                message = i18n.global.t('axios.The same data already exists in the system!')
                break
            case 500:
                message = i18n.global.t('axios.Server internal error!')
                break
            case 501:
                message = i18n.global.t('axios.Service not implemented!')
                break
            case 502:
                message = i18n.global.t('axios.Gateway error!')
                break
            case 503:
                message = i18n.global.t('axios.Service unavailable!')
                break
            case 504:
                message = i18n.global.t('axios.The service is temporarily unavailable Please try again later!')
                break
            case 505:
                message = i18n.global.t('axios.HTTP version is not supported!')
                break
            default:
                message = i18n.global.t('axios.Abnormal problem, please contact the website administrator!')
                break
        }
    }
    if (error.message.includes('timeout')) message = i18n.global.t('axios.Network request timeout!')
    if (error.message.includes('Network'))
        message = window.navigator.onLine ? i18n.global.t('axios.Server exception!') : i18n.global.t('axios.You are disconnected!')

    ElNotification({
        type: 'error',
        message,
        zIndex: SYSTEM_ZINDEX,
    })
}

/**
 * 关闭Loading层实例
 */
function closeLoading(options: Options) {
    if (options.loading && loadingInstance.count > 0) loadingInstance.count--
    if (loadingInstance.count === 0) {
        loadingInstance.target.close()
        loadingInstance.target = null
    }
}

/**
 * 储存每个请求的唯一cancel回调, 以此为标识
 */
function addPending(config: AxiosRequestConfig) {
    const pendingKey = getPendingKey(config)
    config.cancelToken =
        config.cancelToken ||
        new axios.CancelToken((cancel) => {
            if (!pendingMap.has(pendingKey)) {
                pendingMap.set(pendingKey, cancel)
            }
        })
}

/**
 * 删除重复的请求
 */
function removePending(config: AxiosRequestConfig) {
    const pendingKey = getPendingKey(config)
    if (pendingMap.has(pendingKey)) {
        const cancelToken = pendingMap.get(pendingKey)
        cancelToken(pendingKey)
        pendingMap.delete(pendingKey)
    }
}

/**
 * 生成每个请求的唯一key
 */
function getPendingKey(config: AxiosRequestConfig) {
    let { data } = config
    const { url, method, params, headers } = config
    if (typeof data === 'string') data = JSON.parse(data) // response里面返回的config.data是个字符串对象
    const tokenValues: string[] = []
    const tokenHeaders = new Set<string>()
    getTokenProviders().forEach((provider) => {
        if (tokenHeaders.has(provider.header)) return
        tokenHeaders.add(provider.header)
        tokenValues.push(headers && (headers as anyObj)[provider.header] ? (headers as anyObj)[provider.header] : '')
    })
    return [
        url,
        method,
        ...tokenValues,
        JSON.stringify(params),
        JSON.stringify(data),
    ].join('&')
}

/**
 * 根据请求方法组装请求数据/参数
 */
export function requestPayload(method: Method, data: anyObj) {
    if (method == 'GET') {
        return {
            params: data,
        }
    } else if (method == 'POST') {
        return {
            data: data,
        }
    }
}

interface LoadingInstance {
    target: any
    count: number
}
export interface Options {
    // 是否开启取消重复请求, 默认为 true
    cancelDuplicateRequest?: boolean
    // 是否开启loading层效果, 默认为false
    loading?: boolean
    // 是否开启简洁的数据结构响应, 默认为true
    reductDataFormat?: boolean
    // 是否开启接口错误信息展示,默认为true
    showErrorMessage?: boolean
    // 是否开启code不为1时的信息提示, 默认为true
    showCodeMessage?: boolean
    // 是否开启code为1时的信息提示, 默认为false
    showSuccessMessage?: boolean
    // 当前请求使用另外的用户token
    anotherToken?: string
    // 当前请求使用的token provider domain
    tokenDomain?: string
}

/*
 * 感谢掘金@橙某人提供的思路和分享
 * 本axios封装详细解释请参考：https://juejin.cn/post/6968630178163458084?share_token=7831c9e0-bea0-469e-8028-b587e13681a8#heading-27
 */
