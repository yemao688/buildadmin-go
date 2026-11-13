package handler

import "testing"

// 三个设计器共用同一转换，差异只在排除清单：回收不含 config/admin_log，
// 敏感数据含；条目按去前缀后的表名比较。
func TestAdminDataTableListExclusionLists(t *testing.T) {
	tables := map[string]string{
		"ba_area":           "地区",
		"ba_config":         "配置",
		"ba_admin_log":      "日志",
		"ba_user_money_log": "余额日志",
		"ba_order_recharge": "充值订单",
	}

	recycle := adminDataTableList(tables, "ba_", []string{"area", "token", "captcha", "admin_group_access", "user_money_log"})
	if _, ok := recycle["config"]; !ok {
		t.Fatal("回收清单不应排除 config")
	}
	if _, ok := recycle["admin_log"]; !ok {
		t.Fatal("回收清单不应排除 admin_log")
	}
	if _, ok := recycle["area"]; ok {
		t.Fatal("回收清单必须排除 area")
	}
	if recycle["order_recharge"] != "充值订单" {
		t.Fatalf("注释未透传: %#v", recycle)
	}

	sensitive := adminDataTableList(tables, "ba_", []string{"area", "token", "captcha", "admin_group_access", "config", "admin_log", "user_money_log"})
	for _, name := range []string{"area", "config", "admin_log", "user_money_log"} {
		if _, ok := sensitive[name]; ok {
			t.Fatalf("敏感数据清单必须排除 %q", name)
		}
	}
	if sensitive["order_recharge"] != "充值订单" {
		t.Fatalf("敏感数据清单丢失未排除的表: %#v", sensitive)
	}
}
