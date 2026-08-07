/*
 * 门户域公共语言包（路由前缀 '/' 及无前缀门户页面按需加载，路径 ./frontend/<locale>.ts）
 * 新增语言：在 web/src/lang/<locale>/ 建目录/文件后无需修改任何 glob 分支（全量 glob 自动发现）
 * 覆盖风险：请避免使用页面语言包的目录名、文件名作为翻译 key、请使用大写开头避免覆盖
 */
export default {
    Welcome: '欢迎',
    Language: '语言',
    'Current language': '当前语言',
}
