import { defineStore } from 'pinia'
import { reactive } from 'vue'
import type { UseLanguageTabs, langItem } from '/@/stores/interface/index'

export const useLanguageTabs = defineStore(
    'languageTab',
    () => {
        const state: UseLanguageTabs = reactive({
            langList: [],
        })

        const setLang = (item: langItem[]) => {
            state.langList = item
        }

        return {
            state,
            setLang,
        }
    },
    {
        persist: true,
    }
)