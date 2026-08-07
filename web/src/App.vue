<template>
    <el-config-provider :value-on-clear="() => null" :locale="lang">
        <router-view></router-view>
    </el-config-provider>
</template>
<script setup lang="ts">
import { onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { useConfig } from '/@/stores/config'
import { isAdminApp, setTitleFromRoute } from '/@/utils/common'
import iconfontInit from '/@/utils/iconfont'
import { init as viteInit } from '/@/utils/vite'
// modules import mark, Please do not remove.

const route = useRoute()
const config = useConfig()

// 初始化 element 的语言包
// 按应用域选择语言（与 lang/index.ts 的 loadLang 同规则）：
// 后台（admin 应用）用后台域语言 config.lang.defaultLang，门户应用用前台域语言
// config.portalLang.defaultLang，使 element-plus 内建文案（日期选择器/分页/表格等）跟随当前域语言。
// getLocaleMessage 取回 vue-i18n 实例中该 locale 的合并消息
// （loadLang 已将 element-plus locale 以 assignLocale 并入），供 el-config-provider 注入。
const { getLocaleMessage } = useI18n()
const lang = getLocaleMessage(isAdminApp() ? config.lang.defaultLang : config.portalLang.defaultLang) as any
onMounted(() => {
    viteInit()
    iconfontInit()

    // Modules onMounted mark, Please do not remove.
})

// 监听路由变化时更新浏览器标题
watch(
    () => route.path,
    () => {
        setTitleFromRoute()
    }
)
</script>
