<!--
 * lang-switch 前台门户语言切换组件
 *
 * 用途：给前台门户（web/src/views/frontend/ 及自定义门户）提供语言切换 UI。
 * 组件是纯展示与交互层：
 *   - 不读取/写入任何 store，不调用 editDefaultLang，不 location.reload
 *   - 用户选中语言后只 emit('change', name)，切换行为完全由父组件决定
 *     （父组件可调用 web/src/lang/index.ts 的 editDefaultLang 等）
 *
 * 语言列表由调用方通过 props.langArray 传入（与 stores/config.ts 的
 * lang.langArray 形状一致：{ name: locale 代码, value: 展示名 }），
 * 组件内部不硬编码任何语言。
 *
 * 样式与覆盖方式见同目录 lang-switch.scss；交互说明与使用示例见 README.md。
 -->
<template>
    <el-dropdown
        class="lang-switch"
        trigger="click"
        placement="bottom-end"
        :hide-timeout="50"
        :hide-on-click="true"
        :class="{ 'is-open': dropdownVisible }"
        @visible-change="dropdownVisible = $event"
        @command="onSelect"
    >
        <button type="button" class="lang-switch__trigger" :aria-label="labelText || currentValue">
            <!-- 地球图标（内联 SVG，无依赖） -->
            <svg class="lang-switch__globe" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                <circle cx="12" cy="12" r="9.5" />
                <path d="M2.5 12h19" />
                <path d="M12 2.5c2.8 2.6 4.2 5.9 4.2 9.5S14.8 18.9 12 21.5C9.2 18.9 7.8 15.6 7.8 12S9.2 5.1 12 2.5z" />
            </svg>
            <span v-if="labelText" class="lang-switch__label">{{ labelText }}</span>
            <span class="lang-switch__value">{{ currentValue }}</span>
            <!-- 移动端紧凑形态显示的缩写（如 ZH / EN） -->
            <span class="lang-switch__code">{{ currentCode }}</span>
            <!-- 下拉箭头（展开时翻转） -->
            <svg class="lang-switch__caret" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                <path d="M6 9.5l6 6 6-6" />
            </svg>
        </button>

        <template #dropdown>
            <el-dropdown-menu class="lang-switch__menu">
                <el-dropdown-item
                    v-for="item in langArray"
                    :key="item.name"
                    :command="item.name"
                    :class="{ 'is-current': item.name === current }"
                >
                    <span class="lang-switch__option">
                        <svg v-if="item.name === current" class="lang-switch__check" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                            <path d="M4.5 12.5l5 5 10-11" />
                        </svg>
                        {{ item.value }}
                    </span>
                </el-dropdown-item>
            </el-dropdown-menu>
        </template>
    </el-dropdown>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import './lang-switch.scss'

interface LangOption {
    /** 语言代码，如 zh-cn / en */
    name: string
    /** 展示名，如 中文简体 / English */
    value: string
}

const props = withDefaults(
    defineProps<{
        /** 语言列表，与 stores/config.ts 的 lang.langArray 形状一致，由调用方传入 */
        langArray: LangOption[]
        /** 当前语言（name），不在 langArray 中时原样显示 */
        current?: string
        /**
         * 触发器上可见的标签文字（如「语言」）。
         * 缺省使用 t('Language') 并回退为 'Language'；传空字符串 '' 可隐藏标签。
         */
        label?: string
    }>(),
    {
        current: 'zh-cn',
    }
)

const emit = defineEmits<{
    /**
     * 用户选中某个语言时触发，参数为所选语言 name（如 'en'）。
     * 组件不做任何切换动作，由父组件决定后续行为。
     */
    (e: 'change', name: string): void
}>()

const { t, te } = useI18n()

const dropdownVisible = ref(false)

/** 可见标签：props.label 优先；缺省取 i18n 的 Language 键，键不存在时直接显示 'Language' */
const labelText = computed(() => props.label ?? (te('Language') ? t('Language') : 'Language'))

/** 当前语言的展示名；不在列表中时直接显示语言代码 */
const currentValue = computed(() => props.langArray.find((item) => item.name === props.current)?.value ?? props.current)

/** 移动端紧凑形态：取语言代码第一段大写（zh-cn → ZH、en → EN） */
const currentCode = computed(() => (props.current.split('-')[0] || props.current).toUpperCase())

function onSelect(name: string) {
    emit('change', name)
}
</script>
