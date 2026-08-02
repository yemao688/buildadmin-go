// Package router 提供 admin 渠道模块注册器的能力声明辅助：AtomicRoute
// 能力（add/edit/del）与路由注册器聚合。能力注册表在
// internal/middleware/security.go，admin 与 api 渠道共用。
package router

import (
	"net/http"
	"strings"

	"buildadmin-go/internal/middleware"
)

const (
	actionAdd  = "add"
	actionEdit = "edit"
	actionDel  = "del"
)

// CRUDCapabilities 声明标准 CRUD 路由（index/add/edit/del/sortable）的
// 原子写能力：add/edit 为 POST，del 为 DELETE。路由注册器在
// Capabilities() 中返回它，由 AdminRouter 在挂载路由前统一登记。
func CRUDCapabilities(name string) []middleware.AtomicRoute {
	route := capabilityRoute(name)
	return []middleware.AtomicRoute{
		{Route: route, Action: actionAdd, Method: http.MethodPost},
		{Route: route, Action: actionEdit, Method: http.MethodPost},
		{Route: route, Action: actionDel, Method: http.MethodDelete},
	}
}

func capabilityRoute(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, ".", "/"))
}
