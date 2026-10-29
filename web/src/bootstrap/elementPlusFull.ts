import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import 'element-plus/theme-chalk/display.css'

// 门户逃生口专用：整包引入 Element Plus（含完整样式）。
// 独立成模块，保证只有显式调用时才被动态加载，不进入门户首屏静态图。
export default ElementPlus
