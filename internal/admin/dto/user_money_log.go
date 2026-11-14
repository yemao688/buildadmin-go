package dto

import (
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"

	"buildadmin-go/internal/pkg/validator"
)

// UserMoneyLogParam 是会员余额变动（表 user_money_log）的新增请求 DTO。
// 金额在 DTO 层解析与校验，业务编排（变动类型收敛、余额事务链）由 service
// 负责，handler 只绑定参数。
type UserMoneyLogParam struct {
	UserID validator.FlexInt32 `json:"user_id" binding:"required"` // 会员ID
	Money  json.RawMessage     `json:"money" binding:"required"`   // 变动金额（元，最多两位小数）
	Type   string              `json:"type"`                       // 类型:system=系统,recharge=充值,withdraw=提现,extend=拓展（缺省 system；仅超管可指定）
	Memo   string              `json:"memo"`                       // 备注（可选）
}

func (v UserMoneyLogParam) GetMessages() validator.ValidatorMessages {
	return validator.ValidatorMessages{
		"user_id.required": "user_id required",
		"money.required":   "money required",
	}
}

// Amount 解析并校验 money 字段：接受 JSON 数字与数字字符串，拒绝指数记法
// （e/E/+）、超过两位小数与非有限值。错误文案与历史接口保持一致。
func (v UserMoneyLogParam) Amount() (float64, error) {
	s := trimMoneyAmountQuotes(strings.TrimSpace(string(v.Money)))
	if s == "" || strings.ContainsAny(s, "eE+") {
		return 0, errors.New("invalid money amount")
	}

	negative := false
	if s[0] == '-' {
		negative = true
		s = s[1:]
	}
	if s == "" {
		return 0, errors.New("invalid money amount")
	}
	if err := validateMoneyDecimals(s); err != nil {
		return 0, err
	}

	amount, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return 0, errors.New("invalid money amount")
	}
	if negative {
		amount = -amount
	}
	return amount, nil
}

func trimMoneyAmountQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// validateMoneyDecimals 校验十进制金额形状：最多两位小数且只含数字。
func validateMoneyDecimals(s string) error {
	parts := strings.Split(s, ".")
	if len(parts) > 2 || parts[0] == "" || (len(parts) == 2 && len(parts[1]) > 2) {
		return errors.New("money must have at most two decimals")
	}
	for _, part := range parts {
		for _, char := range part {
			if char < '0' || char > '9' {
				return errors.New("invalid money amount")
			}
		}
	}
	return nil
}
