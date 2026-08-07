/**
 * 金额换算/格式化工具（前台门户货币基座，与 stores/config 的 portalCurrency 域配套）。
 *
 * 换算语义：后端 country_currency.rate 是相对基准货币的乘数，展示金额 = 数值 × rate；
 * 符号默认前缀（如 ¥100.00），未知/未配置货币兜底 rate=1、symbol=''（原样展示数值）。
 * 需在 pinia 安装后调用（组件 setup 或事件回调中）。
 */
import { useConfig } from '/@/stores/config'

export interface MoneyFormatOptions {
    // 货币 code；缺省取 store 当前默认货币（portalCurrency.defaultCurrency）
    code?: string
    // 货币符号覆盖；缺省按 code 从 store 的 currencyArray 查找，未知货币为空字符串
    symbol?: string
    // 换算率覆盖；缺省按 code 从 store 的 currencyRates 查找，未知货币为 1
    rate?: number
    // 符号位置：prefix=前置（¥100.00）、suffix=后置（100.00¥），缺省 prefix
    position?: 'prefix' | 'suffix'
    // 小数位，缺省 2
    decimals?: number
    // 千分位：仅整数部分三位分组（1,234.56），缺省 true（业务默认千分位展示）
    thousandSeparator?: boolean
    // 显式正负号：signed 时正数前缀 +、负数前缀 -（负数用 ASCII '-'，不用 U+2212），
    // 且正负号在货币符号之前（如 +$100.00 / -$100.00，suffix 位置 +100.00$ / -100.00$）；
    // 缺省 false 时正数不带符号、负数保持现状（符号在货币符号后，如 $-123.45）
    signed?: boolean
}

// 千分位分组：对数值串的整数部分按三位分组。正则 \B(?=(\d{3})+(?!\d)) 只匹配
// "其后紧跟三位的整数倍、且不是最后三位"的位置：1234.56 → 1,234.56、
// 1234567.89 → 1,234,567.89、123 → 123；负号前无数字故不被分组
// （-1234.56 → -1,234.56）。先按 '.' 拆分、只对整数部分应用该正则，避免
// 小数部分被误分组（纯正则会把 123.4567 的 4 位小数分成 123.4,567）。
function addThousandSeparator(formatted: string): string {
    const [integer, fraction] = formatted.split('.')
    const grouped = integer.replace(/\B(?=(\d{3})+(?!\d))/g, ',')
    return fraction === undefined ? grouped : `${grouped}.${fraction}`
}

export function formatMoney(amount: number | string, opts: MoneyFormatOptions = {}): string {
    // 空态不显示金额：null / undefined / 空串直接返回空串（业务语义对齐）；
    // 0 / '0' / '0.00' 等仍正常格式化，NaN 走 Number(NaN)||0 → 0.00
    if (amount === null || amount === undefined || amount === '') {
        return ''
    }
    const config = useConfig()
    const code = opts.code ?? config.portalCurrency.defaultCurrency
    const currency = config.portalCurrency.currencyArray.find((item) => item.code === code)
    const rate = opts.rate ?? config.portalCurrency.currencyRates[code] ?? 1
    const symbol = opts.symbol ?? currency?.symbol ?? ''
    // toFixed 小数位上限 100，钳制避免调用方传入非法值抛错
    const decimals = Math.min(100, Math.max(0, opts.decimals ?? 2))
    const position = opts.position ?? 'prefix'
    const signed = opts.signed ?? false
    const thousandSeparator = opts.thousandSeparator ?? true

    const value = (Number(amount) || 0) * rate
    // 用绝对值 toFixed，避免负数自带 '-' 与货币符号拼出双负号（-$-123.45）
    const abs = Math.abs(value)
    const body = addThousandSeparator(abs.toFixed(decimals))
    // signed 时正负号显式前缀且位于货币符号之前；非 signed 时负号由 toFixed
    // 的 '-' 承载、保持现状（$-123.45：符号 prefix + 负号在数值上）
    const display = signed ? `${value < 0 ? '-' : value > 0 ? '+' : ''}${body}` : `${value < 0 ? '-' : ''}${body}`

    return position === 'suffix' ? `${display}${symbol}` : `${symbol}${display}`
}
