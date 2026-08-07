<!--
 * currency-switch 前台门户货币切换组件
 *
 * 用途：给前台门户（web/src/views/frontend/ 及自定义门户）提供货币切换 UI。
 * 组件是纯展示与交互层：
 *   - 不读取/写入任何 store，不调用接口，不 location.reload
 *   - 用户选中货币后只 emit('change', code)，切换行为完全由父组件决定
 *     （货币切换通常无需整页刷新，父组件决定是否仅更新展示/本地状态）
 *
 * 货币列表由调用方通过 props.currencyArray 传入（与 /api/index/index 返回的
 * currency 数组形状一致：{ code, name, symbol }），组件内部不硬编码任何货币。
 *
 * 样式与覆盖方式见同目录 currency-switch.scss；交互说明与使用示例见 README.md。
 -->
<template>
    <el-dropdown
        class="currency-switch"
        trigger="click"
        placement="bottom-end"
        :hide-timeout="50"
        :hide-on-click="true"
        :class="{ 'is-open': dropdownVisible }"
        @visible-change="dropdownVisible = $event"
        @command="onSelect"
    >
        <button type="button" class="currency-switch__trigger" :aria-label="labelText || currentValue">
            <!-- 货币符号（如 ¥ / $ / €，来自 currencyArray，无硬编码） -->
            <span v-if="currentSymbol" class="currency-switch__symbol">{{ currentSymbol }}</span>
            <span v-if="labelText" class="currency-switch__label">{{ labelText }}</span>
            <span class="currency-switch__value">{{ currentValue }}</span>
            <!-- 移动端紧凑形态显示的货币代码（如 CNY / USD） -->
            <span class="currency-switch__code">{{ currentCode }}</span>
            <!-- 下拉箭头（展开时翻转） -->
            <svg class="currency-switch__caret" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                <path d="M6 9.5l6 6 6-6" />
            </svg>
        </button>

        <template #dropdown>
            <el-dropdown-menu class="currency-switch__menu">
                <el-dropdown-item
                    v-for="item in currencyArray"
                    :key="item.code"
                    :command="item.code"
                    :class="{ 'is-current': item.code === current }"
                >
                    <span class="currency-switch__option">
                        <svg v-if="item.code === current" class="currency-switch__check" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                            <path d="M4.5 12.5l5 5 10-11" />
                        </svg>
                        <span class="currency-switch__symbol">{{ item.symbol }}</span>
                        {{ item.name }}
                    </span>
                </el-dropdown-item>
            </el-dropdown-menu>
        </template>
    </el-dropdown>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import './currency-switch.scss'

interface CurrencyOption {
    /** 货币代码，如 CNY / USD */
    code: string
    /** 展示名，如 人民币 / US Dollar */
    name: string
    /** 货币符号，如 ¥ / $ / € */
    symbol: string
}

const props = withDefaults(
    defineProps<{
        /** 货币列表，与 /api/index/index 返回的 currency 数组形状一致，由调用方传入 */
        currencyArray: CurrencyOption[]
        /** 当前货币（code），不在 currencyArray 中时原样显示 code */
        current?: string
        /**
         * 触发器上可见的标签文字（如「货币」）。
         * 缺省使用 t('Currency') 并回退为 'Currency'；传空字符串 '' 可隐藏标签。
         */
        label?: string
    }>(),
    {
        current: 'CNY',
    }
)

const emit = defineEmits<{
    /**
     * 用户选中某个货币时触发，参数为所选货币 code（如 'USD'）。
     * 组件不做任何切换动作（无需 reload），由父组件决定后续行为。
     */
    (e: 'change', code: string): void
}>()

const { t, te } = useI18n()

const dropdownVisible = ref(false)

/** 可见标签：props.label 优先；缺省取 i18n 的 Currency 键，键不存在时直接显示 'Currency' */
const labelText = computed(() => props.label ?? (te('Currency') ? t('Currency') : 'Currency'))

/** 当前货币的展示名；不在列表中时直接显示货币代码 */
const currentValue = computed(() => props.currencyArray.find((item) => item.code === props.current)?.name ?? props.current)

/** 当前货币的符号；不在列表中时为空（触发器不显示符号） */
const currentSymbol = computed(() => props.currencyArray.find((item) => item.code === props.current)?.symbol ?? '')

/** 移动端紧凑形态：货币代码（如 CNY / USD） */
const currentCode = computed(() => props.current.toUpperCase())

function onSelect(code: string) {
    emit('change', code)
}
</script>
