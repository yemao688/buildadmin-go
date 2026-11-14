import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { findIndexRow, getCellValue, invokeTableContextDataFun } from '/@/components/table/index'
import ColorRender from '/@/components/table/fieldRender/color.vue'

// 表格工具只依赖 i18n 的 t()（defaultOptButtons 用），屏蔽真实 lang 入口可避免
// 把 router/axios/element-plus locale 整条首屏链路拉进单测
vi.mock('/@/lang/index', () => ({
    i18n: { global: { t: (key: string) => key } }
}))

const column = {} as any

const cell = (row: TableRow, field: TableColumn, index = 0) => getCellValue(row, field, column, index)

describe('getCellValue', () => {
    it('无 prop 时返回空串', () => {
        expect(cell({ a: 1 }, {})).toBe('')
    })

    it('支持点号嵌套 prop，中间层缺失按无值处理', () => {
        expect(cell({ user: { nickname: 'Tom' } }, { prop: 'user.nickname' })).toBe('Tom')
        expect(cell({}, { prop: 'user.nickname' })).toBe('')
    })

    it('无值时回退 default，仅 undefined/null/空串算无值', () => {
        expect(cell({}, { prop: 'x', default: '未知' })).toBe('未知')
        expect(cell({ x: null }, { prop: 'x', default: '未知' })).toBe('未知')
        expect(cell({ x: 0 }, { prop: 'x', default: '未知' })).toBe(0)
    })

    it('formatter 以 el-table 签名（row, column, cellValue, index）参与处理', () => {
        const field: TableColumn = { prop: 'money', formatter: (row, _column, value, index) => `${row.id}-${value}-${index}` }
        expect(cell({ id: 7, money: 12 }, field, 3)).toBe('7-12-3')
    })

    it('renderFormatter 生效并提示废弃', () => {
        const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
        const field: TableColumn = { prop: 'name', renderFormatter: (_row, _field, value) => String(value).toUpperCase() }

        expect(cell({ name: 'abc' }, field)).toBe('ABC')
        expect(warn).toHaveBeenCalledTimes(1)
        expect(String(warn.mock.calls[0][0])).toContain('renderFormatter')
    })
})

describe('invokeTableContextDataFun', () => {
    it('undefined 返回默认值', () => {
        const fallback = { a: 1 }
        expect(invokeTableContextDataFun(undefined, {})).toEqual({})
        expect(invokeTableContextDataFun(undefined, {}, fallback)).toBe(fallback)
    })

    it('函数入参用上下文求值', () => {
        const context: TableContextData = { cellValue: 2, index: 1 }
        const fun = (ctx: TableContextData) => ({ text: `${ctx.cellValue}@${ctx.index}` })
        expect(invokeTableContextDataFun(fun, context)).toEqual({ text: '2@1' })
    })

    it('对象入参原样返回', () => {
        const props = { type: 'success' }
        expect(invokeTableContextDataFun(props, {})).toBe(props)
    })
})

describe('findIndexRow', () => {
    const data: TableRow[] = [{ id: 1 }, { id: 2, children: [{ id: 3 }, { id: 4 }] }, { id: 5 }]

    // findIdx 是降维后的行序号：0={id:1} 1={id:2} 2={id:3} 3={id:4} 4={id:5}
    it('命中带 children 的深层行时返回该行对象', () => {
        expect(findIndexRow(data, 2)).toEqual({ id: 3 })
        expect(findIndexRow(data, 4)).toEqual({ id: 5 })
    })

    it('未命中时返回游标数字而非行对象', () => {
        expect(findIndexRow(data, 99)).toBe(4)
    })
})

describe('fieldRender 运行时渲染', () => {
    it('color.vue 用 getCellValue 渲染背景色', () => {
        const wrapper = mount(ColorRender, {
            props: {
                row: { color: '#ff0000' },
                field: { prop: 'color', render: 'color' },
                column,
                index: 0
            }
        })

        const style = wrapper.find('.ba-table-render-color').attributes('style') ?? ''
        // happy-dom 对 style 绑定保留原始值，不同环境可能规范化为 rgb()
        expect(style).toMatch(/#ff0000|rgb\(255,\s*0,\s*0\)/)
    })
})
