<script lang="ts">
import { createVNode, defineAsyncComponent, getCurrentInstance, defineComponent, computed, type Component, type CSSProperties } from 'vue'
import Svg from '/@/components/icon/svg/index.vue'
import { isExternal } from '/@/utils/common'

// 已解析过的 element 图标缓存，避免每次渲染都创建新的异步组件
const elementIconCache = new Map<string, Component>()

/**
 * 按需解析 Element Plus 图标。
 * 后台域由 registerElementPlusIcons 全局注册，直接命中 appContext；
 * 门户域不注册图标（barrel 约 490KB），首次用到才动态导入。
 */
function resolveElementIcon(name: string): Component {
    const cached = elementIconCache.get(name)
    if (cached) return cached
    const icon = defineAsyncComponent(async () => {
        const icons = (await import('@element-plus/icons-vue')) as any
        const component = icons[name.replace(/^el-icon-/, '')]
        if (!component) throw new Error(`[Icon] 未知的 Element Plus 图标: ${name}`)
        return component
    })
    elementIconCache.set(name, icon)
    return icon
}

export default defineComponent({
    name: 'Icon',
    props: {
        name: {
            type: String,
            required: true,
        },
        size: {
            type: String,
            default: '18px',
        },
        color: {
            type: String,
            default: '#000000',
        },
    },
    setup(props) {
        const iconStyle = computed((): CSSProperties => {
            const { size, color } = props
            let s = `${size.replace('px', '')}px`
            return {
                fontSize: s,
                color: color,
            }
        })

        if (props.name.indexOf('el-icon-') === 0) {
            const registered = getCurrentInstance()?.appContext.components[props.name] as Component | undefined
            const elementIcon = registered ?? resolveElementIcon(props.name)
            return () => createVNode('el-icon', { class: 'icon el-icon', style: iconStyle.value }, [createVNode(elementIcon)])
        } else if (props.name.indexOf('local-') === 0 || isExternal(props.name)) {
            return () => createVNode(Svg, { name: props.name, size: props.size, color: props.color })
        } else {
            return () => createVNode('i', { class: [props.name, 'icon'], style: iconStyle.value })
        }
    },
})
</script>
