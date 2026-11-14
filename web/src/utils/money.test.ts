import { describe, expect, it, vi } from 'vitest'
import type { CurrencyOption, PortalCurrency } from '/@/stores/interface'
import { formatMoney } from '/@/utils/money'

// useConfig 依赖已安装的 pinia；这里只注入 formatMoney 读取的 portalCurrency 域
const { portalCurrency } = vi.hoisted(() => {
    const currencyArray: CurrencyOption[] = [
        { code: 'CNY', name: '人民币', symbol: '¥' },
        { code: 'USD', name: 'US Dollar', symbol: '$' }
    ]
    const state: PortalCurrency = {
        defaultCurrency: 'CNY',
        currencySet: false,
        currencyArray,
        currencyRates: { CNY: 1, USD: 7.2 }
    }
    return { portalCurrency: state }
})

vi.mock('/@/stores/config', () => ({
    useConfig: () => ({ portalCurrency })
}))

describe('formatMoney', () => {
    it('空值返回空串，0 与 NaN 正常展示为 0.00', () => {
        expect(formatMoney('')).toBe('')
        expect(formatMoney(null as unknown as number)).toBe('')
        expect(formatMoney(undefined as unknown as number)).toBe('')
        expect(formatMoney(0)).toBe('¥0.00')
        expect(formatMoney(NaN)).toBe('¥0.00')
    })

    it('缺省使用 store 默认货币并按 rate 换算', () => {
        expect(formatMoney(1234.56)).toBe('¥1,234.56')
        expect(formatMoney(100, { code: 'USD' })).toBe('$720.00')
        expect(formatMoney('1234.5')).toBe('¥1,234.50')
    })

    it('未知货币兜底 rate=1、symbol 为空', () => {
        expect(formatMoney(100.5, { code: 'JPY' })).toBe('100.50')
    })

    it('千分位只作用于整数部分', () => {
        expect(formatMoney(1234567.89)).toBe('¥1,234,567.89')
        expect(formatMoney(-1234.56)).toBe('¥-1,234.56')
        // 注：thousandSeparator:false 目前是空操作（实现未消费该选项），故不在此断言
    })

    it('符号位置 prefix/suffix', () => {
        expect(formatMoney(1234.56, { position: 'suffix' })).toBe('1,234.56¥')
        expect(formatMoney(10, { symbol: '€' })).toBe('€10.00')
    })

    it('小数位钳制在 0..100', () => {
        expect(formatMoney(1.6, { decimals: 0 })).toBe('¥2')
        expect(formatMoney(1.4, { decimals: -1 })).toBe('¥1')
        // toFixed 超过 100 会抛 RangeError，钳制后按 100 位输出
        expect(formatMoney(1, { decimals: 200 })).toBe('¥' + (1).toFixed(100))
    })

    it('signed 为正负数显式加 ASCII 符号，0 不带符号', () => {
        // 现状：prefix 时货币符号仍在前（¥+100.00），与接口注释“符号位于货币符号之前”不一致
        expect(formatMoney(100, { signed: true })).toBe('¥+100.00')
        expect(formatMoney(-100, { signed: true })).toBe('¥-100.00')
        expect(formatMoney(-100, { signed: true, position: 'suffix' })).toBe('-100.00¥')
        expect(formatMoney(0, { signed: true })).toBe('¥0.00')
    })

    it('显式 rate 覆盖 store 换算率', () => {
        expect(formatMoney(10, { rate: 2 })).toBe('¥20.00')
    })
})
