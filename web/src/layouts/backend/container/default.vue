<template>
    <el-container class="layout-container">
        <Aside />
        <el-container class="content-wrapper">
            <Header />
            <Main />
        </el-container>
        <CloseFullScreen v-if="navTabs.state.tabFullScreen" />

        <el-tour v-model="config.layout.layoutTour" :gap="{ offset: 0, radius: 2 }" @close="onTourClose('layoutTourUnfinished')">
            <el-tour-step
                placement="bottom"
                target=".nav-tabs .ba-nav-tab.active"
                :title="t('layouts.Context menu')"
                :description="t('layouts.Context menu tips')"
            />
            <el-tour-step
                placement="left-end"
                target=".ba-layout-config-btn"
                :title="t('layouts.Layout configuration')"
                :description="t('layouts.Layout configuration tips')"
            />
            <el-tour-step
                placement="right-start"
                target=".aside-footer-toolbar-wrap .fa-search"
                :title="t('layouts.Menu Search')"
                :description="t('layouts.Menu Search tips')"
            />
        </el-tour>
    </el-container>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Aside from '/@/layouts/backend/components/aside.vue'
import CloseFullScreen from '/@/layouts/backend/components/closeFullScreen.vue'
import Header from '/@/layouts/backend/components/header.vue'
import Main from '/@/layouts/backend/router-view/main.vue'
import { useConfig } from '/@/stores/config'
import { Layout } from '/@/stores/interface'
import { useNavTabs } from '/@/stores/navTabs'

const { t } = useI18n()
const config = useConfig()
const navTabs = useNavTabs()

const onTourClose = (key: keyof Layout) => {
    config.setLayout(key, false)
}
</script>

<style scoped>
.layout-container {
    height: 100%;
    width: 100%;
}
.content-wrapper {
    flex-direction: column;
    width: 100%;
    height: 100%;
}
</style>
