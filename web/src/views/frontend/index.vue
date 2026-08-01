<template>
    <main class="frontend-home">
        <el-card class="welcome-card" shadow="never">
            <h1>{{ siteConfig.siteName }}</h1>
            <p>欢迎来到这里，前台会员门户正在准备中。</p>
            <el-button type="primary" @click="goToAdmin">进入后台</el-button>
        </el-card>
    </main>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useSiteConfig } from '/@/stores/siteConfig'

const router = useRouter()
const siteConfig = useSiteConfig()

const goToAdmin = () => {
    router.push('/admin')
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
    margin: 0;
    color: var(--el-text-color-primary);
    font-size: 28px;
}

p {
    margin: 16px 0 24px;
    color: var(--el-text-color-secondary);
}
</style>
