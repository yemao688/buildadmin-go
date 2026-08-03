<template>
    <main class="frontend-home">
        <el-card class="welcome-card" shadow="never">
            <h1>Welcome</h1>
        </el-card>
    </main>
</template>

<script setup lang="ts"></script>

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
</style>
