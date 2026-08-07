<template>
    <main class="frontend-home">
        <el-card class="welcome-card" shadow="never">
            <h1>{{ t('Welcome') }}</h1>
            <p class="current-lang">{{ t('Current language') }}: {{ currentLang }}</p>
            <LangSwitch :lang-array="langArray" :current="currentLang" @change="onLangChange" />
        </el-card>
    </main>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import LangSwitch from '/@/components/lang-switch/index.vue'
import { editDefaultLang } from '/@/lang/index'
import { useConfig } from '/@/stores/config'

const { t } = useI18n()
const config = useConfig()
const langArray = config.lang.langArray

// 门户域当前语言：初始值取 portalLang（后端 default_language / 用户前台选择），
// 切换后 editDefaultLang(name, 'portal') 写入 portalLang 并整页刷新
const currentLang = ref(config.portalLang.defaultLang)

function onLangChange(name: string) {
    currentLang.value = name
    // 门户域切换；后台切换用 editDefaultLang(name)（admin 域默认）
    editDefaultLang(name, 'portal')
}
</script>

<style scoped lang="scss">
.frontend-home {
    min-height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 24px;
    box-sizing: border-box;
    background-color: var(--el-bg-color-page);
}

.welcome-card {
    width: min(100%, 520px);
    text-align: center;
}

h1 {
    margin: 0 0 8px;
    color: var(--el-text-color-primary);
    font-size: 28px;
}

.current-lang {
    margin: 0 0 20px;
    color: var(--el-text-color-secondary);
    font-size: 14px;
}
</style>
