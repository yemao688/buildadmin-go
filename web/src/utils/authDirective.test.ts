import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { App, ObjectDirective } from 'vue'
import { adminDirectives } from '/@/utils/authDirective'
import { auth } from '/@/utils/auth'
import { directives } from '/@/utils/directives'

vi.mock('/@/utils/auth', () => ({ auth: vi.fn() }))
vi.mock('/@/utils/directives', () => ({ directives: vi.fn() }))

type MountedHandler = (el: HTMLElement, binding: { value?: any }) => void

// adminDirectives 把指令注册到 app 上：这里用 Map 兜住注册表，等价于真实 app.directive
function registerAuthDirective(): MountedHandler {
    const registry = new Map<string, ObjectDirective>()
    const app = {
        directive: (name: string, def: ObjectDirective) => registry.set(name, def)
    } as unknown as App

    adminDirectives(app)

    const directive = registry.get('auth')
    if (!directive) throw new Error('v-auth 未注册')
    return directive.mounted as MountedHandler
}

function createEl(): HTMLElement {
    const parent = document.createElement('div')
    const el = document.createElement('button')
    parent.appendChild(el)
    return el
}

describe('v-auth 指令', () => {
    // restoreMocks 只覆盖 spy，模块 mock 的 vi.fn 需要手动清空调用记录
    beforeEach(() => {
        vi.mocked(directives).mockClear()
        vi.mocked(auth).mockReset()
    })

    it('注册 auth 指令并先注册通用指令', () => {
        registerAuthDirective()
        expect(vi.mocked(directives)).toHaveBeenCalledTimes(1)
    })

    it('binding.value 为假值时保留元素且不鉴权', () => {
        const mounted = registerAuthDirective()
        const el = createEl()

        mounted(el, { value: '' })

        expect(el.parentElement).not.toBeNull()
        expect(vi.mocked(auth)).not.toHaveBeenCalled()
    })

    it('鉴权失败时把元素从父节点移除', () => {
        const mounted = registerAuthDirective()
        const el = createEl()
        vi.mocked(auth).mockReturnValue(false)

        mounted(el, { value: 'user.edit' })

        expect(auth).toHaveBeenCalledWith('user.edit')
        expect(el.parentElement).toBeNull()
    })

    it('鉴权通过时保留元素', () => {
        const mounted = registerAuthDirective()
        const el = createEl()
        const parent = el.parentElement
        vi.mocked(auth).mockReturnValue(true)

        mounted(el, { value: 'user.edit' })

        expect(el.parentElement).toBe(parent)
    })

    it('{ name, subNodeName } 对象形态原样传给 auth', () => {
        const mounted = registerAuthDirective()
        const el = createEl()
        const node = { name: 'user', subNodeName: 'edit' }
        vi.mocked(auth).mockReturnValue(true)

        mounted(el, { value: node })

        expect(vi.mocked(auth).mock.calls[0][0]).toBe(node)
        expect(el.parentElement).not.toBeNull()
    })
})
