import ElementPlus from 'element-plus'
import elementPlusCss from 'element-plus/dist/index.css?inline'
import displayCss from 'element-plus/theme-chalk/display.css?inline'

// 门户逃生口专用：整包引入 Element Plus（含完整样式）。
// CSS 以 ?inline 字符串随本模块加载，避免被并入入口 CSS；只有显式调用时才动态加载。
export { displayCss, elementPlusCss }
export default ElementPlus
