import { defineStore } from 'pinia'
import router from '../router'
import { postLogout } from '/@/api/frontend/user/index'
import { USER_INFO } from '/@/stores/constant/cacheKey'
import type { UserInfo } from '/@/stores/interface'
import { Local } from '/@/utils/storage'

export const useUserInfo = defineStore('userInfo', {
    state: (): UserInfo => {
        return {
            id: 0,
            username: '',
            nickname: '',
            avatar: '',
            email: '',
            mobile: '',
            money: '0',
            last_login_time: '',
            last_login_ip: '',
            join_time: '',
            token: '',
            refresh_token: '',
        }
    },
    actions: {
        dataFill(state: Partial<UserInfo>, exclude: boolean | string[] = true) {
            if (exclude === true) {
                exclude = ['token', 'refresh_token']
            } else if (exclude === false) {
                exclude = []
            }

            if (Array.isArray(exclude)) {
                exclude.forEach((item) => {
                    delete state[item as keyof UserInfo]
                })
            }

            this.$patch(state)
        },
        setToken(token: string, type: 'auth' | 'refresh') {
            const field = type == 'auth' ? 'token' : 'refresh_token'
            this[field] = token
        },
        getToken(type: 'auth' | 'refresh' = 'auth') {
            return type === 'auth' ? this.token : this.refresh_token
        },
        removeToken() {
            this.token = ''
            this.refresh_token = ''
        },
        isLogin() {
            return !!this.id && !!this.token
        },
        logout() {
            postLogout().then((res) => {
                if (res.code == 1) {
                    Local.remove(USER_INFO)
                    router.go(0)
                }
            })
        },
    },
    persist: {
        key: USER_INFO,
    },
})
