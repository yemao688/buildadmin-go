# lang-switch 前台门户语言切换组件

给**前台门户**（`web/src/views/frontend/` 及自定义门户，如 `frontend/seller/`）使用的语言切换器。纯展示与交互层：组件不读写 store、不调用 `editDefaultLang`、不 `location.reload`，选中语言后只触发 `change` 事件，切换行为完全由父组件决定。

## 文件

| 文件 | 说明 |
|---|---|
| `index.vue` | 组件主体（el-dropdown + 内联 SVG 图标，无额外依赖） |
| `lang-switch.scss` | 组件样式（全局样式表，类名统一 `lang-switch-` 前缀），含覆盖方式注释 |
| `README.md` | 本文档 |

## Props

| 名称 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `langArray` | `{ name: string; value: string }[]` | 必填 | 语言列表，与 `web/src/stores/config.ts` 的 `lang.langArray` 形状一致，如 `[{ name: 'zh-cn', value: '中文简体' }, { name: 'en', value: 'English' }]` |
| `current` | `string` | `'zh-cn'` | 当前语言（`name`）；不在 `langArray` 中时原样显示 |
| `label` | `string` | — | 触发器上可见的标签文字（如「语言」）。缺省取 `t('Language')`，i18n 键不存在时回退显示 `'Language'`；传空字符串 `''` 可隐藏标签 |

## Emits

| 名称 | 参数 | 说明 |
|---|---|---|
| `change` | `name: string` | 用户选中某个语言时触发，参数为所选语言 `name`（如 `'en'`）。父组件决定后续行为（例如调用并行线提供的 `editDefaultLang(lang, 'portal')`） |

## 样式覆盖

默认是中性浅色样式（灰白文字 + 透明背景），适配常见门户页头/页脚；不依赖后台 design system 的任何变量或样式。

覆盖方式见 `lang-switch.scss` 顶部注释，主要有两种：

1. **CSS 变量（推荐）**——默认值定义在 `:root`，可在任意层级覆盖：

   ```scss
   .site-header .lang-switch {
       --lang-switch-text-color: #e5e7eb;   /* 文字颜色（深色页头） */
       --lang-switch-hover-bg: rgba(255, 255, 255, 0.12);
       --lang-switch-active-color: #60a5fa; /* 下拉中当前语言高亮 */
       --lang-switch-radius: 6px;
       --lang-switch-font-size: 14px;
       --lang-switch-gap: 6px;
   }
   ```

2. **直接覆盖类**——所有类名带 `lang-switch-` 前缀：`.lang-switch__trigger`、`.lang-switch__label`、`.lang-switch__value`、`.lang-switch__menu` 等。

注意：下拉菜单（el-dropdown 的 popper）渲染在 `body` 下，不继承 `.lang-switch` 上的变量，只继承 `:root`；菜单部分需要定制时直接覆盖 `.lang-switch__menu .el-dropdown-menu__item`。

## 在门户中使用

```vue
<template>
    <header class="site-header">
        <LangSwitch
            :lang-array="langArray"
            :current="current"
            :label="t('Language')"
            @change="onLangChange"
        />
    </header>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import LangSwitch from '/@/components/lang-switch/index.vue'
import { useI18n } from 'vue-i18n'
import { useConfig } from '/@/stores/config'
import { editDefaultLang } from '/@/lang/index'

const { t } = useI18n()
const config = useConfig()
const langArray = config.lang.langArray
// 门户域当前语言取 portalLang（默认来自后端 /api/index/index 的 default_language，
// 经 main.ts 的 initPortalLang 写入；用户在前台显式切换后不再被后端默认语言覆盖）
const current = ref(config.portalLang.defaultLang)

function onLangChange(name: string) {
    current.value = name
    // 门户域切换：写入 portalLang 并整页刷新；需要保留门户（不刷新整页）时可自行实现局部重载逻辑。
    // 后台（/admin/*）切换请用 editDefaultLang(name)，admin 域为默认参数
    editDefaultLang(name, 'portal')
}
</script>
```

## 响应式

窗口宽度 ≤ 767px 时自动进入紧凑形态：隐藏「语言」标签与当前语言全名，只显示地球图标 + 语言缩写（`zh-cn → ZH`、`en → EN`）。如需调整断点，覆盖 `lang-switch.scss` 中的 `@media (max-width: 767px)` 即可。
