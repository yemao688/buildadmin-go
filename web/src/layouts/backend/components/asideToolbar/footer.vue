<template>
    <div>
        <div
            class="aside-footer-toolbar-wrap"
            :class="[config.layout.menuCollapse ? 'collapse' : '', config.layout.menuToolBarAutoHide ? 'auto-hide' : '']"
        >
            <div class="aside-footer-toolbar">
                <Icon
                    @click="onMenuCollapse"
                    :name="config.layout.menuCollapse ? 'fa fa-indent' : 'fa fa-dedent'"
                    :class="config.layout.menuCollapse ? 'unfold' : ''"
                    :color="config.getColorVal('menuToolBarColor')"
                    size="14"
                    class="footer-toolbar-item"
                />
                <Icon
                    @click="onMenuSearch"
                    name="fa fa-search"
                    :color="config.getColorVal('menuToolBarColor')"
                    size="16"
                    class="footer-toolbar-item"
                />
            </div>
        </div>

        <MenuSearchDialog v-model="menuSearchDialogVisible" />
    </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import MenuSearchDialog from '/@/layouts/backend/components/asideToolbar/menuSearch/dialog.vue'
import { useConfig } from '/@/stores/config'
import { BEFORE_RESIZE_LAYOUT } from '/@/stores/constant/cacheKey'
import { setNavTabsWidth } from '/@/utils/layout'
import { closeShade } from '/@/utils/pageShade'
import { Session } from '/@/utils/storage'

const config = useConfig()
const menuSearchDialogVisible = ref(false)

const onMenuSearch = function () {
    menuSearchDialogVisible.value = true
}

const onMenuCollapse = function () {
    if (config.layout.shrink && !config.layout.menuCollapse) {
        closeShade()
    }

    config.setLayout('menuCollapse', !config.layout.menuCollapse)

    Session.set(BEFORE_RESIZE_LAYOUT, {
        layoutMode: config.layout.layoutMode,
        menuCollapse: config.layout.menuCollapse,
    })

    // 等待侧边栏动画结束后重新计算导航栏宽度
    setTimeout(() => {
        setNavTabsWidth()
    }, 350)
}
</script>

<style scoped lang="scss">
.aside-footer-toolbar-wrap {
    position: relative;
    height: 50px;
    background-color: v-bind('config.getColorVal("menuBackground")');
    .aside-footer-toolbar {
        position: absolute;
        display: flex;
        align-items: center;
        justify-content: space-between;
        height: 50px;
        width: 100%;
        padding: 0 20px;
        transition: all 0.2s ease;
        .footer-toolbar-item {
            padding: 10px;
            border-radius: 50%;
            cursor: pointer;
            &:hover {
                color: v-bind('config.getColorVal("menuToolBarHoverColor")') !important;
                background-color: v-bind('config.getColorVal("menuToolBarHoverBackground")');
            }
        }
    }
    &.collapse {
        height: 100px;
        .aside-footer-toolbar {
            flex-direction: column-reverse;
            padding: 10px 0;
            height: 100px;
        }
    }
    &.auto-hide.collapse {
        .aside-footer-toolbar {
            top: 100px;
        }
    }
    &.auto-hide {
        cursor: pointer;
        .aside-footer-toolbar {
            top: 50px;
        }
        &:hover {
            .aside-footer-toolbar {
                top: 0;
            }
        }
    }
}
</style>
