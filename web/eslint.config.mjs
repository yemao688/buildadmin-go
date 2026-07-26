// ESLint flat config — 对齐 PHP 上游 BuildAdmin web/.eslintrc.js（v2.3.7）规则集。
// 上游 legacy 配置（eslint 8 + @typescript-eslint v5 + eslint-plugin-vue v9）在此平移到
// ESLint 10 flat config；已删除/改名的规则在对应行内注明。
import globals from 'globals'
import tseslint from 'typescript-eslint'
import pluginVue from 'eslint-plugin-vue'
import eslintConfigPrettier from 'eslint-config-prettier'

const tsFiles = ['**/*.{ts,mts,cts,tsx,vue}']
const jsFiles = ['**/*.{js,mjs,cjs,jsx}']

export default tseslint.config(
    {
        name: 'buildadmin/ignores',
        ignores: ['node_modules/**', 'dist/**', 'public/**', 'types/**'],
    },

    // 上游 extends: plugin:@typescript-eslint/recommended
    ...tseslint.configs.recommended,

    // 上游 extends: plugin:vue/vue3-essential + plugin:vue/vue3-recommended
    // （flat/recommended 已内含 essential → strongly-recommended → recommended 整条链）
    ...pluginVue.configs['flat/recommended'],

    // 上游 extends: prettier（关闭与 Prettier 冲突的格式化规则）
    eslintConfigPrettier,

    {
        name: 'buildadmin/globals',
        languageOptions: {
            sourceType: 'module',
            // 上游 env: browser + es2021 + node；globals: { window, NodeJS }
            globals: {
                ...globals.browser,
                ...globals.node,
            },
        },
    },

    {
        // 上游 parser: vue-eslint-parser + parserOptions.parser: @typescript-eslint/parser
        name: 'buildadmin/vue-ts-parser',
        files: ['**/*.vue'],
        plugins: { '@typescript-eslint': tseslint.plugin },
        languageOptions: {
            parserOptions: {
                parser: tseslint.parser,
            },
        },
    },

    {
        name: 'buildadmin/core-rules',
        rules: {
            'no-useless-escape': 'off',
            'no-sparse-arrays': 'off',
            'no-prototype-builtins': 'off',
            'no-use-before-define': 'off',
            'no-case-declarations': 'off',
            'no-console': 'off',
            indent: ['warn', 4, { SwitchCase: 1 }],
        },
    },

    {
        name: 'buildadmin/ts-rules',
        files: tsFiles,
        plugins: { '@typescript-eslint': tseslint.plugin },
        rules: {
            '@typescript-eslint/no-empty-function': 'off',
            '@typescript-eslint/no-explicit-any': 'off',
            '@typescript-eslint/no-non-null-assertion': 'off',
            // 上游 ban-types 在 v8 已移除，拆分为 no-banned-types / no-unsafe-function-type
            // （上游 ban-types: off 当时覆盖了 Function 类型，故一并关闭）
            '@typescript-eslint/no-banned-types': 'off',
            '@typescript-eslint/no-unsafe-function-type': 'off',
            // 上游 ban-ts-ignore 在 v8 已并入 ban-ts-comment
            '@typescript-eslint/ban-ts-comment': 'off',
            // v8 recommended 新增，上游 v5 recommended 未启用
            '@typescript-eslint/no-unused-expressions': 'off',
            '@typescript-eslint/no-unused-vars': [
                'warn',
                {
                    argsIgnorePattern: '^_',
                    varsIgnorePattern: '^_',
                    // 上游 v5 默认不检查 catch 参数，v8 默认检查，调回上游行为
                    caughtErrors: 'none',
                },
            ],
        },
    },

    {
        // 上游对 js 文件同时启用核心 no-unused-vars；ts/vue 文件走 @typescript-eslint 版本避免重复报告
        name: 'buildadmin/js-rules',
        files: jsFiles,
        rules: {
            'no-unused-vars': [
                'warn',
                {
                    argsIgnorePattern: '^_',
                    varsIgnorePattern: '^_',
                },
            ],
        },
    },

    {
        // 插件的 flat/recommended 规则块不限定 .vue 文件（会命中 defineComponent 所在的 ts 文件），
        // 与上游 legacy 行为一致：这些 off 全局生效
        name: 'buildadmin/vue-rules',
        plugins: { vue: pluginVue },
        rules: {
            'vue/v-on-event-hyphenation': 'off',
            'vue/custom-event-name-casing': 'off',
            'vue/component-definition-name-casing': 'off',
            'vue/attributes-order': 'off',
            'vue/one-component-per-file': 'off',
            'vue/html-closing-bracket-newline': 'off',
            'vue/max-attributes-per-line': 'off',
            'vue/multiline-html-element-content-newline': 'off',
            'vue/singleline-html-element-content-newline': 'off',
            'vue/attribute-hyphenation': 'off',
            'vue/html-self-closing': 'off',
            'vue/require-default-prop': 'off',
            'vue/no-arrow-functions-in-watch': 'off',
            'vue/no-v-html': 'off',
            'vue/comment-directive': 'off',
            'vue/multi-word-component-names': 'off',
            'vue/require-prop-types': 'off',
            'vue/html-indent': 'off',
        },
    },
)
