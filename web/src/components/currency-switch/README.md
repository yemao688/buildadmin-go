# currency-switch 前台门户货币切换组件

给**前台门户**（`web/src/views/frontend/` 及自定义门户，如 `frontend/seller/`）使用的货币切换器。纯展示与交互层：组件不读写 store、不调用接口、不 `location.reload`，选中货币后只触发 `change` 事件，切换行为完全由父组件决定（货币切换通常无需整页刷新，父组件只需更新本地展示状态即可）。

与 `lang-switch` 组件结构、接口风格与样式策略完全对称，可并排使用。

## 文件

| 文件 | 说明 |
|---|---|
| `index.vue` | 组件主体（el-dropdown + 内联 SVG，无额外依赖） |
| `currency-switch.scss` | 组件样式（全局样式表，类名统一 `currency-switch-` 前缀），含覆盖方式注释 |
| `README.md` | 本文档 |

## Props

| 名称 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `currencyArray` | `{ code: string; name: string; symbol: string }[]` | 必填 | 货币列表，与 `/api/index/index` 返回的 `currency` 数组形状一致，如 `[{ code: 'CNY', name: '人民币', symbol: '¥' }, { code: 'USD', name: 'US Dollar', symbol: '$' }]` |
| `current` | `string` | `'CNY'` | 当前货币（`code`）；不在 `currencyArray` 中时原样显示 `code`（触发器不显示符号） |
| `label` | `string` | — | 触发器上可见的标签文字（如「货币」）。缺省取 `t('Currency')`，i18n 键不存在时回退显示 `'Currency'`；传空字符串 `''` 可隐藏标签 |

## Emits

| 名称 | 参数 | 说明 |
|---|---|---|
| `change` | `code: string` | 用户选中某个货币时触发，参数为所选货币 `code`（如 `'USD'`）。父组件决定后续行为（例如更新当前货币展示状态、按币种换算价格） |

## 样式覆盖

默认是中性浅色样式（灰白文字 + 透明背景），适配常见门户页头/页脚；不依赖后台 design system 的任何变量或样式。

覆盖方式见 `currency-switch.scss` 顶部注释，主要有两种：

1. **CSS 变量（推荐）**——默认值定义在 `:root`，可在任意层级覆盖：

   ```scss
   .site-header .currency-switch {
       --currency-switch-text-color: #e5e7eb;   /* 文字颜色（深色页头） */
       --currency-switch-hover-bg: rgba(255, 255, 255, 0.12);
       --currency-switch-active-color: #60a5fa; /* 下拉中当前货币高亮 */
       --currency-switch-radius: 6px;
       --currency-switch-font-size: 14px;
       --currency-switch-gap: 6px;
   }
   ```

2. **直接覆盖类**——所有类名带 `currency-switch-` 前缀：`.currency-switch__trigger`、`.currency-switch__symbol`、`.currency-switch__label`、`.currency-switch__value`、`.currency-switch__menu` 等。

注意：下拉菜单（el-dropdown 的 popper）渲染在 `body` 下，不继承 `.currency-switch` 上的变量，只继承 `:root`；菜单部分需要定制时直接覆盖 `.currency-switch__menu .el-dropdown-menu__item`。

## 在门户中使用

与 `lang-switch` 并排放置示例（如门户页头右侧）：

```vue
<template>
    <header class="site-header">
        <nav class="site-header__nav"><!-- 门户导航 --></nav>
        <div class="site-header__switches">
            <LangSwitch
                :lang-array="langArray"
                :current="currentLang"
                :label="t('Language')"
                @change="onLangChange"
            />
            <CurrencySwitch
                :currency-array="currencyArray"
                :current="currentCurrency"
                :label="t('Currency')"
                @change="onCurrencyChange"
            />
        </div>
    </header>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import LangSwitch from '/@/components/lang-switch/index.vue'
import CurrencySwitch from '/@/components/currency-switch/index.vue'
import { useI18n } from 'vue-i18n'
import { useConfig } from '/@/stores/config'

const { t } = useI18n()
const config = useConfig()

// 语言列表来自 config store（/api/index/index 的 language 数组）
const langArray = config.lang.langArray
const currentLang = ref(config.portalLang.defaultLang)

// 货币列表来自 /api/index/index 的 currency 数组（由并行线在 config store 中提供，
// 示例仅示意字段名，以实际 store 为准）；currentCurrency 初始可取默认币种
const currencyArray = config.currencyArray
const currentCurrency = ref('CNY')

function onLangChange(name: string) {
    currentLang.value = name
    // 语言切换需要整页刷新（门户域用 editDefaultLang(name, 'portal')）
}

function onCurrencyChange(code: string) {
    currentCurrency.value = code
    // 货币切换无需 reload：父组件按币种更新价格展示/本地状态即可
}
</script>

<style scoped>
.site-header__switches {
    display: flex;
    align-items: center;
    gap: 4px;
}
</style>
```

## 响应式

窗口宽度 ≤ 767px 时自动进入紧凑形态：隐藏「货币」标签与当前货币全名，只显示货币符号 + 货币代码（`人民币 → ¥ CNY`）。如需调整断点，覆盖 `currency-switch.scss` 中的 `@media (max-width: 767px)` 即可。
