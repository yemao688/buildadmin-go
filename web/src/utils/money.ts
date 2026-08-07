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
}

export function formatMoney(amount: number | string, opts: MoneyFormatOptions = {}): string {
    const config = useConfig()
    const code = opts.code ?? config.portalCurrency.defaultCurrency
    const currency = config.portalCurrency.currencyArray.find((item) => item.code === code)
    const rate = opts.rate ?? config.portalCurrency.currencyRates[code] ?? 1
    const symbol = opts.symbol ?? currency?.symbol ?? ''
    // toFixed 小数位上限 100，钳制避免调用方传入非法值抛错
    const decimals = Math.min(100, Math.max(0, opts.decimals ?? 2))
    const position = opts.position ?? 'prefix'

    const value = (Number(amount) || 0) * rate
    const formatted = value.toFixed(decimals)

    return position === 'suffix' ? `${formatted}${symbol}` : `${symbol}${formatted}`
}
