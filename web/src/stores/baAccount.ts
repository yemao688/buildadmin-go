import { defineStore } from 'pinia'
import router from '../router'
import { baAccountLogout } from '/@/api/backend/index'
import { BA_ACCOUNT } from '/@/stores/constant/cacheKey'
import type { AdminInfo } from '/@/stores/interface'
import { Local } from '/@/utils/storage'

export const useBaAccount = defineStore('baAccount', {
    state: (): Partial<AdminInfo> => {
        return {
            id: 0,
            username: '',
            nickname: '',
            avatar: '',
            token: '',
            refresh_token: '',
        }
    },
    actions: {
        /**
         * 状态批量填充
         * @param state 新状态数据
         * @param [exclude=true] 是否排除某些字段（忽略填充），默认值 true 排除 token 和 refresh_token，传递 false 则不排除，还可传递 string[] 指定排除字段列表
         */
        dataFill(state: Partial<AdminInfo>, exclude: boolean | string[] = true) {
            if (exclude === true) {
                exclude = ['token', 'refresh_token']
            } else if (exclude === false) {
                exclude = []
            }

            if (Array.isArray(exclude)) {
                exclude.forEach((item) => {
                    delete state[item as keyof AdminInfo]
                })
            }

            this.$patch(state)
        },
        removeToken() {
            this.token = ''
            this.refresh_token = ''
        },
        setToken(token: string, type: 'auth' | 'refresh') {
            const field = type == 'auth' ? 'token' : 'refresh_token'
            this[field] = token
        },
        getToken(type: 'auth' | 'refresh' = 'auth') {
            return type === 'auth' ? this.token : this.refresh_token
        },
        logout() {
            baAccountLogout().then((res) => {
                if (res.code == 1) {
                    Local.remove(BA_ACCOUNT)
                    router.go(0)
                }
            })
        },
    },
    persist: {
        key: BA_ACCOUNT,
    },
})
