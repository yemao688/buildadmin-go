// element-plus 的按组件样式入口（es/components/<name>/style/css）只发 .mjs，不随类型声明导出。
// 这里逐个声明门户白名单实际用到的路径（不用通配，避免样式路径拼写错误被 typecheck 放过）。
declare module 'element-plus/es/components/button/style/css'
declare module 'element-plus/es/components/button-group/style/css'
declare module 'element-plus/es/components/card/style/css'
declare module 'element-plus/es/components/config-provider/style/css'
declare module 'element-plus/es/components/dropdown/style/css'
declare module 'element-plus/es/components/dropdown-menu/style/css'
declare module 'element-plus/es/components/dropdown-item/style/css'
declare module 'element-plus/es/components/icon/style/css'
declare module 'element-plus/es/components/loading/style/css'
declare module 'element-plus/es/components/notification/style/css'
